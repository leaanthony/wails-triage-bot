package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	ghissues "github.com/leaanthony/wails-triage-bot/internal/github"
)

type searchArgs struct {
	Query         string `json:"query"`
	State         string `json:"state"`
	CreatedAfter  string `json:"created_after"`
	CreatedBefore string `json:"created_before"`
	Limit         int    `json:"limit"`
}

func (t *Dispatcher) searchIssues(_ context.Context, raw json.RawMessage, emit Emitter) (json.RawMessage, error) {
	var a searchArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, fmt.Errorf("search_issues args: %w", err)
	}
	a.Query = strings.TrimSpace(a.Query)
	state := strings.ToLower(strings.TrimSpace(a.State))
	if a.Limit <= 0 {
		a.Limit = 20
	}
	if a.Limit > 50 {
		a.Limit = 50
	}
	var (
		createdAfter, createdBefore time.Time
		err                         error
	)
	if a.CreatedAfter != "" {
		createdAfter, err = parseFlexDate(a.CreatedAfter)
		if err != nil {
			return nil, fmt.Errorf("search_issues.created_after: %w", err)
		}
	}
	if a.CreatedBefore != "" {
		createdBefore, err = parseFlexDate(a.CreatedBefore)
		if err != nil {
			return nil, fmt.Errorf("search_issues.created_before: %w", err)
		}
	}
	terms := strings.Fields(strings.ToLower(a.Query))
	onlyDateFilter := len(terms) == 0
	type scored struct {
		it    ghissues.Issue
		score int
	}
	var hits []scored
	for _, it := range t.deps.Store.All() {
		if state != "" && strings.ToLower(it.State) != state {
			continue
		}
		if !createdAfter.IsZero() && (it.CreatedAt.IsZero() || it.CreatedAt.Before(createdAfter)) {
			continue
		}
		if !createdBefore.IsZero() && (it.CreatedAt.IsZero() || it.CreatedAt.After(createdBefore)) {
			continue
		}
		score := 1 // baseline for date-only queries.
		if !onlyDateFilter {
			hay := strings.ToLower(it.Title + "\n" + it.Body)
			score = 0
			for _, term := range terms {
				score += strings.Count(hay, term)
			}
			if score == 0 {
				continue
			}
		}
		hits = append(hits, scored{it, score})
	}
	if onlyDateFilter {
		// Sort newest first.
		sort.Slice(hits, func(i, j int) bool { return hits[i].it.CreatedAt.After(hits[j].it.CreatedAt) })
	} else {
		sort.Slice(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	}
	if len(hits) > a.Limit {
		hits = hits[:a.Limit]
	}
	out := searchResult{Count: len(hits), Matches: make([]issueBrief, 0, len(hits))}
	for _, h := range hits {
		out.Matches = append(out.Matches, toBrief(h.it))
	}
	emitActionsForIssues(emit, out.Matches)
	return json.Marshal(out)
}

func parseFlexDate(s string) (time.Time, error) {
	formats := []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02", "2006/01/02"}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date %q", s)
}
