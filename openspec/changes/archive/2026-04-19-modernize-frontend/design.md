## Context

`add-triage-agent` mocked the UI with a single HTML file pulling React + Babel from a CDN. The choice let anyone clone and run the system with a single `go run .` command and no Node toolchain. It worked for the prototype but three pressures pushed a rewrite: (1) the chain-of-thought UX needed real components (`ai-elements`) rather than hand-rolled markup; (2) shadcn/ui primitives gave us the design system for free; (3) Tailwind + Vite made iteration cycles fast. Committing `frontend/dist/` preserves the "Go-only on clone" property that was the whole point of the CDN approach.

The 4-tool catalogue also proved incomplete in practice. The in-memory corpus is a snapshot — it has no issues filed after the last `import-issues` run. Users hit this on day one asking about recent bugs. Adding `github_search` as an explicit fallback tool (rather than hiding it inside `search_issues`) lets the system prompt teach the model *when* to reach for the live API and keeps the two modes auditable in the chain-of-thought stream.

Moving `main.go` out of repo root is a small thing but matches Go convention and makes room for additional binaries without root-level name collisions.

## Goals / Non-Goals

**Goals:**
- Preserve the "Go-only on clone" property: committed `dist/` means no `npm install` required.
- Reuse existing WS frame protocol with additive frame types (`log`, `quick_actions`) — no migration needed for any inbound client.
- Keep chat and embedding endpoints independently configurable.
- Standard Go layout under `cmd/`.

**Non-goals:**
- Server-side rendering. Static `dist/` + client hydration is enough.
- i18n.
- Backwards compat with the old single-file UI. Direct replacement, not a parallel implementation.
- UI testing harness beyond manual verify. This is still a prototype.

## Decisions

### Decision: Commit `frontend/dist/`, keep out of `.gitignore`
**What:** Built Vite output lives in the repo and ships with each commit.
**Why:** Preserves the zero-node-step clone-and-run property — the single compelling argument for the original CDN approach. Node toolchain is a maintainer concern, not a runtime concern.
**Alternatives:** (a) Require `npm install && npm run build` on clone — violates the property. (b) CI-built release bundle — too much infra for a prototype.

### Decision: `ai-elements` copy-in, not package import
**What:** Components are copied into `frontend/src/components/ai-elements/` (shadcn-style ownership), not imported from an npm package.
**Why:** Direct edits without fork/upstream friction. Matches the shadcn philosophy that powers the rest of the UI. Bundle size is not relevant for a local-only app.
**Alternatives:** Package import — cleaner updates, less flexibility. Rejected for the interview context.

### Decision: `github_search` as its own tool, not a `search_issues` flag
**What:** Separate schema, separate handler, distinct tool name visible in chain-of-thought frames.
**Why:** The two have different cost shapes (one is free + instant; the other is a network call + GitHub rate limit) and different inputs (one takes date filters; the other takes GitHub search operators). Forcing them into one schema obscures both. Keeping them separate lets the system prompt give clear rules about when each applies.
**Alternatives:** Overloaded `search_issues(source: "local"|"github")` — simpler schema count but harder to prompt and harder to trace in the UI.

### Decision: OS-picked port on unset `PORT`
**What:** When `PORT` is empty, server binds to `:0`, OS picks a free port, resolved URL is logged and auto-opened in browser.
**Why:** Every dev machine has something on 8080 already. Manual port negotiation is the #1 friction on first run. Explicit `PORT=8080` still works exactly as before.
**Alternatives:** Static default with a "port in use" retry loop — more code, worse UX when the assumed ports are all taken.

### Decision: Split chat and embeddings clients
**What:** Two separate `openai.Client` instances. Chat reads `LLM_*`. Embeddings read `OPENAI_*`.
**Why:** OpenRouter is cheaper for chat but doesn't offer embeddings. Operators who care about cost route chat elsewhere while keeping embeddings on OpenAI. The deployment surface maps 1:1 to the two business concerns.
**Alternatives:** Single client + model-name routing — works in theory, breaks when the underlying providers accept different model names or auth formats.

## Risks / Trade-offs

- **Committed `dist/` bloats the repo.** Each frontend change adds a build artefact to git history. Acceptable at this scale; would reconsider if this grew to a team project.
- **`github_search` depends on a live API.** An outage or rate-limit burst breaks the tool mid-conversation. Client rate-limit handling backs off; the LLM can route around the failure by re-prompting with stored-only searches.
- **Frame-type additions are additive, but `log` frame volume is unbounded.** Slow WS subscribers drop lines (by design). Debug users may miss lines without realising — noted in the LogBus review as a follow-up.

## Migration Plan

One-shot swap:
1. Delete `frontend/index.html` single-file UI.
2. Introduce `frontend/src/`, `frontend/package.json`, `frontend/vite.config.ts`, `frontend/tsconfig.json`, etc.
3. Build once, commit `frontend/dist/`.
4. Move `main.go`, `api.go`, `ws.go` → `cmd/wails-triage/`.
5. Update README: `go run ./cmd/wails-triage`.
6. Add `github_search` tool schema and handler.
7. Add `FrameLog`, `FrameActions` frame types + corresponding UI renderers (chain-of-thought log entries, suggestion pills).

No feature flags. The old UI stays dead.
