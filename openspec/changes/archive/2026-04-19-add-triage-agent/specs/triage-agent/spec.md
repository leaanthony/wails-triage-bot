## ADDED Requirements

### Requirement: HTTP server entry point
System SHALL provide a Go HTTP server started via `go run .` that serves the chat UI and a WebSocket endpoint for agent conversations.

#### Scenario: Startup
- **WHEN** operator runs `go run .` with required env vars set and `issues.db` present
- **THEN** server listens on `PORT` (default 8080), serves `frontend/index.html` at `/`, and accepts WebSocket upgrades at `/ws`

### Requirement: ReAct agent loop
System SHALL run a ReAct loop that sends user messages to the configured chat model with tool schemas, executes returned tool calls, feeds results back, and repeats until the model returns a final message or the iteration cap is hit.

#### Scenario: Single-tool turn
- **WHEN** user asks a question answerable with one tool call
- **THEN** agent calls the tool, receives its result, and produces a final answer in the same turn

#### Scenario: Multi-tool turn
- **WHEN** a user query requires chained tools
- **THEN** agent performs successive tool calls within one turn until it emits a final answer

#### Scenario: Iteration cap
- **WHEN** the loop reaches 8 tool-call iterations without a final message
- **THEN** agent aborts with an error frame rather than looping indefinitely

### Requirement: Streaming responses
System SHALL stream assistant tokens and tool-call events to the client over WebSocket as they are produced.

#### Scenario: Token streaming
- **WHEN** model emits assistant content
- **THEN** server forwards `token` frames incrementally, not only on completion

#### Scenario: Tool-call frame
- **WHEN** agent dispatches a tool call
- **THEN** server emits a `tool_call` frame with tool name and argument summary before execution

#### Scenario: Tool-result frame
- **WHEN** a tool call completes
- **THEN** server emits a `tool_result` frame with tool name and ok/error status

### Requirement: Agent tools
Agent SHALL expose `import_issues`, `search_issues`, `get_issue`, and `check_duplicate` to the LLM with JSON-schema-declared parameters.

#### Scenario: Tool catalogue
- **WHEN** agent constructs the chat completion request
- **THEN** all four tools are declared in the `tools` field

#### Scenario: search_issues
- **WHEN** agent calls `search_issues` with a keyword query
- **THEN** tool returns matching issues from the in-memory store, ranked by relevance

#### Scenario: get_issue
- **WHEN** agent calls `get_issue` with an issue number
- **THEN** tool returns metadata from the store; if absent, it fetches from GitHub, embeds, adds to the store, and returns the metadata

#### Scenario: import_issues
- **WHEN** agent calls `import_issues` with a repo not in the store
- **THEN** tool fetches, embeds, and adds issues to the in-memory store for use in later tool calls

### Requirement: Swappable LLM endpoint
System SHALL allow overriding the chat model base URL and model name via `LLM_BASE_URL` and `LLM_MODEL` with an OpenAI-compatible API. Embedding calls continue to use `OPENAI_API_KEY` / `OPENAI_BASE_URL`.

#### Scenario: Override used
- **WHEN** `LLM_BASE_URL` is set
- **THEN** chat completions go to that URL with the `LLM_MODEL` name

#### Scenario: Default model
- **WHEN** `LLM_MODEL` is unset
- **THEN** agent uses `gpt-4o`
