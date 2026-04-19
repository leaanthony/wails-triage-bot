## RENAMED Requirements

- FROM: `### Requirement: Single-file React UI`
- TO: `### Requirement: Vite-built React UI`

## MODIFIED Requirements

### Requirement: Vite-built React UI
System SHALL provide a Vite-built React + TypeScript app under `frontend/src/`, with the pre-built bundle committed to `frontend/dist/` and served statically by the Phase 2 server at `/`. Operators SHALL NOT need Node or `npm` to run the system.

#### Scenario: Served at root
- **WHEN** browser requests `/`
- **THEN** server returns `frontend/dist/index.html` and its referenced `frontend/dist/assets/*`

#### Scenario: Clone-and-run workflow
- **WHEN** an operator clones the repo and runs `go run ./cmd/wails-triage`
- **THEN** server starts with no Node toolchain required; `frontend/dist/` is already committed

#### Scenario: Maintainer rebuild
- **WHEN** a maintainer changes UI source under `frontend/src/`
- **THEN** they run `npm install && npm run build` and commit the updated `frontend/dist/` alongside the source change

## ADDED Requirements

### Requirement: Log frame rendering
UI SHALL render `log` frames as entries in the chain-of-thought stream so operators can watch server-side progress alongside agent reasoning.

#### Scenario: Log line arrives mid-turn
- **WHEN** a `log` frame with `data` arrives while an assistant turn is streaming
- **THEN** UI appends a muted log entry to the current chain-of-thought panel without disrupting the active assistant bubble

### Requirement: Quick-action suggestion pills
UI SHALL render `quick_actions` frames as clickable suggestion pills that, when clicked, submit the associated prompt as a new user message.

#### Scenario: Pills appear after tool result
- **WHEN** a `quick_actions` frame with a non-empty `actions` array arrives
- **THEN** UI renders each action as a pill showing `label`; clicking a pill submits `prompt` as the next user message

#### Scenario: Pills are ephemeral
- **WHEN** a new user message is sent
- **THEN** previously rendered pills are cleared — the pill set is owned by the most recent tool result, not the conversation
