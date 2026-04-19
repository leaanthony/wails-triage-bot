# GitHub Issue Triage Agent — High Level Specification

## Overview

A two-phase system for triaging GitHub issues using LLM embeddings and agentic
reasoning.

The system works on any GitHub repository. The demo uses the Wails project
(Go framework for cross-platform desktop apps, ~31k GitHub stars) as the
target corpus.

---

## The Two Phases

### Phase 1 — Embedding Pipeline (offline, run once)

**Purpose:** Fetch GitHub issues, generate vector embeddings, persist to disk.

**Input:** A GitHub repository + API credentials  
**Output:** `issues.db` — a SQLite database containing issue metadata and
embeddings, committed to the repo

**Key characteristic:** This phase is expensive (API calls, time) but runs
once. The result is a static artefact that is committed to the repository.
Anyone who clones the repo gets the pre-built corpus for free — no API calls,
no waiting.

**Standalone binary:** `go run ./cmd/import-issues`

---

### Phase 2 — Triage Agent (online, runs continuously)

**Purpose:** Accept natural language queries from a user, reason about the
issue corpus using tools, and return structured responses.

**Input:** Natural language chat messages via a web UI  
**Output:** Agent responses streamed in real time — issue lists, duplicate
analysis, summaries

**Key characteristic:** The agent loads the pre-built corpus from `issues.db`
at startup (fast, no embedding needed) and uses GPT-4o tool calling to answer
queries. The duplicate detection combines fast vector similarity search against
the loaded corpus with a second LLM reasoning call for accuracy.

**Server:** `go run ./cmd/wails-triage`

---

## System Boundary Diagram

```
┌─────────────────────────────────────────────────────────┐
│  PHASE 1 — Embedding Pipeline          (run once, commit)│
│                                                         │
│  GitHub API                                             │
│      ↓ issues (title, body, labels, state)              │
│  OpenAI text-embedding-3-small                          │
│      ↓ []float32 vectors (1536 dimensions)              │
│  issues.db  ←──── committed to repo ────────────────→  │
│      ├── issues table  (metadata)                       │
│      └── vec_issues    (vec0 virtual table, KNN search) │
└─────────────────────────────────────────────────────────┘

                    ↓  clone repo  ↓

┌─────────────────────────────────────────────────────────┐
│  PHASE 2 — Triage Agent                  (runs always)  │
│                                                         │
│  issues.db → load into memory at startup                │
│                                                         │
│  Browser (React chat UI)                                │
│      ↕ WebSocket                                        │
│  Go HTTP Server                                         │
│      ↕                                                  │
│  Agent Loop (ReAct, GPT-4o)                             │
│      ↕ tool calls                                       │
│  ┌─────────────────────────────────────┐                │
│  │ Tools                               │                │
│  │  search_issues  — keyword + date    │                │
│  │  github_search  — live GitHub API   │                │
│  │  get_issue      — fetch one issue   │                │
│  │  import_issues  — session top-up    │                │
│  │  check_duplicate— vector KNN + LLM  │                │
│  └─────────────────────────────────────┘                │
│      ↕                                                  │
│  GitHub API (read only)                                 │
│  In-memory VectorStore (loaded from issues.db)          │
└─────────────────────────────────────────────────────────┘
```

---

## What Each Phase Owns

| Concern | Phase 1 | Phase 2 |
|---|---|---|
| GitHub API access | Read all issues | Read single issues |
| OpenAI API | Embeddings only | Chat completions + embeddings for queries |
| SQLite | Write issues + vectors | Read only |
| VectorStore | — | In-memory, loaded from DB |
| User interaction | None (CLI progress logs) | Web chat UI |
| Runs | Once, offline | Continuously, online |

---

## Environment Variables

| Variable | Phase 1 | Phase 2 | Notes |
|---|---|---|---|
| `GITHUB_TOKEN` | required | required | PAT with issues:read |
| `OPENAI_API_KEY` | required | required | Embeddings (Phase 1 + Phase 2). Chat fallback when `LLM_API_KEY` unset. |
| `GITHUB_REPO` | required | required | `owner/repo` |
| `OPENAI_BASE_URL` | optional | optional | Override embeddings endpoint |
| `ISSUES_DB` | optional | optional | Corpus path, default `issues.db` |
| `MAX_ISSUES` | optional | — | Cap ingestion count (0 = no cap) |
| `LLM_API_KEY` | — | optional | Chat key; falls back to `OPENAI_API_KEY` |
| `LLM_BASE_URL` | — | optional | Chat endpoint override (e.g. OpenRouter) |
| `LLM_MODEL` | — | optional | Defaults to `gpt-4o` |
| `PORT` | — | optional | Unset → OS picks a free port + logs URL |

---

## Duplicate Detection — Two-Stage Design

The core technical innovation lives in `check_duplicate`, which spans both phases:

```
Phase 1 built:   corpus of N issue embeddings in issues.db

Phase 2 query:
  1. Embed the target issue           → query vector
  2. KNN search vec_issues            → top 5 candidates by cosine distance  (fast)
  3. LLM reasoning call (GPT-4o)      → is_duplicate, confidence, reasoning  (accurate)
  4. Apply confidence threshold:
       ≥ 0.85  → recommend auto-close
       0.60–0.84 → flag for human review
       < 0.60  → not a duplicate
```

Stage 1 (vector search) is fast and cheap. Stage 2 (LLM reasoning) is accurate
but expensive — only runs on the top 5 candidates, not the full corpus.

The two-stage pattern (cheap IR prefilter → LLM verification) is the same
shape as the CUPID paper (Zhang et al., 2023, arXiv:2308.10022). We substitute
dense embeddings for CUPID's traditional IR prefilter; the LLM reasoning stage
is analogous.

Stage-2 call uses `json_object` response format and a single retry on parse
failure. Server-side `tierFor(confidence, is_duplicate)` is authoritative — the
LLM never decides the tier directly.

---

## Repository Structure

```
wails-triage-bot/
├── cmd/
│   ├── import-issues/
│   │   └── main.go          ← PHASE 1: standalone embedding pipeline
│   └── wails-triage/
│       ├── main.go          ← PHASE 2: HTTP server entry point
│       ├── api.go           ← PHASE 2: REST handlers (/api/triage, /api/issue)
│       └── ws.go            ← PHASE 2: WebSocket handler + agent driver
├── internal/
│   ├── agent/               ← PHASE 2: ReAct loop + embedded system prompt
│   ├── tools/               ← PHASE 2: tool schemas + dispatch (5 tools)
│   ├── store/               ← shared: SQLite + in-memory VectorStore (KNN)
│   ├── embed/               ← shared: chunk + pool + L2-normalize
│   ├── github/              ← shared: go-github client + rate-limit backoff
│   ├── logbus/              ← PHASE 2: log fanout → WS subscribers
│   └── wsproto/             ← shared: WS frame envelope
├── frontend/                ← PHASE 2: React + Vite + shadcn/ui + ai-elements
│   ├── src/                 ←         App.tsx, IssuePanel, TriageDialog
│   └── dist/                ←         built assets served at /
├── issues.db                ← PHASE 1 output, committed to repo
└── .env.example
```

---

## Out of Scope

- Writing to GitHub (closing, labelling) — recommendations only
- Authentication / multi-user
- Persistent conversation history
- Real-time webhook ingestion of new issues
- Production error handling and retry logic

These are known simplifications. The system is a working prototype demonstrating
the core agentic pattern, not a production deployment.

