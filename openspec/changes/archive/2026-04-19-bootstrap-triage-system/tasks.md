## 1. Project setup

- [x] 1.1 Init Go module and commit `go.mod`
- [x] 1.2 Create repo layout: `cmd/import-issues/`, `internal/github/`, `internal/store/`
- [x] 1.3 Add `.env.example` with `GITHUB_TOKEN`, `OPENAI_API_KEY`, `GITHUB_REPO`
- [x] 1.4 Add `.gitignore` (env files, build output) and `.gitattributes` marking `issues.db` binary
- [x] 1.5 Pick and vendor deps: GitHub client, OpenAI-compatible client, `sqlite-vec`, SQLite driver

## 2. Shared GitHub client (`internal/github`)

- [x] 2.1 Define `Issue` struct (number, title, body, labels, state, URL)
- [x] 2.2 Implement paginated `ListIssues(owner, repo)` with authenticated PAT
- [x] 2.3 Filter out pull requests from results
- [x] 2.4 Handle secondary rate-limit (basic sleep/retry)

## 3. Shared store (`internal/store`)

- [x] 3.1 Define SQLite schema: `issues` table + `vec_issues` `vec0` virtual table (1536 dim)
- [x] 3.2 Implement `OpenDB(path)` that creates schema if missing
- [x] 3.3 Implement `UpsertIssue(issue, vector)` writing both tables atomically
- [x] 3.4 Unit test round-trip: upsert then read back metadata + vector

## 4. Phase 1 — import CLI (`cmd/import-issues`)

- [x] 4.1 Parse env; fail fast on missing required vars
- [x] 4.2 Fetch all issues for `GITHUB_REPO` via `internal/github`
- [x] 4.3 Embed each issue (title + body) via OpenAI `text-embedding-3-small`
- [x] 4.4 Upsert into temp DB file via `internal/store`; rename to `issues.db` on success
- [x] 4.5 Log progress (count, tokens, elapsed) to stdout
- [x] 4.6 Smoke-run against `wailsapp/wails` (20-issue cap verified; full run/commit deferred to user)

## 5. Docs + verification

- [x] 5.1 README section: setup, env vars, how to run Phase 1
- [x] 5.2 Verify `issues.db` opens and `SELECT COUNT(*)` matches fetched issue count
- [x] 5.3 Verify vector KNN query against `vec_issues` returns results
