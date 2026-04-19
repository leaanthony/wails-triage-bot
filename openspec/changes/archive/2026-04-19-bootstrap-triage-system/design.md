## Context

Greenfield Go repo. Deliver Phase 1 of `docs/SPEC-overview.md` only: offline embedding pipeline producing committed `issues.db`. Demo corpus: `wailsapp/wails` (~31k stars). Phase 2 (agent, HTTP server, React UI, duplicate detection) deferred to a later change — keep `internal/github` and `internal/store` shaped so Phase 2 can consume them without refactor.

## Goals / Non-Goals

**Goals:**
- One-shot CLI producing a static artefact consumable by Phase 2 later.
- Shared packages (`internal/github`, `internal/store`) usable by Phase 2 without change.
- `issues.db` small and portable; commit to repo so clones get corpus free.
- Clear progress logging for long runs.

**Non-Goals:**
- Any Phase 2 concerns: agent loop, HTTP server, WebSocket, React UI, `check_duplicate` tool, in-memory VectorStore.
- Incremental re-embedding (full recompute each run).
- Rate-limit backoff beyond basic sleep on secondary limit.
- Writes to GitHub.

## Decisions

### SQLite + sqlite-vec (`vec0` virtual table) for corpus storage
Commit binary `issues.db` to repo. Clone = instant corpus, no API cost downstream.
Alternatives: Postgres+pgvector (server required), Pinecone (external, paid), flat JSON (no SQL KNN later). `sqlite-vec` gives embedded SQL KNN in one file.

### `text-embedding-3-small` (1536 dim)
Good quality/cost ratio; matches spec. `3-large` (3072 dim) = 2× cost, marginal gain at this scale.

### Standalone CLI under `cmd/import-issues`
Own `main.go` keeps Phase 1 independent of future Phase 2 server. No shared entry point.

### Upsert semantics
Re-runs replace rows by `(repo, issue_number)` key — simplest reproducibility. No diff/incremental logic.

### Embed title + body concatenation
Single string `title + "\n\n" + body` per issue. Matches spec intent. Alternatives (separate embeddings, weighted sums) add complexity without clear win at this scale.

### Full-corpus single run
No resume / checkpoint. If run fails mid-way, re-run from scratch. Simplifies code; acceptable for one-shot artefact.

## Risks / Trade-offs

- [Committed `issues.db` grows repo size] → Accept for demo; `.gitattributes` mark binary; document in README.
- [OpenAI embedding cost] → One-time; log token counts; optional issue-count cap flag if needed.
- [GitHub secondary rate-limit during import] → Authenticated PAT (5000 req/hr); paginate; sleep-retry on 403 secondary.
- [`sqlite-vec` Cgo dep] → Verify local build; note platform caveats in README.
- [Phase 2 shape drift] → Keep `Issue` struct and store API minimal and generic so Phase 2 consumes without change; review before committing package boundaries.
- [Run fails mid-way, partial DB committed] → Treat `issues.db` as atomic: write to temp file, rename on success.

## Open Questions

- Include closed issues or open-only? Spec implies all — default to all, confirm in impl.
- Max issue count for demo run (wails has ~4k issues — run full set or sample)?
- Pull requests: GitHub API returns PRs as issues — filter out or keep? Default: filter out (PRs ≠ issues for triage).
