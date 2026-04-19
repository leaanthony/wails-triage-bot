## Why

Phase 1 (`bootstrap-triage-system`) produced the committed `issues.db` corpus. Phase 2 turns the static artefact into an interactive triage tool — accept natural-language queries, reason with tools over the corpus, return structured responses. Matches Phase 2 of `docs/SPEC-overview.md`.

## What Changes

- Add HTTP server (`main.go`) started via `go run .` — serves UI, opens WebSocket, runs per-connection agent session.
- Add in-memory `VectorStore` in `internal/store` — loaded from `issues.db` at startup; supports KNN + runtime `Add`.
- Add `internal/agent` ReAct loop using OpenAI-compatible chat API with tool calling; streams tokens + tool events over a channel/writer.
- Add `internal/tools` — schemas + dispatch for:
  - `search_issues` (keyword over in-memory store)
  - `get_issue` (store lookup, fall through to GitHub fetch + embed + add)
  - `import_issues` (fetch + embed whole repo, add to store live)
  - `check_duplicate` (two-stage: KNN top-5 → LLM reasoning → confidence tiers)
- Add single-file React chat UI at `frontend/index.html` served from `/`.
- Add env: `LLM_BASE_URL`, `LLM_MODEL` (default `gpt-4o`), `PORT` (default `8080`).

## Capabilities

### New Capabilities
- `vector-store`: In-memory KNN store loaded from SQLite `vec0` virtual table; supports runtime `Add`.
- `triage-agent`: ReAct loop with OpenAI-compatible tool calling; streams tokens + tool events.
- `duplicate-detection`: Two-stage KNN + LLM reasoning with confidence tiers (≥0.85 auto-close, 0.60–0.84 review, <0.60 not dup).
- `chat-ui`: Single-file React UI over WebSocket; renders streamed tokens + tool-call indicators.

### Modified Capabilities
- `issue-ingestion`: `internal/github` gains a `GetIssue(number)` single-issue fetch used by the `get_issue` tool. No change to existing requirements; additive.

## Impact

- New code: `main.go`, `internal/agent/`, `internal/tools/`, `frontend/index.html`; `internal/store` gains in-memory `VectorStore`.
- New deps: WebSocket lib (`nhooyr.io/websocket` or `gorilla/websocket`). Chat completions use existing `sashabaranov/go-openai`.
- External services: GitHub API (read single issues on demand), OpenAI-compatible chat + embeddings.
- Requires `issues.db` present at startup (produced by Phase 1).
- Out of scope: GitHub writes, auth, conversation persistence, webhooks, prod retry/error handling.
