package store_test

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	ghissues "github.com/leaanthony/wails-triage-bot/internal/github"
	"github.com/leaanthony/wails-triage-bot/internal/store"
)

func TestUpsertAndRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.db")
	db, err := store.OpenDB(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	vec := make([]float32, store.VectorDim)
	for i := range vec {
		vec[i] = float32(i) * 0.001
	}
	issue := ghissues.Issue{
		Number: 42,
		Title:  "hello",
		Body:   "world",
		Labels: []string{"bug", "needs-triage"},
		State:  "open",
		URL:    "https://example/42",
	}
	if err := db.UpsertIssue(issue, vec); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := db.GetIssue(42)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != issue.Title || got.Body != issue.Body || got.State != issue.State || got.URL != issue.URL {
		t.Fatalf("metadata mismatch: %+v", got)
	}
	if len(got.Labels) != 2 || got.Labels[0] != "bug" {
		t.Fatalf("labels mismatch: %+v", got.Labels)
	}

	gotVec, err := db.GetVector(42)
	if err != nil {
		t.Fatalf("get vec: %v", err)
	}
	if len(gotVec) != len(vec) {
		t.Fatalf("vec len %d != %d", len(gotVec), len(vec))
	}
	for i := range vec {
		if math.Abs(float64(gotVec[i]-vec[i])) > 1e-6 {
			t.Fatalf("vec[%d]: got %v want %v", i, gotVec[i], vec[i])
		}
	}

	// Upsert replaces.
	issue.Title = "updated"
	if err := db.UpsertIssue(issue, vec); err != nil {
		t.Fatalf("upsert2: %v", err)
	}
	got, err = db.GetIssue(42)
	if err != nil {
		t.Fatalf("get2: %v", err)
	}
	if got.Title != "updated" {
		t.Fatalf("title not updated: %q", got.Title)
	}

	// File exists, nonzero.
	st, err := os.Stat(path)
	if err != nil || st.Size() == 0 {
		t.Fatalf("db file empty: %v size=%d", err, st.Size())
	}
}
