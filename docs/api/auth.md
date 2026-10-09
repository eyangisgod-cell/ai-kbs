# Authentication

AI-KBS supports three authentication methods. Choose based on your client.

## 1. HMAC (CLI / automation)

Sign each request with HMAC-SHA256 over `timestamp + nonce + body`.

```bash
# CLI injects this automatically from env / credentials file
export AIKBS_KEY_ID="ak_xxx"
export AIKBS_KEY_SECRET="sk_xxx"
ai-kbs search "what is X"
```

The CLI sends these headers:

- `X-AIKBS-KeyId`
- `X-AIKBS-Timestamp` (Unix seconds)
- `X-AIKBS-Nonce` (random 16 bytes hex)
- `X-AIKBS-Signature` (hex of HMAC-SHA256 of `timestamp + "\n" + nonce + "\n" + body`)

The backend rejects signatures older than 5 minutes or with a repeated nonce (replay protection).

## 2. JWT (Web UI)

Standard HS256 / RS256 JWT. Issued by `/api/ai-kbs/auth/login` after OAuth2 callback or email+password.

```http
POST /api/ai-kbs/auth/login
Authorization: Bearer <hmac-signed login request>
```

```json
{
  "success": true,
  "data": {
    "access_token": "eyJ...",
    "refresh_token": "eyJ...",
    "expires_in": 3600
  }
}
```

Send `Authorization: Bearer <access_token>` on subsequent requests.

## 3. Scoped API Key (MCP / embedded use)

Long-lived keys with explicit scope and per-Space restrictions.

```bash
ai-kbs mcp install --api-key ak_live_xxx --scopes kbs:read,kbs:write
```

Scopes:

- `kbs:read` — search / read documents / list versions
- `kbs:write` — create / update / move / delete documents
- `kbs:version` — rollback / list / diff versions
- `kbs:admin` — manage Spaces, members, API keys, audit log

Keys can be further restricted to a subset of Spaces:

```bash
ai-kbs keys create --name "agent-x" \
  --scopes kbs:read,kbs:write \
  --spaces dev,staging \
  --expires-in 90d
```

## RBAC

Per (user, space) role:

| Role | Can |
|------|-----|
| Owner | everything, including delete-space and rotate-keys |
| Admin | manage members, manage keys, all write operations |
| Contributor | read, write, version operations |
| Viewer | read only |

Cross-workspace super-admin (`is_super_admin` flag) is reserved for SaaS deployments and is not used in self-hosted.
