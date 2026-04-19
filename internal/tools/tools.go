// Package tools implements the agent's tool catalogue: schemas, handlers, and
// dispatch. Each tool's handler and types live in its own file
// (search_issues.go, github_search.go, get_issue.go, import_issues.go,
// check_duplicate.go). This file holds the shared plumbing: the Emitter
// surface, the Deps bundle, the Dispatcher + Schemas, and a handful of helpers
// used across tools.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
	jsonschema "github.com/sashabaranov/go-openai/jsonschema"

	"github.com/leaanthony/wails-triage-bot/internal/embed"
	ghissues "github.com/leaanthony/wails-triage-bot/internal/github"
	"github.com/leaanthony/wails-triage-bot/internal/store"
	"github.com/leaanthony/wails-triage-bot/internal/wsproto"
)

// Emitter is the minimal surface the dispatcher needs to push internal events
// (nested LLM calls, embeddings) into the chain-of-thought stream. Any type
// with Emit(wsproto.Frame) satisfies it — the agent's own emitter does.
type Emitter interface {
	Emit(wsproto.Frame)
}

type noopEmitter struct{}

func (noopEmitter) Emit(wsproto.Frame) {}

// Deps bundles the dependencies every tool handler needs.
type Deps struct {
	Store     *store.VectorStore
	GH        *ghissues.Client
	Embedder  *embed.Embedder
	Chat      *openai.Client // used for the Stage-2 duplicate reasoning call.
	ChatModel string
}

// Schemas returns the OpenAI tool definitions for every tool.
func Schemas() []openai.Tool {
	return []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "search_issues",
				Description: "Keyword search over the in-memory issue store with optional date filters. Returns matches ranked by keyword hit count. Prefer omitting `limit` so the default (20) applies. For richer GitHub search operators (e.g. `is:open`, `label:bug`, author filters, full-text) use github_search instead.",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"query":          {Type: jsonschema.String, Description: "Keywords to search for in issue title and body. Pass empty string to filter purely by date/state."},
						"state":          {Type: jsonschema.String, Description: "Filter by state: \"open\" or \"closed\". Omit for all."},
						"created_after":  {Type: jsonschema.String, Description: "ISO-8601 date/time; include only issues created on or after this instant."},
						"created_before": {Type: jsonschema.String, Description: "ISO-8601 date/time; include only issues created on or before this instant."},
						"limit":          {Type: jsonschema.Integer, Description: "Max results to return. Default 20, max 50."},
					},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "github_search",
				Description: "Fallback search that hits the GitHub Issues Search API directly. Use when the local store returns nothing or the user asks for something our corpus can't answer (e.g. very recent issues, native search operators like `label:bug is:open`, specific authors). Repo is scoped automatically.",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"query": {Type: jsonschema.String, Description: "GitHub search query. Repo is injected automatically. Supports operators like `label:bug`, `is:open`, `author:foo`, `created:>2025-01-01`."},
						"limit": {Type: jsonschema.Integer, Description: "Max results to return. Default 10, max 30."},
					},
					Required: []string{"query"},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "get_issue",
				Description: "Look up a single issue by number. Returns from the in-memory store; falls through to GitHub + embeddings on miss.",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"number": {Type: jsonschema.Integer, Description: "GitHub issue number."},
					},
					Required: []string{"number"},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "import_issues",
				Description: "Fetch every issue from the configured GitHub repo, embed, and add to the live in-memory store. Ephemeral; not persisted to disk.",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"reason": {Type: jsonschema.String, Description: "Optional note about why this import is being triggered."},
					},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "check_duplicate",
				Description: "Two-stage duplicate check: KNN top-5 from the in-memory corpus, then LLM reasoning. Returns tier (recommend_auto_close/human_review/not_duplicate), confidence, and reasoning.",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"number": {Type: jsonschema.Integer, Description: "Existing issue number to check. Optional if text is provided."},
						"text":   {Type: jsonschema.String, Description: "Free-form issue text to check. Optional if number is provided."},
					},
				},
			},
		},
	}
}

// Dispatcher routes tool calls to their handlers.
type Dispatcher struct {
	deps Deps
}

func NewDispatcher(d Deps) *Dispatcher { return &Dispatcher{deps: d} }

// Dispatch executes the named tool with the given JSON args and returns the
// JSON response payload the agent should feed back to the LLM. `emit` receives
// internal events (nested LLM calls etc.) so they show up in the UI's chain
// of thought; pass nil to suppress them.
func (t *Dispatcher) Dispatch(ctx context.Context, name string, args json.RawMessage, emit Emitter) (json.RawMessage, error) {
	if emit == nil {
		emit = noopEmitter{}
	}
	switch name {
	case "search_issues":
		return t.searchIssues(ctx, args, emit)
	case "github_search":
		return t.githubSearch(ctx, args, emit)
	case "get_issue":
		return t.getIssue(ctx, args, emit)
	case "import_issues":
		return t.importIssues(ctx, args)
	case "check_duplicate":
		return t.checkDuplicate(ctx, args, emit)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// ---------- shared types + helpers ----------

type issueBrief struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	State     string   `json:"state"`
	Labels    []string `json:"labels"`
	URL       string   `json:"url"`
	Author    string   `json:"author,omitempty"`
	CreatedAt string   `json:"created_at,omitempty"`
}

type searchResult struct {
	Count   int          `json:"count"`
	Matches []issueBrief `json:"matches"`
}

func toBrief(it ghissues.Issue) issueBrief {
	b := issueBrief{
		Number: it.Number, Title: it.Title, State: it.State,
		Labels: it.Labels, URL: it.URL, Author: it.Author,
	}
	if !it.CreatedAt.IsZero() {
		b.CreatedAt = it.CreatedAt.UTC().Format("2006-01-02")
	}
	return b
}

// emitActionsForIssues turns a search result into a rolling set of follow-up
// suggestions so the UI always has something actionable to offer.
func emitActionsForIssues(emit Emitter, matches []issueBrief) {
	if len(matches) == 0 {
		return
	}
	actions := make([]wsproto.QuickAction, 0, len(matches)*2+1)
	for i, m := range matches {
		if i >= 3 {
			break
		}
		actions = append(actions, wsproto.QuickAction{
			Label:  fmt.Sprintf("Triage #%d", m.Number),
			Prompt: fmt.Sprintf("Triage issue #%d — check for duplicates.", m.Number),
		})
	}
	if len(matches) > 0 {
		first := matches[0]
		actions = append(actions, wsproto.QuickAction{
			Label:  fmt.Sprintf("Summarise #%d", first.Number),
			Prompt: fmt.Sprintf("Summarise issue #%d and recommend next steps.", first.Number),
		})
		actions = append(actions, wsproto.QuickAction{
			Label:  "More like these",
			Prompt: fmt.Sprintf("Find more issues similar to #%d using the local corpus.", first.Number),
		})
	}
	emit.Emit(wsproto.Frame{Type: wsproto.FrameActions, Actions: actions})
}
