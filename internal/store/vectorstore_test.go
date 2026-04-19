package store_test

import (
	"context"
	"math"
	"path/filepath"
	"sync"
	"testing"

	ghissues "github.com/leaanthony/wails-triage-bot/internal/github"
	"github.com/leaanthony/wails-triage-bot/internal/store"
)

func unitVec(seed float32) []float32 {
	v := make([]float32, store.VectorDim)
	for i := range v {
		v[i] = seed + float32(i)*1e-5
	}
	var norm float64
	for _, x := range v {
		norm += float64(x) * float64(x)
	}
	norm = math.Sqrt(norm)
	inv := float32(1.0 / norm)
	for i := range v {
		v[i] *= inv
	}
	return v
}

func TestKNNOrderAndAdd(t *testing.T) {
	vs := store.NewVectorStore()
	seeds := []float32{0.1, 0.2, 0.3, 0.4, 0.5}
	for i, s := range seeds {
		if err := vs.Add(ghissues.Issue{Number: i + 1, Title: "t"}, unitVec(s)); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if vs.Len() != 5 {
		t.Fatalf("len=%d", vs.Len())
	}
	matches, err := vs.KNN(unitVec(0.1), 3)
	if err != nil {
		t.Fatalf("knn: %v", err)
	}
	if len(matches) != 3 {
		t.Fatalf("want 3 matches, got %d", len(matches))
	}
	if matches[0].Issue.Number != 1 {
		t.Fatalf("nearest should be #1, got #%d", matches[0].Issue.Number)
	}
	for i := 1; i < len(matches); i++ {
		if matches[i].Distance < matches[i-1].Distance {
			t.Fatalf("distances not ascending: %v", matches)
		}
	}

	// K > N returns all.
	all, err := vs.KNN(unitVec(0.0), 99)
	if err != nil {
		t.Fatalf("knn all: %v", err)
	}
	if len(all) != 5 {
		t.Fatalf("k>N want 5, got %d", len(all))
	}

	// Upsert-by-number replaces metadata.
	if err := vs.Add(ghissues.Issue{Number: 1, Title: "updated"}, unitVec(0.1)); err != nil {
		t.Fatalf("re-add: %v", err)
	}
	if got, _ := vs.GetByNumber(1); got.Title != "updated" {
		t.Fatalf("title not updated: %q", got.Title)
	}
	if vs.Len() != 5 {
		t.Fatalf("len changed after upsert: %d", vs.Len())
	}
}

func TestLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.db")
	db, err := store.OpenDB(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	for i := 1; i <= 3; i++ {
		if err := db.UpsertIssue(ghissues.Issue{Number: i, Title: "t", Body: "b", Labels: []string{"a"}, State: "open", URL: "u"},
			unitVec(float32(i)*0.1)); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}
	vs, err := store.Load(context.Background(), db)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if vs.Len() != 3 {
		t.Fatalf("loaded %d, want 3", vs.Len())
	}
	matches, _ := vs.KNN(unitVec(0.1), 3)
	if matches[0].Issue.Number != 1 {
		t.Fatalf("nearest should be #1, got #%d", matches[0].Issue.Number)
	}
}

func TestConcurrentAccess(t *testing.T) {
	vs := store.NewVectorStore()
	for i := 1; i <= 20; i++ {
		_ = vs.Add(ghissues.Issue{Number: i}, unitVec(float32(i)*0.01))
	}
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_, _ = vs.KNN(unitVec(0.05), 3)
				}
			}
		}()
	}
	for i := 21; i <= 100; i++ {
		_ = vs.Add(ghissues.Issue{Number: i}, unitVec(float32(i)*0.01))
	}
	close(stop)
	wg.Wait()
	if vs.Len() != 100 {
		t.Fatalf("len=%d", vs.Len())
	}
}
