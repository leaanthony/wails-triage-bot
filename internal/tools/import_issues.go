package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type importResult struct {
	Fetched int `json:"fetched"`
	Added   int `json:"added"`
	Tokens  int `json:"tokens"`
}

func (t *Dispatcher) importIssues(ctx context.Context, _ json.RawMessage) (json.RawMessage, error) {
	issues, err := t.deps.GH.ListIssues(ctx)
	if err != nil {
		return nil, err
	}
	var added, tokens int
	for _, it := range issues {
		if _, ok := t.deps.Store.GetByNumber(it.Number); ok {
			continue
		}
		text := strings.TrimSpace(it.Title + "\n\n" + it.Body)
		if text == "" {
			continue
		}
		vec, used, err := t.deps.Embedder.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("embed #%d: %w", it.Number, err)
		}
		tokens += used
		if err := t.deps.Store.Add(it, vec); err != nil {
			return nil, err
		}
		added++
	}
	return json.Marshal(importResult{Fetched: len(issues), Added: added, Tokens: tokens})
}
