# wails-triage-bot

A GitHub issue triage agent for the [Wails](https://github.com/wailsapp/wails)
project. Ask natural-language questions about the backlog; get streamed answers
with tool calls, an issue panel, and a two-stage duplicate checker (KNN +
LLM reasoning).

The corpus (`issues.db`, ~2,400 embedded issues) is committed — **you do not need to rebuild it**. Clone,
set two env vars, run one command.

## Run it (≈30 seconds)

```sh
cp .env.example .env
# edit .env — fill in GITHUB_TOKEN and OPENAI_API_KEY
go run ./cmd/wails-triage
```

The server picks a free port, logs the URL, and opens it in your browser.

### Required

| Var              | What it's for                                              |
| ---------------- | ---------------------------------------------------------- |
| `GITHUB_TOKEN`   | Used by `get_issue` / `github_search` / `import_issues`. A PAT with `issues:read` is enough. |
| `OPENAI_API_KEY` | Embeddings (`text-embedding-3-small`) and — by default — chat. |
| `GITHUB_REPO`    | `owner/repo`. Pre-filled in `.env.example` as `wailsapp/wails`. |

### Optional

| Var             | Default          | Effect                                               |
| --------------- | ---------------- | ---------------------------------------------------- |
| `LLM_MODEL`     | `gpt-4o`         | Chat model.                                          |
| `LLM_BASE_URL`  | OpenAI           | Route chat elsewhere (e.g. OpenRouter). Embeddings stay on OpenAI. |
| `LLM_API_KEY`   | `OPENAI_API_KEY` | Chat-only key. Falls back to the OpenAI key.         |
| `PORT`          | OS-picked        | Force a specific port.                               |
| `ISSUES_DB`     | `issues.db`      | Corpus path.                                         |

## What to try

Open the UI and ask:

- **"Any issues about WebView2 on Windows?"** — keyword search over the corpus.
- **"Anything opened in the last week?"** — date filter; falls through to live
  GitHub search if the corpus is stale.
- **"Triage #3889"** — runs the two-stage duplicate check (top-5 KNN + LLM
  reasoning) and returns a tier: `recommend_auto_close` / `human_review` /
  `not_duplicate`.
- Click any row in a search result to see the full issue in the side panel;
  click a **Triage** button to run the duplicate check without leaving the UI.

## Requirements

- Go 1.24+ (see `.tool-versions`)
- C toolchain (`gcc`) — `sqlite-vec` is loaded via cgo

## Docs

- [`docs/SPEC-overview.md`](docs/SPEC-overview.md) — architecture, two-phase
  design, duplicate-detection rationale.
- [`openspec/specs/`](openspec/specs/) — canonical capability specs (scenario-based
  acceptance criteria).
- [`openspec/changes/archive/`](openspec/changes/archive/) — decision history:
  three proposal → design → tasks → delta-spec bundles showing how the system
  was built and revised.

## Rebuilding the corpus (optional)

If you want to rebuild against a different repo:

```sh
go run ./cmd/import-issues
```

Takes ~5 minutes for a ~5,000-issue repo. Writes atomically (temp file +
rename) so a crash leaves the existing corpus intact.

## Tests

```sh
go test ./...
```
