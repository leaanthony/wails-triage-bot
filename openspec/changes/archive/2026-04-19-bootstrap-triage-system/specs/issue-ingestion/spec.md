## ADDED Requirements

### Requirement: Standalone import CLI
System SHALL provide a standalone binary `cmd/import-issues` runnable via `go run ./cmd/import-issues` that fetches issues from a configured GitHub repository, embeds them, and persists the corpus to `issues.db`.

#### Scenario: Successful import run
- **WHEN** operator runs `go run ./cmd/import-issues` with `GITHUB_TOKEN`, `OPENAI_API_KEY`, and `GITHUB_REPO` set
- **THEN** CLI fetches all issues via GitHub API, generates embeddings via OpenAI, writes `issues.db`, and logs progress

#### Scenario: Missing credentials
- **WHEN** required env var is unset
- **THEN** CLI exits non-zero with message naming the missing variable

### Requirement: GitHub issue fetch
System SHALL fetch issues (title, body, labels, state, number, URL) for the configured repository using authenticated GitHub API calls with pagination.

#### Scenario: Paginated fetch
- **WHEN** repository has more issues than one API page
- **THEN** CLI walks pages until exhausted and collects all results

#### Scenario: Default repository
- **WHEN** `GITHUB_REPO` is unset in Phase 1
- **THEN** CLI errors; repo is required in Phase 1

### Requirement: Issue embedding
System SHALL generate a 1536-dimension vector embedding per issue using OpenAI `text-embedding-3-small` over a concatenation of title and body.

#### Scenario: Embedding produced
- **WHEN** an issue has non-empty title or body
- **THEN** a `[]float32` of length 1536 is produced and associated with the issue

### Requirement: SQLite persistence
System SHALL persist issue metadata to an `issues` table and vectors to a `vec_issues` `vec0` virtual table in `issues.db` at repository root.

#### Scenario: Fresh DB write
- **WHEN** import completes
- **THEN** `issues.db` contains every fetched issue's metadata row and corresponding vector row keyed by issue number

#### Scenario: Re-run overwrites
- **WHEN** operator re-runs the importer against existing `issues.db`
- **THEN** previous rows are replaced so the DB reflects the latest fetch
