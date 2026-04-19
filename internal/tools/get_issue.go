package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	ghissues "github.com/leaanthony/wails-triage-bot/internal/github"
)

type getArgs struct {
	Number int `json:"number"`
}

type issueFull struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	State     string   `json:"state"`
	Labels    []string `json:"labels"`
	URL       string   `json:"url"`
	Author    string   `json:"author,omitempty"`
	CreatedAt string   `json:"created_at,omitempty"`
	UpdatedAt string   `json:"updated_at,omitempty"`
	Source    string   `json:"source"` // "store" or "github"
}

func (t *Dispatcher) getIssue(ctx context.Context, raw json.RawMessage, emit Emitter) (json.RawMessage, error) {
	var a getArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, fmt.Errorf("get_issue args: %w", err)
	}
	if a.Number <= 0 {
		return nil, errors.New("get_issue: number required")
	}
	if it, ok := t.deps.Store.GetByNumber(a.Number); ok {
		emitActionsForIssues(emit, []issueBrief{toBrief(it)})
		return json.Marshal(toFull(it, "store"))
	}
	it, err := t.deps.GH.GetIssue(ctx, a.Number)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(it.Title + "\n\n" + it.Body)
	if text != "" {
		vec, _, err := t.deps.Embedder.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("embed fetched issue: %w", err)
		}
		if err := t.deps.Store.Add(it, vec); err != nil {
			return nil, err
		}
	}
	emitActionsForIssues(emit, []issueBrief{toBrief(it)})
	return json.Marshal(toFull(it, "github"))
}

func toFull(it ghissues.Issue, source string) issueFull {
	f := issueFull{
		Number: it.Number, Title: it.Title, Body: it.Body, State: it.State,
		Labels: it.Labels, URL: it.URL, Author: it.Author, Source: source,
	}
	if !it.CreatedAt.IsZero() {
		f.CreatedAt = it.CreatedAt.UTC().Format(time.RFC3339)
	}
	if !it.UpdatedAt.IsZero() {
		f.UpdatedAt = it.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return f
}
