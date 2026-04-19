## Why

`add-triage-agent` shipped a single-file React + Babel CDN UI and a four-tool agent started via `go run .`. Three things moved after: (1) the frontend was rebuilt on Vite + TypeScript + shadcn/ui + `ai-elements` to support real components and a tight Tailwind design system; (2) a `github_search` tool was added to cover recency + native GitHub search operators the in-memory store can't serve; (3) the Phase 2 binary was moved under `cmd/wails-triage/` for conventional Go layout. This change reconciles the shipped system with its specs.

## What Changes

- **BREAKING** Replace `frontend/index.html` single-file CDN UI with a Vite-built React + TypeScript app under `frontend/src/`. `frontend/dist/` is committed and served at `/`.
- Add `github_search` tool: live GitHub Issues Search API fallback for queries the local corpus can't answer (recency, `label:`, `is:open`, `author:`).
- Move Phase 2 entry from repo-root `main.go` to `cmd/wails-triage/` (with `api.go`, `ws.go` as siblings). Startup command is now `go run ./cmd/wails-triage`.
- Replace `PORT` default of `8080` with OS-picked free port when unset; chosen URL is logged and opened in the browser.
- Add `log` and `quick_actions` frame types to the WebSocket protocol so the UI can render server log lines as chain-of-thought entries and deterministic follow-up suggestion pills.
- Split embeddings and chat clients: embeddings use `OPENAI_API_KEY` / `OPENAI_BASE_URL`; chat uses `LLM_API_KEY` (falls back to `OPENAI_API_KEY`) / `LLM_BASE_URL`. Enables routing chat to OpenRouter while keeping embeddings on OpenAI.

## Capabilities

### New Capabilities
<!-- None — all changes modify existing capabilities. -->

### Modified Capabilities
- `chat-ui`: UI is a Vite-built React + TypeScript app served from `frontend/dist`, not a single-file CDN page. Adds rendering for `log` and `quick_actions` frames.
- `triage-agent`: Tool catalogue grows from 4 to 5 (adds `github_search`). Startup command is `go run ./cmd/wails-triage`. `PORT` unset → OS picks a free port. Chat endpoint uses `LLM_API_KEY` / `LLM_BASE_URL` independent of embeddings.

## Impact

- **Code layout**: `main.go` / `api.go` / `ws.go` moved to `cmd/wails-triage/`. `frontend/index.html` removed; `frontend/src/`, `frontend/dist/`, `frontend/package.json`, `frontend/vite.config.ts` added.
- **New deps (frontend)**: Vite, React 18, TypeScript, Tailwind, shadcn/ui primitives, `ai-elements` components (copied into `src/components/ai-elements/`).
- **New deps (backend)**: none.
- **Build**: `frontend/dist/` is committed — operators still run Go only (`go run ./cmd/wails-triage`), no `npm install` required. Maintainers rebuild with `npm install && npm run build` before committing dist.
- **WS protocol**: additive (`FrameLog`, `FrameActions`) — no existing frame shape changes.
- **Env**: `LLM_API_KEY` is new (optional, falls back to `OPENAI_API_KEY`). `PORT` semantics change (default behaviour only; explicit values still honoured).
- **README**: updated Phase 2 command.
