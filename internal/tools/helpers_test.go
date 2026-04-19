package tools

import (
	"encoding/json"
	"testing"
	"time"

	ghissues "github.com/leaanthony/wails-triage-bot/internal/github"
	"github.com/leaanthony/wails-triage-bot/internal/store"
	"github.com/leaanthony/wails-triage-bot/internal/wsproto"
)

func TestTierFor(t *testing.T) {
	cases := []struct {
		name string
		conf float64
		dup  bool
		want string
	}{
		{"dup high", 0.9, true, "recommend_auto_close"},
		{"dup mid", 0.7, true, "human_review"},
		{"dup low", 0.3, true, "not_duplicate"},
		{"not-dup high-conf", 0.7, false, "human_review"},
		{"not-dup low-conf", 0.2, false, "not_duplicate"},
		{"boundary 0.85", 0.85, true, "recommend_auto_close"},
		{"boundary 0.60", 0.60, true, "human_review"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := tierFor(c.conf, c.dup); got != c.want {
				t.Errorf("tierFor(%v,%v)=%q want %q", c.conf, c.dup, got, c.want)
			}
		})
	}
}

func mkMatch(num int, title string) store.Match {
	return store.Match{Issue: ghissues.Issue{
		Number: num, Title: title, URL: "u", State: "open",
	}}
}

func TestBriefVerdicts(t *testing.T) {
	matches := []store.Match{mkMatch(1, "a"), mkMatch(2, "b")}
	got := briefVerdicts(matches)
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	for _, g := range got {
		if g.Verdict != "unknown" {
			t.Errorf("verdict=%q", g.Verdict)
		}
	}
}

func TestEnrichCandidatesEmptyLLM(t *testing.T) {
	matches := []store.Match{mkMatch(1, "a")}
	got := enrichCandidates(nil, matches)
	if len(got) != 1 || got[0].Verdict != "unknown" || got[0].Title != "a" {
		t.Errorf("%+v", got)
	}
}

func TestEnrichCandidatesMerges(t *testing.T) {
	matches := []store.Match{mkMatch(1, "title-one"), mkMatch(2, "title-two")}
	llm := []candidateVerdict{{Number: 1, Verdict: "duplicate"}}
	got := enrichCandidates(llm, matches)
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	// #1 from LLM enriched with store title.
	if got[0].Number != 1 || got[0].Verdict != "duplicate" || got[0].Title != "title-one" {
		t.Errorf("first=%+v", got[0])
	}
	// #2 appended as unknown.
	if got[1].Number != 2 || got[1].Verdict != "unknown" {
		t.Errorf("second=%+v", got[1])
	}
}

func TestEnrichCandidatesBlankVerdictDefaults(t *testing.T) {
	llm := []candidateVerdict{{Number: 9, Verdict: ""}}
	got := enrichCandidates(llm, nil)
	if got[0].Verdict != "unknown" {
		t.Errorf("verdict=%q", got[0].Verdict)
	}
}

func TestToBrief(t *testing.T) {
	ts := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)
	it := ghissues.Issue{
		Number: 7, Title: "t", State: "open", URL: "u",
		Labels: []string{"bug"}, Author: "lea", CreatedAt: ts,
	}
	b := toBrief(it)
	if b.Number != 7 || b.Title != "t" || b.State != "open" || b.URL != "u" ||
		b.Author != "lea" || b.CreatedAt != "2026-04-19" {
		t.Errorf("%+v", b)
	}
}

func TestToBriefZeroCreatedAt(t *testing.T) {
	b := toBrief(ghissues.Issue{Number: 1})
	if b.CreatedAt != "" {
		t.Errorf("CreatedAt=%q", b.CreatedAt)
	}
}

type captureEmitter struct{ frames []wsproto.Frame }

func (c *captureEmitter) Emit(f wsproto.Frame) { c.frames = append(c.frames, f) }

func TestEmitActionsForIssuesEmpty(t *testing.T) {
	e := &captureEmitter{}
	emitActionsForIssues(e, nil)
	if len(e.frames) != 0 {
		t.Errorf("expected no emit, got %d", len(e.frames))
	}
}

func TestEmitActionsForIssuesCaps(t *testing.T) {
	e := &captureEmitter{}
	matches := []issueBrief{
		{Number: 1}, {Number: 2}, {Number: 3}, {Number: 4}, {Number: 5},
	}
	emitActionsForIssues(e, matches)
	if len(e.frames) != 1 {
		t.Fatalf("frames=%d", len(e.frames))
	}
	f := e.frames[0]
	if f.Type != wsproto.FrameActions {
		t.Errorf("type=%v", f.Type)
	}
	// 3 triage + 1 summarise + 1 "more like" = 5
	if len(f.Actions) != 5 {
		t.Errorf("actions=%d", len(f.Actions))
	}
}

func TestNoopEmitter(t *testing.T) {
	var n noopEmitter
	n.Emit(wsproto.Frame{Type: wsproto.FrameToken})
}

func TestParseFlexDate(t *testing.T) {
	cases := []string{
		"2026-04-19",
		"2026/04/19",
		"2026-04-19T10:00:00",
		"2026-04-19T10:00:00Z",
	}
	for _, s := range cases {
		if _, err := parseFlexDate(s); err != nil {
			t.Errorf("parseFlexDate(%q): %v", s, err)
		}
	}
	if _, err := parseFlexDate("not-a-date"); err == nil {
		t.Error("expected error for bad date")
	}
}

func TestSchemasPresent(t *testing.T) {
	s := Schemas()
	names := map[string]bool{}
	for _, tool := range s {
		if tool.Function == nil {
			t.Errorf("nil function in %v", tool)
			continue
		}
		names[tool.Function.Name] = true
	}
	for _, want := range []string{"search_issues", "github_search", "get_issue", "import_issues", "check_duplicate"} {
		if !names[want] {
			t.Errorf("missing tool %q", want)
		}
	}
}

func TestDispatchUnknownTool(t *testing.T) {
	d := NewDispatcher(Deps{})
	_, err := d.Dispatch(nil, "does_not_exist", json.RawMessage(`{}`), nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
