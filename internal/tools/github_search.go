package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type ghSearchArgs struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

func (t *Dispatcher) githubSearch(ctx context.Context, raw json.RawMessage, emit Emitter) (json.RawMessage, error) {
	var a ghSearchArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, fmt.Errorf("github_search args: %w", err)
	}
	a.Query = strings.TrimSpace(a.Query)
	if a.Query == "" {
		return nil, errors.New("github_search: empty query")
	}
	if a.Limit <= 0 {
		a.Limit = 10
	}
	if a.Limit > 30 {
		a.Limit = 30
	}
	issues, err := t.deps.GH.SearchIssues(ctx, a.Query, a.Limit)
	if err != nil {
		return nil, err
	}
	out := searchResult{Count: len(issues), Matches: make([]issueBrief, 0, len(issues))}
	for _, it := range issues {
		out.Matches = append(out.Matches, toBrief(it))
	}
	emitActionsForIssues(emit, out.Matches)
	return json.Marshal(out)
}
