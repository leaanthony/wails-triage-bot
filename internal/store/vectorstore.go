package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"

	ghissues "github.com/leaanthony/wails-triage-bot/internal/github"
)

// VectorStore holds every issue in memory with its unit-normalized embedding.
// Safe for concurrent use.
type VectorStore struct {
	mu      sync.RWMutex
	issues  []ghissues.Issue
	vectors [][]float32
	byNum   map[int]int // issue number -> slot index
}

// Match is a KNN result paired with its cosine distance.
type Match struct {
	Issue    ghissues.Issue
	Distance float64
}

// NewVectorStore returns an empty in-memory store.
func NewVectorStore() *VectorStore {
	return &VectorStore{byNum: map[int]int{}}
}

// Load reads every row from issues + vec_issues and returns a populated store.
func Load(ctx context.Context, db *DB) (*VectorStore, error) {
	vs := NewVectorStore()
	rows, err := db.sql.QueryContext(ctx, `
SELECT i.number, i.title, i.body, i.labels, i.state, i.url, i.author, i.created_at, i.updated_at, i.closed_at, v.embedding
FROM issues i
JOIN vec_issues v ON v.number = i.number`)
	if err != nil {
		return nil, fmt.Errorf("query corpus: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			it                               ghissues.Issue
			labels                           string
			author, created, updated, closed sql.NullString
			blob                             []byte
		)
		if err := rows.Scan(&it.Number, &it.Title, &it.Body, &labels, &it.State, &it.URL, &author, &created, &updated, &closed, &blob); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		if err := json.Unmarshal([]byte(labels), &it.Labels); err != nil {
			return nil, fmt.Errorf("unmarshal labels for #%d: %w", it.Number, err)
		}
		if author.Valid {
			it.Author = author.String
		}
		it.CreatedAt = parseNullTime(created)
		it.UpdatedAt = parseNullTime(updated)
		it.ClosedAt = parseNullTime(closed)
		if len(blob) != VectorDim*4 {
			return nil, fmt.Errorf("issue #%d: vec blob has %d bytes, want %d", it.Number, len(blob), VectorDim*4)
		}
		vec := make([]float32, VectorDim)
		for i := 0; i < VectorDim; i++ {
			b := blob[i*4 : i*4+4]
			bits := uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
			vec[i] = math.Float32frombits(bits)
		}
		vs.addLocked(it, vec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return vs, nil
}

// Len returns the number of issues in the store.
func (v *VectorStore) Len() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.issues)
}

// Add inserts or replaces an issue + vector.
func (v *VectorStore) Add(issue ghissues.Issue, vec []float32) error {
	if len(vec) != VectorDim {
		return fmt.Errorf("vector has %d dims, want %d", len(vec), VectorDim)
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	v.addLocked(issue, vec)
	return nil
}

func (v *VectorStore) addLocked(issue ghissues.Issue, vec []float32) {
	if idx, ok := v.byNum[issue.Number]; ok {
		v.issues[idx] = issue
		v.vectors[idx] = vec
		return
	}
	v.byNum[issue.Number] = len(v.issues)
	v.issues = append(v.issues, issue)
	v.vectors = append(v.vectors, vec)
}

// GetByNumber returns the issue with the given number, if present.
func (v *VectorStore) GetByNumber(number int) (ghissues.Issue, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	idx, ok := v.byNum[number]
	if !ok {
		return ghissues.Issue{}, false
	}
	return v.issues[idx], true
}

// GetVectorByNumber returns a copy of the stored embedding for the given
// issue number. The copy prevents callers from mutating internal state.
func (v *VectorStore) GetVectorByNumber(number int) ([]float32, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	idx, ok := v.byNum[number]
	if !ok {
		return nil, false
	}
	out := make([]float32, len(v.vectors[idx]))
	copy(out, v.vectors[idx])
	return out, true
}

// All returns a snapshot of every issue. Callers must not mutate.
func (v *VectorStore) All() []ghissues.Issue {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := make([]ghissues.Issue, len(v.issues))
	copy(out, v.issues)
	return out
}

// KNN returns up to k nearest issues by cosine distance (1 - cosine similarity).
// Expects the query vector to already be unit-normalized.
func (v *VectorStore) KNN(query []float32, k int) ([]Match, error) {
	if len(query) != VectorDim {
		return nil, fmt.Errorf("query has %d dims, want %d", len(query), VectorDim)
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	type scored struct {
		idx  int
		dist float64
	}
	scores := make([]scored, len(v.vectors))
	for i, vec := range v.vectors {
		scores[i] = scored{idx: i, dist: cosineDistance(query, vec)}
	}
	sort.Slice(scores, func(a, b int) bool { return scores[a].dist < scores[b].dist })
	if k > len(scores) {
		k = len(scores)
	}
	out := make([]Match, k)
	for i := 0; i < k; i++ {
		s := scores[i]
		out[i] = Match{Issue: v.issues[s.idx], Distance: s.dist}
	}
	return out, nil
}

// cosineDistance = 1 - dot(a,b)/(|a||b|). Assumes inputs are non-zero;
// unit-normalized inputs reduce to 1 - dot(a,b).
func cosineDistance(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 1
	}
	return 1 - dot/(math.Sqrt(na)*math.Sqrt(nb))
}

