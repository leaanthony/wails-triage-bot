# vector-store Specification

## Purpose
TBD - created by archiving change add-triage-agent. Update Purpose after archive.
## Requirements
### Requirement: In-memory corpus loader
System SHALL load the full `issues.db` corpus into an in-memory VectorStore at server startup, before HTTP listener accepts connections.

#### Scenario: Startup load
- **WHEN** server starts and `issues.db` exists
- **THEN** every issue metadata row and corresponding embedding is loaded into memory

#### Scenario: Missing DB
- **WHEN** `issues.db` is absent at startup
- **THEN** server exits non-zero with a message pointing to `cmd/import-issues`

### Requirement: KNN search by vector
VectorStore SHALL return the top-K issues nearest to a query vector by cosine distance.

#### Scenario: Top-5 retrieval
- **WHEN** caller invokes KNN with a 1536-dim query vector and K=5
- **THEN** store returns up to 5 issues ordered by ascending cosine distance

#### Scenario: K larger than corpus
- **WHEN** K exceeds the number of issues in the store
- **THEN** store returns every issue ordered by distance without error

### Requirement: Add issue at runtime
VectorStore SHALL accept new issues (metadata + vector) added during a session and include them in subsequent KNN queries and metadata lookups.

#### Scenario: Ad-hoc addition
- **WHEN** a tool adds a freshly-embedded issue to the store
- **THEN** the next KNN query can return that issue as a neighbour

### Requirement: Concurrent-safe access
VectorStore SHALL serialize writes with a mutex so that concurrent `Add` and KNN calls do not corrupt state.

#### Scenario: Parallel readers + writer
- **WHEN** multiple goroutines call KNN while another calls `Add`
- **THEN** no data race occurs and readers see a consistent snapshot

