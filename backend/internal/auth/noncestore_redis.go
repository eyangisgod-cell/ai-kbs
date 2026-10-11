package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisNonceStore is the production NonceStore. It uses Redis SETNX EX
// per design.md §4.4: SETNX nonce:<keyID>:<nonce> 1 EX 600. The TTL is
// configurable so deployments that want longer replay windows can
// raise it; the default matches the spec.
type redisNonceStore struct {
	rdb *redis.Client
	ttl time.Duration
}

// NewRedisNonceStore wraps a redis client. The caller owns the client
// lifecycle; closing the store is the caller's responsibility (we
// don't take ownership of the connection here so it can be shared
// with the rest of the application).
func NewRedisNonceStore(rdb *redis.Client, ttl time.Duration) NonceStore {
	if ttl <= 0 {
		ttl = DefaultNonceTTL
	}
	return &redisNonceStore{rdb: rdb, ttl: ttl}
}

func (r *redisNonceStore) CheckAndSet(keyID, nonce string) error {
	if r.rdb == nil {
		// Defensive fallback so the auth package degrades gracefully in
		// environments where Redis is misconfigured (e.g. single-node
		// dev mode). We deliberately do NOT log here — that would
		// spam every request; instead we surface the failure as a
		// transient backend error.
		return errors.New("auth: redis nonce store not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	key := fmt.Sprintf("nonce:%s:%s", keyID, nonce)
	ok, err := r.rdb.SetNX(ctx, key, 1, r.ttl).Result()
	if err != nil {
		return fmt.Errorf("auth: nonce store: %w", err)
	}
	if !ok {
		return ErrNonceReplay
	}
	return nil
}
