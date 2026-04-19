## Why

Build a GitHub issue triage system targeting the Wails repo corpus (~31k stars). Spec at `docs/SPEC-overview.md` defines two phases; this change delivers Phase 1 only — offline embedding pipeline producing a committed `issues.db` artefact. Phase 2 (agent, server, UI) deferred.

## What Changes

- Add standalone CLI `cmd/import-issues` runnable via `go run ./cmd/import-issues`.
- Fetch issues (title, body, labels, state, number, URL) from configured repo via GitHub API with pagination.
- Embed each issue via OpenAI `text-embedding-3-small` (1536 dim).
- Persist to `issues.db` at repo root: `issues` metadata table + `vec_issues` `vec0` virtual table.
- Add shared `internal/github` (API client) and `internal/store` (SQLite schema + writer).
- Add `.env.example` with `GITHUB_TOKEN`, `OPENAI_API_KEY`, `GITHUB_REPO`.

## Capabilities

### New Capabilities
- `issue-ingestion`: Phase 1 pipeline — fetch GitHub issues, generate embeddings, persist SQLite corpus.

### Modified Capabilities
<!-- None — greenfield. -->

## Impact

- New repo layout: `cmd/import-issues/`, `internal/github/`, `internal/store/`.
- New deps: GitHub Go client, OpenAI-compatible client, `sqlite-vec` (`vec0`), SQLite driver.
- External services: GitHub API (read), OpenAI API (embeddings).
- Committed artefact: `issues.db` in repo root (binary).
- Deferred to later change: Phase 2 agent loop, HTTP server, WebSocket, React UI, `check_duplicate`, in-memory VectorStore.
- Out of scope: GitHub writes, auth, webhooks, prod retry/error handling.
