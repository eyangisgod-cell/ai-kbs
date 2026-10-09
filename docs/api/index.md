# HTTP API

The backend exposes a single canonical API under `/api/ai-kbs/`.

## Conventions

- All requests / responses are JSON
- Error responses follow the [frozen envelope v1.0](#error-envelope)
- All write operations require authentication (HMAC, JWT, or scoped API Key)
- All write operations are audit-logged

## Address resolution

Documents are addressed by one of three URL shapes (mutually exclusive):

| Flag combination | Resolution |
|------------------|------------|
| `--url <self-contained URL>` | Parse the URL, extract space + docCode |
| `--space <code> --doc <docCode>` | Direct docCode lookup |
| `--space <code> --path <path>` | Path-based lookup; resolves to docCode on first access |

## Endpoints (excerpt)

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/api/ai-kbs/auth/login` | Exchange HMAC key for JWT |
| `GET`  | `/api/ai-kbs/spaces` | List visible Spaces for current user |
| `POST` | `/api/ai-kbs/spaces` | Create a new Space (admin scope) |
| `GET`  | `/api/ai-kbs/spaces/{code}/tree` | Tree of all documents in a Space |
| `GET`  | `/api/ai-kbs/documents/{docCode}` | Read a single document (latest version) |
| `GET`  | `/api/ai-kbs/documents/{docCode}/versions` | List versions |
| `GET`  | `/api/ai-kbs/documents/{docCode}/versions/{n}` | Read a specific version |
| `POST` | `/api/ai-kbs/documents` | Create a new document (CAS, optional `base_version`) |
| `PUT`  | `/api/ai-kbs/documents/{docCode}` | Update an existing document (CAS) |
| `POST` | `/api/ai-kbs/documents/{docCode}/rollback` | Rollback (CAS; produces a new version) |
| `POST` | `/api/ai-kbs/search` | Multi-layer search |
| `POST` | `/api/ai-kbs/ask` | NDJSON stream of an Agent answer |

## Error envelope

```json
{
  "success": false,
  "exit_code": 22,
  "error": {
    "code": "doc.version_conflict",
    "message": "Document is at version 7, you submitted 5",
    "current_version": 7
  },
  "data": null,
  "next_action": {
    "strategies": ["keep-mine", "keep-theirs", "both", "auto"]
  },
  "request_id": "req_..."
}
```

| Exit code | Meaning |
|-----------|---------|
| 0 | success |
| 1 | generic failure |
| 2 | misuse (bad input) |
| 10 | auth missing / invalid |
| 11 | permission denied (RBAC) |
| 20 | not found |
| 21 | already exists |
| 22 | version conflict (CAS) |
| 23 | partial sync failure |
| 24 | rate-limited |
| 25 | upstream timeout |
| 30-32 | search / sag / llm errors |
| 40-41 | internal / config errors |

See [Auth](/api/auth), [Documents](/api/documents), [Search](/api/search) for details.
