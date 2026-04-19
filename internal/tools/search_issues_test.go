package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	ghissues "github.com/leaanthony/wails-triage-bot/internal/github"
	"github.com/leaanthony/wails-triage-bot/internal/store"
)

func mkIssue(num int, title, body, state string, created time.Time, labels ...string) ghissues.Issue {
	return ghissues.Issue{
		Number: num, Title: title, Body: body, State: state,
		URL: "u", Labels: labels, CreatedAt: created,
	}
}

func mkVec(seed float32) []float32 {
	v := make([]float32, store.VectorDim)
	v[0] = seed
	// unit-norm via setting just one dim
	v[0] = 1
	return v
}

func seedStore(t *testing.T, items []ghissues.Issue) *store.VectorStore {
	t.Helper()
	vs := store.NewVectorStore()
	for i, it := range items {
		if err := vs.Add(it, mkVec(float32(i+1))); err != nil {
			t.Fatal(err)
		}
	}
	return vs
}

func TestSearchIssuesKeywordMatch(t *testing.T) {
	now := time.Now().UTC()
	vs := seedStore(t, []ghissues.Issue{
		mkIssue(1, "Crash on startup", "the app crashes", "open", now),
		mkIssue(2, "Typo in docs", "fix typo", "closed", now),
		mkIssue(3, "Startup splash screen", "startup banner", "open", now),
	})
	d := NewDispatcher(Deps{Store: vs})
	args, _ := json.Marshal(searchArgs{Query: "startup"})
	raw, err := d.Dispatch(context.Background(), "search_issues", args, nil)
	if err != nil {
		t.Fatal(err)
	}
	var res searchResult
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatal(err)
	}
	if res.Count != 2 {
		t.Errorf("count=%d want 2", res.Count)
	}
}

func TestSearchIssuesStateFilter(t *testing.T) {
	now := time.Now().UTC()
	vs := seedStore(t, []ghissues.Issue{
		mkIssue(1, "alpha", "", "open", now),
		mkIssue(2, "alpha", "", "closed", now),
	})
	d := NewDispatcher(Deps{Store: vs})
	args, _ := json.Marshal(searchArgs{Query: "alpha", State: "CLOSED"})
	raw, err := d.Dispatch(context.Background(), "search_issues", args, nil)
	if err != nil {
		t.Fatal(err)
	}
	var res searchResult
	_ = json.Unmarshal(raw, &res)
	if res.Count != 1 || res.Matches[0].Number != 2 {
		t.Errorf("got %+v", res)
	}
}

func TestSearchIssuesDateFilter(t *testing.T) {
	old := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	recent := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	vs := seedStore(t, []ghissues.Issue{
		mkIssue(1, "a", "", "open", old),
		mkIssue(2, "a", "", "open", recent),
	})
	d := NewDispatcher(Deps{Store: vs})
	args, _ := json.Marshal(searchArgs{CreatedAfter: "2025-06-01"})
	raw, err := d.Dispatch(context.Background(), "search_issues", args, nil)
	if err != nil {
		t.Fatal(err)
	}
	var res searchResult
	_ = json.Unmarshal(raw, &res)
	if res.Count != 1 || res.Matches[0].Number != 2 {
		t.Errorf("got %+v", res)
	}
}

func TestSearchIssuesLimitClampAndDefault(t *testing.T) {
	now := time.Now().UTC()
	items := make([]ghissues.Issue, 60)
	for i := range items {
		items[i] = mkIssue(i+1, "widget", "", "open", now)
	}
	vs := seedStore(t, items)
	d := NewDispatcher(Deps{Store: vs})

	// Default = 20.
	args, _ := json.Marshal(searchArgs{Query: "widget"})
	raw, _ := d.Dispatch(context.Background(), "search_issues", args, nil)
	var r1 searchResult
	_ = json.Unmarshal(raw, &r1)
	if r1.Count != 20 {
		t.Errorf("default count=%d want 20", r1.Count)
	}

	// Clamp to 50.
	args, _ = json.Marshal(searchArgs{Query: "widget", Limit: 999})
	raw, _ = d.Dispatch(context.Background(), "search_issues", args, nil)
	var r2 searchResult
	_ = json.Unmarshal(raw, &r2)
	if r2.Count != 50 {
		t.Errorf("clamp count=%d want 50", r2.Count)
	}
}

func TestSearchIssuesNoMatch(t *testing.T) {
	vs := seedStore(t, []ghissues.Issue{mkIssue(1, "a", "b", "open", time.Now())})
	d := NewDispatcher(Deps{Store: vs})
	args, _ := json.Marshal(searchArgs{Query: "nonexistent"})
	raw, err := d.Dispatch(context.Background(), "search_issues", args, nil)
	if err != nil {
		t.Fatal(err)
	}
	var res searchResult
	_ = json.Unmarshal(raw, &res)
	if res.Count != 0 {
		t.Errorf("count=%d", res.Count)
	}
}

func TestSearchIssuesBadDate(t *testing.T) {
	vs := seedStore(t, []ghissues.Issue{mkIssue(1, "a", "", "open", time.Now())})
	d := NewDispatcher(Deps{Store: vs})
	args, _ := json.Marshal(searchArgs{CreatedAfter: "garbage"})
	if _, err := d.Dispatch(context.Background(), "search_issues", args, nil); err == nil {
		t.Error("expected error for bad date")
	}
}

func TestSearchIssuesDateOnlySortedNewest(t *testing.T) {
	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	vs := seedStore(t, []ghissues.Issue{
		mkIssue(1, "a", "", "open", t1),
		mkIssue(2, "a", "", "open", t3),
		mkIssue(3, "a", "", "open", t2),
	})
	d := NewDispatcher(Deps{Store: vs})
	args, _ := json.Marshal(searchArgs{CreatedAfter: "2023-01-01"})
	raw, err := d.Dispatch(context.Background(), "search_issues", args, nil)
	if err != nil {
		t.Fatal(err)
	}
	var res searchResult
	_ = json.Unmarshal(raw, &res)
	if len(res.Matches) != 3 || res.Matches[0].Number != 2 || res.Matches[2].Number != 1 {
		t.Errorf("sort wrong: %+v", res.Matches)
	}
}

func TestSearchIssuesEmitsActions(t *testing.T) {
	vs := seedStore(t, []ghissues.Issue{mkIssue(1, "widget", "", "open", time.Now())})
	d := NewDispatcher(Deps{Store: vs})
	e := &captureEmitter{}
	args, _ := json.Marshal(searchArgs{Query: "widget"})
	_, err := d.Dispatch(context.Background(), "search_issues", args, e)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.frames) == 0 {
		t.Error("expected action frame")
	}
}

func TestSearchIssuesBadJSON(t *testing.T) {
	d := NewDispatcher(Deps{Store: store.NewVectorStore()})
	_, err := d.Dispatch(context.Background(), "search_issues", json.RawMessage(`not json`), nil)
	if err == nil {
		t.Error("expected json error")
	}
}
