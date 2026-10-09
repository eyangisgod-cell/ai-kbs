package doc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/eyangisgod-cell/ai-kbs/internal/repository"
	"github.com/eyangisgod-cell/ai-kbs/internal/repository/domain"
)

// ---------------------------------------------------------------------------
// Hand-rolled fakes for DocumentRepository + DocumentVersionRepository.
//
// We deliberately do not use mockgen here. The repository seam exposes
// only the methods CAS touches, and a 30-line fake is easier to audit in
// code review than a 200-line generated file. Each fake tracks call
// counts in atomic counters so the 100-goroutine concurrency test can
// verify expectations safely under -race.
// ---------------------------------------------------------------------------

// fakeDocRepo is a thread-safe fake of repository.DocumentRepository
// that supports only the operations CAS exercises: GetByCode and Update.
// The fields are exported as plain values + setErr hooks so individual
// tests can describe their scenario tersely.
type fakeDocRepo struct {
	mu sync.Mutex

	// current is returned by GetByCode. We dereference under mu
	// because the concurrent test mutates it.
	current *domain.Document

	// getErr is the error returned by GetByCode when set.
	getErr error

	// updateErr is the error returned by Update. If non-nil, the
	// version row should NOT have been bumped (the call fails).
	updateErr error

	// updateNewVersion overrides the newVersion returned by Update
	// in the happy path. Defaults to current.ActiveVersion+1.
	updateNewVersion int

	// counters.
	getCount    atomic.Int64
	updateCount atomic.Int64

	// lastBaseVersion is the baseVersion received by the most
	// recent Update call (testing-only assertion aid).
	lastBaseVersion atomic.Int64
	// lastUpdateID is the document ID received by the most recent
	// Update call.
	lastUpdateID atomic.Value // string
}

func newDocRepo(current *domain.Document) *fakeDocRepo {
	return &fakeDocRepo{current: current}
}

func (f *fakeDocRepo) GetByCode(ctx context.Context, spaceID, docCode string) (*domain.Document, error) {
	f.getCount.Add(1)
	if f.getErr != nil {
		return nil, f.getErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	// Snapshot so concurrent mutation under -race stays clean.
	c := *f.current
	return &c, nil
}

func (f *fakeDocRepo) Update(ctx context.Context, d *domain.Document, baseVersion int) (int, error) {
	f.updateCount.Add(1)
	f.lastBaseVersion.Store(int64(baseVersion))
	f.lastUpdateID.Store(d.ID)
	if f.updateErr != nil {
		return 0, f.updateErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if baseVersion != f.current.ActiveVersion {
		return 0, &repository.VersionConflictError{
			DocCode:        f.current.DocCode,
			BaseVersion:    baseVersion,
			CurrentVersion: f.current.ActiveVersion,
		}
	}
	expected := baseVersion + 1
	if f.updateNewVersion != 0 {
		expected = f.updateNewVersion
	}
	f.current.ActiveVersion = expected
	return expected, nil
}

// Stubs for the rest of the DocumentRepository interface. The CAS
// service only calls GetByCode + Update, but the fake must satisfy the
// interface to be assigned.
func (f *fakeDocRepo) Create(ctx context.Context, d *domain.Document) error {
	return errors.New("fakeDocRepo.Create not implemented")
}
func (f *fakeDocRepo) Get(ctx context.Context, id string) (*domain.Document, error) {
	return nil, errors.New("fakeDocRepo.Get not implemented")
}
func (f *fakeDocRepo) ListBySpace(ctx context.Context, spaceID string, limit, offset int) ([]*domain.Document, error) {
	return nil, errors.New("fakeDocRepo.ListBySpace not implemented")
}
func (f *fakeDocRepo) SoftDelete(ctx context.Context, id string) error {
	return errors.New("fakeDocRepo.SoftDelete not implemented")
}
func (f *fakeDocRepo) GetCurrentVersion(ctx context.Context, documentID string) (*domain.DocumentVersion, error) {
	return nil, errors.New("fakeDocRepo.GetCurrentVersion not implemented")
}

// fakeVersionRepo is a thread-safe fake of
// repository.DocumentVersionRepository. The DeleteByDocumentVersion
// rollback path is exercised by several tests; the others are stubs.
type fakeVersionRepo struct {
	mu sync.Mutex

	// Stored versions by documentID + versionNum. The CAS service
	// creates the row, then may be asked to roll it back. We track
	// both so tests can assert "the rollback ran".
	rows map[string]*domain.DocumentVersion

	// createErr is returned by Create when set.
	createErr error

	// deleteErr is returned by DeleteByDocumentVersion when set.
	deleteErr error

	// counters.
	createCount atomic.Int64
	deleteCount atomic.Int64

	// lastDeletedDocument + lastDeletedVersion track the most-recent
	// rollback call for assertion.
	lastDeletedDocument atomic.Value // string
	lastDeletedVersion  atomic.Int64
}

func newVersionRepo() *fakeVersionRepo {
	return &fakeVersionRepo{rows: make(map[string]*domain.DocumentVersion)}
}

func (f *fakeVersionRepo) Create(ctx context.Context, v *domain.DocumentVersion) error {
	f.createCount.Add(1)
	if f.createErr != nil {
		return f.createErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	key := versionKey(v.DocumentID, v.VersionNum)
	if _, exists := f.rows[key]; exists {
		// Mirror the production UNIQUE(document_id, version_num)
		// constraint: a duplicate insert fails with the same
		// sentinel the real repo's mapPgError would emit. The CAS
		// service then translates that into VersionConflictError,
		// which the concurrent test counts as a conflict.
		return repository.ErrAlreadyExists
	}
	// Copy so callers can mutate without us racing.
	cp := *v
	f.rows[key] = &cp
	return nil
}

func (f *fakeVersionRepo) DeleteByDocumentVersion(ctx context.Context, documentID string, version int) error {
	f.deleteCount.Add(1)
	f.lastDeletedDocument.Store(documentID)
	f.lastDeletedVersion.Store(int64(version))
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	key := versionKey(documentID, version)
	if _, ok := f.rows[key]; !ok {
		return repository.ErrNotFound
	}
	delete(f.rows, key)
	return nil
}

func versionKey(documentID string, version int) string {
	return fmt.Sprintf("%s/%d", documentID, version)
}

// countStoredVersions is a small helper for assertions in tests.
func (f *fakeVersionRepo) countStoredVersions(documentID string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for k := range f.rows {
		// Cheap prefix match without building a slice.
		if len(k) >= len(documentID) && k[:len(documentID)] == documentID {
			n++
		}
	}
	return n
}

func (f *fakeVersionRepo) Get(ctx context.Context, id string) (*domain.DocumentVersion, error) {
	return nil, errors.New("fakeVersionRepo.Get not implemented")
}
func (f *fakeVersionRepo) GetByVersion(ctx context.Context, documentID string, version int) (*domain.DocumentVersion, error) {
	return nil, repository.ErrNotFound
}
func (f *fakeVersionRepo) ListByDocument(ctx context.Context, documentID string) ([]*domain.DocumentVersion, error) {
	return nil, errors.New("fakeVersionRepo.ListByDocument not implemented")
}

// ---------------------------------------------------------------------------
// Tests.
// ---------------------------------------------------------------------------

// newTestDoc returns a Document at ActiveVersion=5 with sane defaults.
func newTestDoc() *domain.Document {
	return &domain.Document{
		ID:            "doc-id-1",
		SpaceID:       "space-1",
		DocCode:       "abc1234",
		Title:         "Hello",
		DocType:       "markdown",
		MimeType:      "text/markdown",
		ActiveVersion: 5,
		ActiveMD5:     "oldmd5",
	}
}

func TestUpdateWithCAS_HappyPath(t *testing.T) {
	docs := newDocRepo(newTestDoc())
	versions := newVersionRepo()
	svc := NewCASService(docs, versions)

	got, err := svc.UpdateWithCAS(context.Background(), "space-1", "abc1234", 5, "new content", "edit msg", "u-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil DocumentVersion")
	}
	if got.VersionNum != 6 {
		t.Fatalf("VersionNum=%d want 6", got.VersionNum)
	}
	if got.ParentVersion == nil || *got.ParentVersion != 5 {
		t.Fatalf("ParentVersion=%v want 5", got.ParentVersion)
	}
	if got.DocumentID != "doc-id-1" {
		t.Fatalf("DocumentID=%q want %q", got.DocumentID, "doc-id-1")
	}
	if got.MD5 == "" || got.MD5 == "oldmd5" {
		t.Fatalf("MD5 should be a fresh hex hash, got %q", got.MD5)
	}
	if got.AuthorID != "u-1" {
		t.Fatalf("AuthorID=%q want %q", got.AuthorID, "u-1")
	}
	if got.Message != "edit msg" {
		t.Fatalf("Message=%q want %q", got.Message, "edit msg")
	}
	if got.ID == "" {
		t.Fatal("ID should be populated (UUID)")
	}
	// Header should have been bumped.
	if docs.updateCount.Load() != 1 {
		t.Fatalf("docs.Update calls=%d want 1", docs.updateCount.Load())
	}
	if docs.lastBaseVersion.Load() != 5 {
		t.Fatalf("lastBaseVersion=%d want 5", docs.lastBaseVersion.Load())
	}
	// Version row should have been inserted.
	if got := versions.countStoredVersions("doc-id-1"); got != 1 {
		t.Fatalf("stored versions=%d want 1", got)
	}
}

func TestUpdateWithCAS_StaleBase(t *testing.T) {
	// Server's view is at v7; client thinks it's at v5. Fast path:
	// we return VersionConflictError BEFORE touching versions.
	doc := newTestDoc()
	doc.ActiveVersion = 7
	docs := newDocRepo(doc)
	versions := newVersionRepo()
	svc := NewCASService(docs, versions)

	_, err := svc.UpdateWithCAS(context.Background(), "space-1", "abc1234", 5, "new", "", "u-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, repository.ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict, got %v", err)
	}
	var vce *repository.VersionConflictError
	if !errors.As(err, &vce) {
		t.Fatalf("expected *VersionConflictError, got %T", err)
	}
	if vce.BaseVersion != 5 || vce.CurrentVersion != 7 {
		t.Fatalf("vce=%+v want base=5 current=7", vce)
	}
	// Critically: NO version row should have been inserted, and
	// NO Update call should have happened.
	if versions.createCount.Load() != 0 {
		t.Fatalf("expected no versions.Create calls, got %d", versions.createCount.Load())
	}
	if docs.updateCount.Load() != 0 {
		t.Fatalf("expected no docs.Update calls, got %d", docs.updateCount.Load())
	}
}

func TestUpdateWithCAS_DocNotFound(t *testing.T) {
	docs := newDocRepo(newTestDoc())
	docs.getErr = repository.ErrNotFound
	versions := newVersionRepo()
	svc := NewCASService(docs, versions)

	_, err := svc.UpdateWithCAS(context.Background(), "space-1", "abc1234", 5, "new", "", "u-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound (wrapped), got %v", err)
	}
	if versions.createCount.Load() != 0 {
		t.Fatalf("expected no Create, got %d", versions.createCount.Load())
	}
}

func TestUpdateWithCAS_RepoUpdateConflict_RollsBackVersion(t *testing.T) {
	// Slow path: GetByCode at v=5, version row created, then
	// docs.Update returns VersionConflictError because another writer
	// bumped it in between. The service must roll back the version
	// row and surface the conflict to the caller.
	docs := newDocRepo(newTestDoc())
	// Inject a conflict on Update. fakeDocRepo.Update returns
	// updateErr verbatim if set.
	docs.updateErr = &repository.VersionConflictError{
		DocCode:        "abc1234",
		BaseVersion:    5,
		CurrentVersion: 9,
	}
	versions := newVersionRepo()
	svc := NewCASService(docs, versions)

	_, err := svc.UpdateWithCAS(context.Background(), "space-1", "abc1234", 5, "new", "", "u-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, repository.ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict, got %v", err)
	}
	var vce *repository.VersionConflictError
	if !errors.As(err, &vce) {
		t.Fatalf("expected *VersionConflictError, got %T", err)
	}
	if vce.CurrentVersion != 9 {
		t.Fatalf("vce.CurrentVersion=%d want 9", vce.CurrentVersion)
	}
	if versions.createCount.Load() != 1 {
		t.Fatalf("expected 1 Create, got %d", versions.createCount.Load())
	}
	if versions.deleteCount.Load() != 1 {
		t.Fatalf("expected 1 rollback Delete, got %d", versions.deleteCount.Load())
	}
	if got := versions.lastDeletedVersion.Load(); got != 6 {
		t.Fatalf("rollback version=%d want 6 (baseVersion+1)", got)
	}
	if n := versions.countStoredVersions("doc-id-1"); n != 0 {
		t.Fatalf("stored versions after rollback=%d want 0", n)
	}
}

func TestUpdateWithCAS_RepoUpdateOtherError_RollsBackVersion(t *testing.T) {
	// Same shape as the conflict test, but Update returns a generic
	// error. The service must still roll back the version row and
	// surface the error to the caller.
	docs := newDocRepo(newTestDoc())
	docs.updateErr = errors.New("postgres: connection reset")
	versions := newVersionRepo()
	svc := NewCASService(docs, versions)

	_, err := svc.UpdateWithCAS(context.Background(), "space-1", "abc1234", 5, "new", "", "u-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, repository.ErrVersionConflict) {
		t.Fatalf("must NOT be ErrVersionConflict; got %v", err)
	}
	if versions.deleteCount.Load() != 1 {
		t.Fatalf("expected 1 rollback Delete, got %d", versions.deleteCount.Load())
	}
	if n := versions.countStoredVersions("doc-id-1"); n != 0 {
		t.Fatalf("stored versions after rollback=%d want 0", n)
	}
}

func TestUpdateWithCAS_InvalidBaseVersion(t *testing.T) {
	docs := newDocRepo(newTestDoc())
	versions := newVersionRepo()
	svc := NewCASService(docs, versions)

	_, err := svc.UpdateWithCAS(context.Background(), "space-1", "abc1234", -1, "new", "", "u-1")
	if err == nil {
		t.Fatal("expected error for negative baseVersion, got nil")
	}
	// Negative base is a precondition violation: NO docs or
	// versions calls should run.
	if docs.getCount.Load() != 0 {
		t.Fatalf("docs.GetByCode should not run, got %d", docs.getCount.Load())
	}
	if versions.createCount.Load() != 0 {
		t.Fatalf("versions.Create should not run, got %d", versions.createCount.Load())
	}
}

func TestUpdateWithCAS_EmptyDocCode(t *testing.T) {
	docs := newDocRepo(newTestDoc())
	versions := newVersionRepo()
	svc := NewCASService(docs, versions)

	_, err := svc.UpdateWithCAS(context.Background(), "space-1", "", 5, "new", "", "u-1")
	if err == nil {
		t.Fatal("expected error for empty docCode, got nil")
	}
	if docs.getCount.Load() != 0 {
		t.Fatalf("docs.GetByCode should not run, got %d", docs.getCount.Load())
	}
}

func TestUpdateWithCAS_NewCASService_NilArgs(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic when docs is nil")
		}
	}()
	_ = NewCASService(nil, newVersionRepo())
}

func TestUpdateWithCAS_Concurrent_100_OneWins(t *testing.T) {
	// AC-B4: 100 goroutines concurrently call UpdateWithCAS at the
	// same baseVersion=5; exactly 1 should succeed; the rest must
	// lose in some way (stale-check OR CAS conflict).
	//
	// Under the fake's locking, the cheap stale-check (step 2 in
	// cas.go) typically catches the 99 losers as soon as the winner
	// bumps current.ActiveVersion from 5 to 6 inside Update. A
	// small tail of goroutines that slipped past GetByCode during
	// the winner's window reach Update with a stale baseVersion
	// and lose at the repo's CAS check — they still see
	// ErrVersionConflict. We assert:
	//   - exactly 1 success,
	//   - the rest returned a VersionConflictError,
	//   - the winner's row is the only stored version row.
	doc := newTestDoc()
	doc.ActiveVersion = 5
	docs := newDocRepo(doc)
	versions := newVersionRepo()
	svc := NewCASService(docs, versions)

	const N = 100
	var wg sync.WaitGroup
	wg.Add(N)
	var success, conflict atomic.Int64
	for i := 0; i < N; i++ {
		go func(i int) {
			defer wg.Done()
			_, err := svc.UpdateWithCAS(context.Background(), "space-1", "abc1234", 5,
				fmt.Sprintf("content from goroutine %d", i), "msg", "u-1")
			if err == nil {
				success.Add(1)
				return
			}
			if errors.Is(err, repository.ErrVersionConflict) {
				conflict.Add(1)
				return
			}
			t.Errorf("goroutine %d: unexpected error %v", i, err)
		}(i)
	}
	wg.Wait()

	if s := success.Load(); s != 1 {
		t.Fatalf("success=%d want 1", s)
	}
	// All losers must be ErrVersionConflict — AC-B4 wording is "99
	// receive ErrVersionConflict", and we get there deterministically
	// because every loser either exits at the cheap stale-check or
	// at the repo's CAS conflict; both go through VersionConflictError.
	// There is one known edge case: if a loser slipped past
	// GetByCode at v=5 AND the previous winner's bump rolled the
	// shared current back through a different path, the loser
	// could theoretically see a different wrapper. We tolerate a
	// 1-row slack to keep -race robust.
	if c := conflict.Load(); c < int64(N-1) {
		t.Fatalf("conflict=%d want >=%d (all losers must be ErrVersionConflict)", c, N-1)
	}
	if g := docs.getCount.Load(); g != int64(N) {
		t.Fatalf("docs.GetByCode call count=%d want %d", g, N)
	}
	// Update is reached only by goroutines whose GetByCode saw v=5
	// AND whose Update call landed after a prior Update's bump.
	// Under -race this is typically 1-3; bound generously to 50 to
	// keep the test non-flaky on heavily-loaded CI machines.
	if u := docs.updateCount.Load(); u < 1 || u > 50 {
		t.Fatalf("docs.Update calls=%d want in [1, 50]", u)
	}
	// One winner's version row stays; losers' rows were rolled back
	// by the service when their Update returned conflict.
	if n := versions.countStoredVersions("doc-id-1"); n != 1 {
		t.Fatalf("stored versions=%d want 1 (only the winner's row remains)", n)
	}
}

func TestUpdateWithCAS_Retry_ChainSucceeds(t *testing.T) {
	// Call 1: stale view (current=v7), caller passes baseVersion=5.
	// Service returns VersionConflictError{Current=7}.
	// Caller re-reads, sees current=7, calls again with
	// baseVersion=7 — succeeds with v=8.
	doc := newTestDoc()
	doc.ActiveVersion = 7
	docs := newDocRepo(doc)
	versions := newVersionRepo()
	svc := NewCASService(docs, versions)

	// First attempt: stale.
	_, err := svc.UpdateWithCAS(context.Background(), "space-1", "abc1234", 5, "v1", "msg", "u-1")
	if !errors.Is(err, repository.ErrVersionConflict) {
		t.Fatalf("first call: expected ErrVersionConflict, got %v", err)
	}
	var vce *repository.VersionConflictError
	if !errors.As(err, &vce) {
		t.Fatalf("first call: expected *VersionConflictError, got %T", err)
	}
	if vce.CurrentVersion != 7 {
		t.Fatalf("first call: current=%d want 7", vce.CurrentVersion)
	}
	// Second attempt: re-read, retry with the conflict's Current.
	got, err := svc.UpdateWithCAS(context.Background(), "space-1", "abc1234", vce.CurrentVersion, "v2", "msg", "u-1")
	if err != nil {
		t.Fatalf("second call: unexpected error %v", err)
	}
	if got.VersionNum != vce.CurrentVersion+1 {
		t.Fatalf("second call: VersionNum=%d want %d", got.VersionNum, vce.CurrentVersion+1)
	}
}

func TestUpdateWithCAS_ContentHashStable(t *testing.T) {
	// Two calls with the same newContent must produce the same
	// MD5 in DocumentVersion.MD5. SHA-256 is deterministic; this
	// test guards against accidental upgrades to a non-deterministic
	// hash (timestamped, salted, etc.).
	docs := newDocRepo(newTestDoc())
	versions := newVersionRepo()
	svc := NewCASService(docs, versions)

	const content = "the quick brown fox jumps over the lazy dog"
	got1, err := svc.UpdateWithCAS(context.Background(), "space-1", "abc1234", 5, content, "", "u-1")
	if err != nil {
		t.Fatalf("call 1: %v", err)
	}
	// Reset state for the second call.
	doc := newTestDoc()
	docs2 := newDocRepo(doc)
	versions2 := newVersionRepo()
	svc2 := NewCASService(docs2, versions2)
	got2, err := svc2.UpdateWithCAS(context.Background(), "space-1", "abc1234", 5, content, "", "u-1")
	if err != nil {
		t.Fatalf("call 2: %v", err)
	}
	if got1.MD5 != got2.MD5 {
		t.Fatalf("MD5 unstable: %q vs %q", got1.MD5, got2.MD5)
	}
	// Sanity: the SHA-256 hex is 64 chars.
	if len(got1.MD5) != 64 {
		t.Fatalf("MD5 length=%d want 64 (SHA-256 hex)", len(got1.MD5))
	}
}

func TestUpdateWithCAS_DocumentIDPropagatedIntoVersion(t *testing.T) {
	// Defensive: ensure the new version row carries the parent
	// document's ID, not an empty string. Trivial happy-path
	// coverage but catches the case where someone refactors the
	// DocumentVersion construction and forgets the DocumentID.
	docs := newDocRepo(newTestDoc())
	versions := newVersionRepo()
	svc := NewCASService(docs, versions)

	got, err := svc.UpdateWithCAS(context.Background(), "space-1", "abc1234", 5, "x", "", "u-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.DocumentID != "doc-id-1" {
		t.Fatalf("DocumentID=%q want doc-id-1", got.DocumentID)
	}
}

func TestUpdateWithCAS_CreateUniquenessBecomesVersionConflict(t *testing.T) {
	// When the production DB rejects a duplicate insert with
	// ErrAlreadyExists (UNIQUE(document_id, version_num) violation),
	// the CAS service must translate that to VersionConflictError
	// so the caller's retry logic works regardless of which step in
	// the CAS loop the race resolves at. This test forces the fake
	// to simulate a duplicate (the second Create call rejects) and
	// asserts the service translates it.
	docs := newDocRepo(newTestDoc())
	versions := newVersionRepo()
	// Pre-populate the version row at (doc-id-1, 6) so the next
	// Create collides. This mimics "another writer stamped v=6
	// before us".
	versions.rows[versionKey("doc-id-1", 6)] = &domain.DocumentVersion{
		DocumentID: "doc-id-1",
		VersionNum: 6,
		MD5:        "x",
	}
	svc := NewCASService(docs, versions)

	_, err := svc.UpdateWithCAS(context.Background(), "space-1", "abc1234", 5, "new", "", "u-1")
	if err == nil {
		t.Fatal("expected error from duplicate version insert")
	}
	if !errors.Is(err, repository.ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict (translated from ErrAlreadyExists), got %v", err)
	}
	// Crucially: docs.Update must NOT have been called, because we
	// already failed at step 4 and short-circuited.
	if u := docs.updateCount.Load(); u != 0 {
		t.Fatalf("docs.Update calls=%d want 0 (we short-circuited at step 4)", u)
	}
}
