## ADDED Requirements

### Requirement: Single-file React UI
System SHALL provide `frontend/index.html` as a single HTML file using React + Babel standalone from a CDN, served by the Phase 2 server with no build step.

#### Scenario: Served at root
- **WHEN** browser requests `/`
- **THEN** server returns `frontend/index.html` containing the React chat app

### Requirement: WebSocket chat transport
UI SHALL exchange chat messages with the server over a WebSocket connection at `/ws`.

#### Scenario: User sends message
- **WHEN** user submits a chat message
- **THEN** UI sends the message over WebSocket and renders incoming `token` frames into the active assistant bubble

#### Scenario: Tool-call visibility
- **WHEN** a `tool_call` frame arrives
- **THEN** UI renders a pill indicator with the tool name and an argument summary

#### Scenario: Tool-result visibility
- **WHEN** a `tool_result` frame arrives
- **THEN** UI updates the corresponding pill to show ok/error status

#### Scenario: Done frame
- **WHEN** a `done` frame arrives
- **THEN** UI finalizes the assistant bubble and re-enables the input
