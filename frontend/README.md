# AI-KBS Frontend

Vue 3 + TypeScript SPA for the AI-KBS (AI Knowledge Base) platform.

This package is the browser-based user interface for browsing knowledge base documents, asking multi-hop questions via the SAG engine, and managing workspace spaces.

## Status

This is an early fork of WeKnora v0.7.1 Vue 3 SPA. The T0.10 step established the package skeleton, base path aliases, dev/proxy configuration, and basic lint/format scaffolding. Full white-labeling (logo, brand color, SSO replacement, internal SDK removal) is scheduled for T1.23.

## Tech Stack

- Vue 3.5 (Composition API + `<script setup>`)
- Vite 7 (build + proxy + chunked vendor output)
- TypeScript 6 (strict)
- Pinia 3 (state management)
- Vue Router 4 (SPA routing)
- Vue I18n 11 (zh-CN / en-US)
- TDesign Vue Next (UI components, inherited from WeKnora fork)
- Axios (HTTP client)
- Marked + DOMPurify (markdown rendering)
- Highlight.js, KaTeX, Mermaid (rich content rendering)

## Quick Start

```bash
# Install dependencies
npm install

# Start dev server (http://localhost:5173)
# Requires the AI-KBS backend running on http://localhost:8080
npm run dev

# Type-check + production build
npm run build-with-types

# Production preview (serves built dist/ on :4173)
npm run build && npm run preview
```

## Configuration

Copy `.env.example` to `.env.local` and adjust values as needed:

| Variable | Purpose |
|----------|---------|
| `VITE_AIKBS_API_BASE` | API base path (default `/api/ai-kbs`) |
| `VITE_AIKBS_BACKEND_URL` | Backend host (dev proxy target) |
| `VITE_AIKBS_MCP_URL` | MCP server host (dev proxy target) |
| `VITE_AIKBS_APP_TITLE` | Browser title |
| `VITE_AIKBS_DEFAULT_LOCALE` | `zh-CN` (default) or `en-US` |

## Backend Dependency

This frontend requires the AI-KBS Go backend running. See `../backend/` for the backend service. The dev server proxies `/api` and `/mcp` requests to the backend (configurable URL).

## Path Aliases

| Alias | Target |
|-------|--------|
| `@/*` | `src/*` |
| `@components/*` | `src/components/*` |
| `@views/*` | `src/views/*` |
| `@stores/*` | `src/stores/*` |
| `@api/*` | `src/api/*` |

## Project Structure

```
frontend/
├── src/
│   ├── api/          # REST + WS clients (baseURL: /api/ai-kbs)
│   ├── components/   # Reusable Vue components
│   ├── views/        # Top-level route views
│   ├── stores/       # Pinia stores (auth, workspace, ask)
│   ├── router/       # Vue Router 4 configuration
│   ├── i18n/         # Locale messages
│   ├── composables/  # Composition utilities
│   └── utils/        # Shared helpers
├── public/           # Static assets (favicon, OG images, etc.)
├── packages/         # Vendored tarballs (xlsx)
├── index.html        # SPA entry
├── embed.html        # Embed iframe entry
└── vite.config.ts    # Build + dev proxy config
```

## License

Apache License 2.0