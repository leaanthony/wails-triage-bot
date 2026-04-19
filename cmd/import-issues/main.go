// Command import-issues fetches every issue from a GitHub repository, embeds
// them with OpenAI text-embedding-3-small, and writes them to issues.db.
//
// Environment:
//
//	GITHUB_TOKEN     required  PAT with issues:read
//	OPENAI_API_KEY   required  OpenAI API key
//	GITHUB_REPO      required  owner/repo
//	OPENAI_BASE_URL  optional  override embeddings endpoint
//	ISSUES_DB        optional  output path (default "issues.db")
//	MAX_ISSUES       optional  cap on number of issues (0 = no cap)
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	openai "github.com/sashabaranov/go-openai"

	"github.com/leaanthony/wails-triage-bot/internal/embed"
	gh "github.com/leaanthony/wails-triage-bot/internal/github"
	"github.com/leaanthony/wails-triage-bot/internal/store"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("import-issues: %v", err)
	}
}

func run() error {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load .env: %w", err)
	}
	ghToken := os.Getenv("GITHUB_TOKEN")
	oaKey := os.Getenv("OPENAI_API_KEY")
	oaBase := os.Getenv("OPENAI_BASE_URL")
	repo := os.Getenv("GITHUB_REPO")
	dbPath := envDefault("ISSUES_DB", "issues.db")
	maxIssues := envInt("MAX_ISSUES", 0)

	var missing []string
	if ghToken == "" {
		missing = append(missing, "GITHUB_TOKEN")
	}
	if oaKey == "" {
		missing = append(missing, "OPENAI_API_KEY")
	}
	if repo == "" {
		missing = append(missing, "GITHUB_REPO")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}

	ghClient, err := gh.New(ghToken, repo)
	if err != nil {
		return err
	}
	oaCfg := openai.DefaultConfig(oaKey)
	if oaBase != "" {
		oaCfg.BaseURL = oaBase
	}
	oa := openai.NewClientWithConfig(oaCfg)
	emb, err := embed.New(oa)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	start := time.Now()
	log.Printf("fetching issues from %s", repo)
	issues, err := ghClient.ListIssues(ctx)
	if err != nil {
		return fmt.Errorf("list issues: %w", err)
	}
	log.Printf("fetched %d issues in %s", len(issues), time.Since(start).Round(time.Second))

	if maxIssues > 0 && len(issues) > maxIssues {
		log.Printf("capping to first %d issues (MAX_ISSUES)", maxIssues)
		issues = issues[:maxIssues]
	}

	tmpPath := dbPath + ".tmp"
	_ = os.Remove(tmpPath)
	db, err := store.OpenDB(tmpPath)
	if err != nil {
		return err
	}

	var totalTokens int
	for i, issue := range issues {
		text := strings.TrimSpace(issue.Title + "\n\n" + issue.Body)
		if text == "" {
			log.Printf("  [%d/%d] #%d empty, skipping", i+1, len(issues), issue.Number)
			continue
		}
		vec, tokens, err := emb.Embed(ctx, text)
		if err != nil {
			_ = db.Close()
			return fmt.Errorf("embed issue #%d: %w", issue.Number, err)
		}
		totalTokens += tokens
		if err := db.UpsertIssue(issue, vec); err != nil {
			_ = db.Close()
			return fmt.Errorf("upsert issue #%d: %w", issue.Number, err)
		}
		if (i+1)%25 == 0 || i == len(issues)-1 {
			log.Printf("  [%d/%d] embedded; tokens=%d elapsed=%s",
				i+1, len(issues), totalTokens, time.Since(start).Round(time.Second))
		}
	}

	if err := db.Close(); err != nil {
		return fmt.Errorf("close db: %w", err)
	}
	if err := os.Rename(tmpPath, dbPath); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmpPath, dbPath, err)
	}

	log.Printf("done: wrote %s (%d issues, %d tokens, %s)",
		dbPath, len(issues), totalTokens, time.Since(start).Round(time.Second))
	return nil
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Printf("warn: %s=%q not an int, using %d", key, v, def)
		return def
	}
	return n
}
