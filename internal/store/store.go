package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	sqlitevec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"

	ghissues "github.com/leaanthony/wails-triage-bot/internal/github"
)

const VectorDim = 1536

type DB struct {
	sql *sql.DB
}

func OpenDB(path string) (*DB, error) {
	sqlitevec.Auto()
	s, err := sql.Open("sqlite3", "file:"+path+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := ensureSchema(s); err != nil {
		_ = s.Close()
		return nil, err
	}
	return &DB{sql: s}, nil
}

func (d *DB) Close() error { return d.sql.Close() }

func ensureSchema(s *sql.DB) error {
	_, err := s.Exec(`
CREATE TABLE IF NOT EXISTS issues (
    number  INTEGER PRIMARY KEY,
    title   TEXT NOT NULL,
    body    TEXT NOT NULL,
    labels  TEXT NOT NULL,
    state   TEXT NOT NULL,
    url     TEXT NOT NULL
);`)
	if err != nil {
		return fmt.Errorf("create issues table: %w", err)
	}
	// Best-effort migration: add date columns. Safe to re-run.
	for _, col := range []string{"created_at", "updated_at", "closed_at", "author"} {
		if _, err := s.Exec(`ALTER TABLE issues ADD COLUMN ` + col + ` TEXT`); err != nil &&
			!strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("migrate %s: %w", col, err)
		}
	}
	_, err = s.Exec(fmt.Sprintf(
		`CREATE VIRTUAL TABLE IF NOT EXISTS vec_issues USING vec0(
    number INTEGER PRIMARY KEY,
    embedding FLOAT[%d]
);`, VectorDim))
	if err != nil {
		return fmt.Errorf("create vec_issues table: %w", err)
	}
	return nil
}

// UpsertIssue writes metadata and vector atomically for a single issue.
func (d *DB) UpsertIssue(issue ghissues.Issue, vector []float32) error {
	if len(vector) != VectorDim {
		return fmt.Errorf("vector has %d dims, want %d", len(vector), VectorDim)
	}
	labels, err := json.Marshal(issue.Labels)
	if err != nil {
		return fmt.Errorf("marshal labels: %w", err)
	}
	blob, err := sqlitevec.SerializeFloat32(vector)
	if err != nil {
		return fmt.Errorf("serialize vector: %w", err)
	}
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`
INSERT INTO issues (number, title, body, labels, state, url, author, created_at, updated_at, closed_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(number) DO UPDATE SET
    title=excluded.title,
    body=excluded.body,
    labels=excluded.labels,
    state=excluded.state,
    url=excluded.url,
    author=excluded.author,
    created_at=excluded.created_at,
    updated_at=excluded.updated_at,
    closed_at=excluded.closed_at;`,
		issue.Number, issue.Title, issue.Body, string(labels), issue.State, issue.URL,
		nullableString(issue.Author),
		nullableTime(issue.CreatedAt), nullableTime(issue.UpdatedAt), nullableTime(issue.ClosedAt)); err != nil {
		return fmt.Errorf("upsert issue %d: %w", issue.Number, err)
	}
	if _, err := tx.Exec(`DELETE FROM vec_issues WHERE number = ?`, issue.Number); err != nil {
		return fmt.Errorf("clear vec row %d: %w", issue.Number, err)
	}
	if _, err := tx.Exec(`INSERT INTO vec_issues (number, embedding) VALUES (?, ?)`,
		issue.Number, blob); err != nil {
		return fmt.Errorf("insert vec row %d: %w", issue.Number, err)
	}
	return tx.Commit()
}

// GetIssue reads metadata for a single issue.
func (d *DB) GetIssue(number int) (ghissues.Issue, error) {
	row := d.sql.QueryRow(`SELECT number, title, body, labels, state, url, author, created_at, updated_at, closed_at FROM issues WHERE number = ?`, number)
	var it ghissues.Issue
	var labelsJSON string
	var author, created, updated, closed sql.NullString
	if err := row.Scan(&it.Number, &it.Title, &it.Body, &labelsJSON, &it.State, &it.URL, &author, &created, &updated, &closed); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ghissues.Issue{}, fmt.Errorf("issue %d not found", number)
		}
		return ghissues.Issue{}, err
	}
	if err := json.Unmarshal([]byte(labelsJSON), &it.Labels); err != nil {
		return ghissues.Issue{}, fmt.Errorf("unmarshal labels: %w", err)
	}
	if author.Valid {
		it.Author = author.String
	}
	it.CreatedAt = parseNullTime(created)
	it.UpdatedAt = parseNullTime(updated)
	it.ClosedAt = parseNullTime(closed)
	return it, nil
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}

func parseNullTime(ns sql.NullString) time.Time {
	if !ns.Valid || ns.String == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, ns.String)
	if err != nil {
		return time.Time{}
	}
	return t
}

// GetVector reads the embedding for a single issue.
func (d *DB) GetVector(number int) ([]float32, error) {
	row := d.sql.QueryRow(`SELECT embedding FROM vec_issues WHERE number = ?`, number)
	var blob []byte
	if err := row.Scan(&blob); err != nil {
		return nil, err
	}
	if len(blob) != VectorDim*4 {
		return nil, fmt.Errorf("vector blob has %d bytes, want %d", len(blob), VectorDim*4)
	}
	out := make([]float32, VectorDim)
	for i := 0; i < VectorDim; i++ {
		b := blob[i*4 : i*4+4]
		bits := uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
		out[i] = math.Float32frombits(bits)
	}
	return out, nil
}
