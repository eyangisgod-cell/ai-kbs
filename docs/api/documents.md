# Documents

## Create

```http
POST /api/ai-kbs/documents
Content-Type: application/json
Authorization: Bearer <token>

{
  "space": "dev",
  "title": "Onboarding",
  "path": "/handbook/onboarding.md",
  "content": "...",
  "format": "markdown",
  "base_version": 0
}
```

Returns the new `docCode` and the assigned `version` (1 on first create).

## Update (CAS)

```http
PUT /api/ai-kbs/documents/aB3xY9zQ
Content-Type: application/json
Authorization: Bearer <token>

{
  "content": "...",
  "base_version": 7
}
```

If the server's current version differs from `base_version`, the response is `409 Conflict` with `error.code = "doc.version_conflict"` and `next_action.strategies` populated.

## Read

```http
GET /api/ai-kbs/documents/aB3xY9zQ
```

Returns the latest version, with metadata:

```json
{
  "docCode": "aB3xY9zQ",
  "title": "Onboarding",
  "path": "/handbook/onboarding.md",
  "content": "...",
  "version": 7,
  "size": 1234,
  "updated_at": "2026-10-09T10:00:00Z",
  "updated_by": "u_...",
  "tags": ["handbook"]
}
```

For large documents, append `?lines=200&offset=0` to paginate.

## Versions

```http
GET /api/ai-kbs/documents/aB3xY9zQ/versions
```

```json
{
  "versions": [
    {"version": 7, "created_at": "...", "created_by": "u_...", "size": 1234, "message": "fix typo"},
    {"version": 6, ...},
    ...
  ]
}
```

## Diff

```http
GET /api/ai-kbs/documents/aB3xY9zQ/diff?from=5&to=7
```

Returns unified-diff text (or JSON with `format: "json"`).

## Rollback

```http
POST /api/ai-kbs/documents/aB3xY9zQ/rollback
Content-Type: application/json

{
  "to_version": 5,
  "base_version": 7
}
```

The new current version becomes `8` (containing the content of version 5). Version 7 is preserved.

## Move / Rename

`docCode` is stable, so rename / move is just a metadata update. References in other documents are unaffected.

```http
PATCH /api/ai-kbs/documents/aB3xY9zQ
Content-Type: application/json

{
  "path": "/handbook/v2/onboarding.md",
  "base_version": 7
}
```

## Delete

Soft-delete by default. The document is hidden from listings but its versions are preserved. Use `?hard=true` to permanently remove (Owner scope only).
