## 1. Frontend Vite migration

- [x] 1.1 Scaffold Vite + React + TypeScript under `frontend/` with Tailwind + shadcn/ui
- [x] 1.2 Copy `ai-elements` components into `frontend/src/components/ai-elements/`
- [x] 1.3 Port chat UI (`App.tsx`), issue panel (`IssuePanel.tsx`), triage dialog (`TriageDialog.tsx`) from the CDN prototype
- [x] 1.4 Wire WebSocket hook with streaming frame handling
- [x] 1.5 Configure Vite for deterministic asset filenames + base path `./`
- [x] 1.6 Build and commit `frontend/dist/`

## 2. Go binary layout

- [x] 2.1 Move `main.go`, `api.go`, `ws.go` to `cmd/wails-triage/`
- [x] 2.2 Update `http.Dir("frontend/dist")` (still repo-relative, still works)
- [x] 2.3 Update README to `go run ./cmd/wails-triage`

## 3. Port handling

- [x] 3.1 Bind to `:0` when `PORT` is unset; log resolved URL
- [x] 3.2 Auto-open resolved URL in default browser (platform switch)

## 4. Tool catalogue: `github_search`

- [x] 4.1 Add `SearchIssues(ctx, query, limit)` to `internal/github` with repo-scope injection
- [x] 4.2 Add tool schema to `internal/tools/tools.go`
- [x] 4.3 Add `github_search.go` handler with limit clamp [1,30]
- [x] 4.4 Update system prompt to teach model when to fall back to live search
- [x] 4.5 Register in dispatcher switch

## 5. WS protocol additions

- [x] 5.1 Add `FrameLog` and `FrameActions` to `wsproto/frames.go`
- [x] 5.2 Add `QuickAction` struct (label + prompt)
- [x] 5.3 Implement `internal/logbus` fanout + `Tee` helper
- [x] 5.4 Subscribe per WebSocket connection, forward as `FrameLog`
- [x] 5.5 Emit `FrameActions` from `search_issues`, `github_search`, `get_issue`, `check_duplicate`

## 6. Chat / embeddings client split

- [x] 6.1 Honour `LLM_BASE_URL` / `LLM_API_KEY` / `LLM_MODEL` on the chat client
- [x] 6.2 Keep embeddings on `OPENAI_API_KEY` / `OPENAI_BASE_URL`
- [x] 6.3 Fall back `LLM_API_KEY` → `OPENAI_API_KEY` when unset

## 7. Docs

- [x] 7.1 Update `docs/SPEC-overview.md` env table + tool diagram + repo structure

## 8. Verification

- [x] 8.1 `go build ./...` + `go test ./...` clean after layout move
- [x] 8.2 Server boots, loads corpus, serves UI — manual verify
- [x] 8.3 `github_search` returns issues created after last ingestion — verified via running server
- [x] 8.4 UI renders log frames as chain-of-thought entries — verified via running server
- [x] 8.5 Quick-action pills appear after tool calls — verified via running server
