# triage-agent Specification

## Purpose
TBD - created by archiving change add-triage-agent. Update Purpose after archive.
## Requirements
### Requirement: HTTP server entry point
System SHALL provide a Go HTTP server started via `go run ./cmd/wails-triage` that serves the chat UI and a WebSocket endpoint for agent conversations.

#### Scenario: Startup with unset PORT
- **WHEN** operator runs `go run ./cmd/wails-triage` with `PORT` unset and `issues.db` present
- **THEN** server binds to an OS-picked free port, logs the resolved URL, auto-opens it in the default browser, serves `frontend/dist/index.html` at `/`, and accepts WebSocket upgrades at `/ws`

#### Scenario: Startup with explicit PORT
- **WHEN** operator runs `go run ./cmd/wails-triage` with `PORT=8080`
- **THEN** server binds to `:8080` and behaves identically otherwise

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
Agent SHALL expose `search_issues`, `github_search`, `get_issue`, `import_issues`, and `check_duplicate` to the LLM with JSON-schema-declared parameters.

#### Scenario: Tool catalogue
- **WHEN** agent constructs the chat completion request
- **THEN** all five tools are declared in the `tools` field

#### Scenario: search_issues
- **WHEN** agent calls `search_issues` with a keyword query
- **THEN** tool returns matching issues from the in-memory store, ranked by relevance

#### Scenario: github_search
- **WHEN** agent calls `github_search` with a query the in-memory corpus cannot answer (recent issues, `label:`, `is:open`, `author:`, etc.)
- **THEN** tool issues a live GitHub Issues Search API request scoped to the configured repo, filters pull requests out, and returns up to the caller-requested limit (default 10, max 30)

#### Scenario: get_issue
- **WHEN** agent calls `get_issue` with an issue number
- **THEN** tool returns metadata from the store; if absent, it fetches from GitHub, embeds, adds to the store, and returns the metadata tagged with source `"store"` or `"github"`

#### Scenario: import_issues
- **WHEN** agent calls `import_issues`
- **THEN** tool fetches every issue from the configured repo, skips issues already in the store, embeds the rest, and reports `{fetched, added, tokens}`

### Requirement: Swappable LLM endpoint
System SHALL allow overriding the chat model base URL, API key, and model name via `LLM_BASE_URL`, `LLM_API_KEY`, and `LLM_MODEL` (OpenAI-compatible). Embedding calls SHALL continue to use `OPENAI_API_KEY` / `OPENAI_BASE_URL` independently so chat and embedding providers can differ.

#### Scenario: Chat routed elsewhere
- **WHEN** `LLM_BASE_URL` is set (e.g. OpenRouter)
- **THEN** chat completions go to that URL using `LLM_API_KEY` and `LLM_MODEL`; embeddings still go to OpenAI

#### Scenario: Chat key falls back
- **WHEN** `LLM_API_KEY` is unset
- **THEN** chat client uses `OPENAI_API_KEY`

#### Scenario: Default model
- **WHEN** `LLM_MODEL` is unset
- **THEN** agent uses `gpt-4o`

