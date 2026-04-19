## Context

Phase 1 shipped. `issues.db` exists with issue metadata + 1536-dim embeddings (mean-pooled, unit-normalized). Phase 2 adds an interactive agent over the static corpus: HTTP server, WebSocket chat, ReAct loop with tool calls, duplicate detection. Corpus loads into memory once at startup — no per-query SQLite hits.

## Goals / Non-Goals

**Goals:**
- Load `issues.db` into memory at startup and serve KNN from RAM.
- Streaming responses (tokens + tool events) to the browser with low latency.
- Four agent tools with clean, JSON-schema-declared signatures.
- Swappable LLM endpoint via `LLM_BASE_URL` / `LLM_MODEL`.
- Runtime `import_issues` extends the live store without restart.
- Single-file UI (no build step) — copy-paste to deploy.

**Non-Goals:**
- GitHub writes (recommendations only).
- Auth / multi-user / session persistence across reconnects.
- Re-embedding of existing issues.
- Webhook ingestion.
- Production-grade retry/error handling beyond basic rate-limit sleep (already in `internal/github`).

## Decisions

### In-memory VectorStore with brute-force KNN
Corpus size ~thousands × 1536 float32 ≈ a few MB. Brute-force cosine over slice of vectors is microseconds; no ANN index needed.
Alternative: query `vec_issues` live via SQL — extra latency, no benefit at this scale.

### VectorStore is the single source of truth for issues at runtime
On startup, load all `(Issue, vector)` into one struct. `Add(issue, vector)` extends it. Tools read only from this — no per-tool-call SQLite open.
Alternative: thin wrapper that queries SQLite each tool call — more moving parts, no win.

### WebSocket transport, JSON envelope
Frame types: `{"type":"token","data":"..."}`, `{"type":"tool_call","name":"...","args":{...}}`, `{"type":"tool_result","name":"...","ok":true}`, `{"type":"done"}`, `{"type":"error","msg":"..."}`. Bidirectional, trivial to stream.
Alternative: SSE — unidirectional, awkward for bidirectional chat events.
Library: `nhooyr.io/websocket` (cleaner API, context-first) over `gorilla/websocket`.

### ReAct loop via OpenAI tool-calling
Pass tool schemas in every `ChatCompletion` request. On `tool_calls` response, execute each, append tool messages, loop. Cap iterations at 8 to prevent runaway.
Alternative: hand-rolled JSON protocol / LangChain — reinvents or adds weight.

### Embedding on demand via same `text-embedding-3-small`
`check_duplicate` embeds target issue text; `get_issue` + `import_issues` embed newly-fetched issues. Same model/tokenizer/chunk logic from Phase 1 — move chunking helpers from `cmd/import-issues` into a shared `internal/embed` package.
Alternative: duplicate code — drift risk.

### Two-stage duplicate detection
Stage 1: embed target → KNN top-5 from store. Stage 2: single chat completion with system prompt instructing JSON output `{is_duplicate, confidence, reasoning, candidates:[{number, verdict}]}`. Parse + apply tiers.
Confidence tiers fixed per spec: ≥0.85 auto-close, 0.60–0.84 review, <0.60 not-dup.

### Single-file React UI via CDN
React + Babel standalone from esm.sh or unpkg. Zero build. Minimal state: message list + input. Renders `token` frames into the current assistant bubble; `tool_call` frames as pill indicators.
Alternative: Vite/Next build — overkill.

### Tool-call visibility
Emit `tool_call` frame before dispatch, `tool_result` frame after. UI shows a small badge per call. Helps demonstrate agentic behaviour.

## Risks / Trade-offs

- [WebSocket disconnect mid-stream] → Prototype scope: client reconnects, starts fresh turn. No resume.
- [Long ReAct loops burn tokens] → Hard cap iterations (8); log warning on hit.
- [LLM returns malformed JSON for duplicate] → Strict system prompt, retry once with "return valid JSON" hint, fail closed as not-a-duplicate on second failure.
- [In-memory store race on concurrent `Add`] → Single `sync.RWMutex`; `Add` takes write lock, KNN takes read.
- [`import_issues` spends real money on large repos] → Log token-count per tool call; UI shows token count in `tool_result` frame so user sees cost.
- [CDN dependency for UI] → Accept for prototype; document offline fallback (vendor two JS files).

## Migration Plan

Additive; depends on Phase 1 artefact. Deploy:
1. `issues.db` present (from Phase 1).
2. `go run .` → server listens on `PORT`.
3. Browser → `http://localhost:8080/`.

Rollback: stop the server; `issues.db` is untouched unless `import_issues` was used (which writes to RAM only, not DB — design decision to keep DB immutable at runtime).

## Open Questions

- Persist `import_issues` additions back to `issues.db`? Current design: RAM only, ephemeral. Pro: simple, safe. Con: restart loses additions. Default: ephemeral; revisit if useful.
- `check_duplicate` input: issue number (look up in store) or free-text? Support both — tool takes either `number` or `text`.
- KNN candidate count: fixed at 5 per spec, or configurable? Fixed.
- Tool-call visibility UI: name only vs name + arg summary? Name + truncated arg summary.
