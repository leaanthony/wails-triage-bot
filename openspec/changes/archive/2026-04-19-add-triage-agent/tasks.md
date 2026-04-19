## 1. Shared embed helper

- [x] 1.1 Extract chunking + embedding logic from `cmd/import-issues` into `internal/embed` (tokenizer, chunkByTokens, mean-pool, unit-normalize)
- [x] 1.2 Refactor `cmd/import-issues` to use `internal/embed` — no behaviour change

## 2. Shared GitHub client gains

- [x] 2.1 Add `GetIssue(ctx, number)` to `internal/github` with PR detection
- [ ] 2.2 Unit test happy path (mock transport or live smoke) and PR-number error — deferred; live smoke via agent session covers this

## 3. In-memory VectorStore (`internal/store`)

- [x] 3.1 Add `VectorStore` struct: slice of `(Issue, []float32)` + `sync.RWMutex`
- [x] 3.2 Implement `LoadVectorStore(db)` — read `issues` + `vec_issues` into memory
- [x] 3.3 Implement `KNN(query []float32, k int)` brute-force cosine
- [x] 3.4 Implement `Add(issue, vector)` write-lock append
- [x] 3.5 Implement `GetByNumber(n)` in-memory lookup
- [x] 3.6 Unit tests: KNN ordering, `Add` visibility, K > N case (plus race test with `-race`)

## 4. Agent tools (`internal/tools`)

- [x] 4.1 Define tool schemas (`name`, `description`, JSON params) for all four tools
- [x] 4.2 Implement `search_issues` — case-insensitive keyword match over title+body, ranked by hit count, top N
- [x] 4.3 Implement `get_issue` — store hit → return; miss → fetch + embed + `Add` → return
- [x] 4.4 Implement `import_issues` — full repo fetch + embed + bulk `Add`; reports count + tokens
- [x] 4.5 Implement `check_duplicate` stage 1 — resolve target (number or free text), embed, KNN top-5
- [x] 4.6 Implement `check_duplicate` stage 2 — LLM structured JSON; retry once on parse failure
- [x] 4.7 Apply confidence tiers (`recommend_auto_close` / `human_review` / `not_duplicate`)
- [x] 4.8 Central dispatcher: `Dispatch(ctx, name, args json.RawMessage) (json.RawMessage, error)`

## 5. Agent loop (`internal/agent`)

- [x] 5.1 Chat client config honouring `LLM_BASE_URL`, `LLM_MODEL` (default `gpt-4o`), separate from embeddings client
- [x] 5.2 ReAct loop: send messages + tools, stream response, handle `tool_calls`, append tool messages, repeat
- [x] 5.3 Emit `token` / `tool_call` / `tool_result` / `done` / `error` frames on a channel
- [x] 5.4 Hard cap: 8 iterations; emit `error` frame on hit
- [x] 5.5 System prompt: describe role, tools, when to call which

## 6. HTTP server (`main.go`)

- [x] 6.1 Env parse; fail fast on missing `OPENAI_API_KEY` or absent `issues.db`
- [x] 6.2 Open DB, build `VectorStore`, load corpus
- [x] 6.3 Serve `frontend/index.html` as static asset on `/`
- [x] 6.4 `/ws` WebSocket endpoint: upgrade, read user messages, run agent session per connection, forward frames as JSON
- [x] 6.5 Graceful shutdown on SIGINT

## 7. Chat UI (`frontend/index.html`)

- [x] 7.1 Single HTML file with React + Babel standalone via CDN
- [x] 7.2 WebSocket client; render streamed `token` frames into active assistant bubble
- [x] 7.3 Render `tool_call` pills with name + truncated args; update on `tool_result`
- [x] 7.4 Input box disabled while streaming; re-enable on `done` or `error`
- [x] 7.5 Minimal styling — monospace transcript, scrollable

## 8. Docs + verification

- [x] 8.1 Extend README with Phase 2 run instructions + env vars
- [ ] 8.2 Manual verify: `search_issues` returns expected hits — user runs server
- [ ] 8.3 Manual verify: `check_duplicate` against a known-duplicate issue returns `recommend_auto_close` or `human_review` — user runs server
- [ ] 8.4 Manual verify: UI streams tokens incrementally and shows tool-call pills — user runs server
