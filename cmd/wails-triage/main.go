// Command wails-triage runs the Phase 2 triage agent: HTTP server, WebSocket
// chat endpoint, and per-connection ReAct loop over the in-memory issue corpus.
//
// Environment:
//
//	OPENAI_API_KEY   required   Embeddings key (always hits OpenAI-compatible).
//	OPENAI_BASE_URL  optional   Embeddings endpoint override.
//	GITHUB_TOKEN     required   Needed by get_issue and import_issues tools.
//	GITHUB_REPO      required   owner/repo.
//	LLM_MODEL        optional   Chat model (default gpt-4o).
//	LLM_BASE_URL     optional   Chat endpoint override.
//	LLM_API_KEY      optional   Chat API key (falls back to OPENAI_API_KEY).
//	ISSUES_DB        optional   Corpus path (default issues.db).
//	PORT             optional   Listen port (default 8080).
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	openai "github.com/sashabaranov/go-openai"

	"github.com/leaanthony/wails-triage-bot/internal/agent"
	"github.com/leaanthony/wails-triage-bot/internal/embed"
	gh "github.com/leaanthony/wails-triage-bot/internal/github"
	"github.com/leaanthony/wails-triage-bot/internal/logbus"
	"github.com/leaanthony/wails-triage-bot/internal/store"
	"github.com/leaanthony/wails-triage-bot/internal/tools"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("wails-triage: %v", err)
	}
}

func run() error {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load .env: %w", err)
	}

	logs := logbus.New()
	log.SetOutput(logs.Tee(os.Stderr))
	oaKey := os.Getenv("OPENAI_API_KEY")
	ghToken := os.Getenv("GITHUB_TOKEN")
	repo := os.Getenv("GITHUB_REPO")
	dbPath := envDefault("ISSUES_DB", "issues.db")
	portEnv := os.Getenv("PORT") // empty = pick a free port.
	llmModel := envDefault("LLM_MODEL", agent.DefaultModel)
	llmBase := os.Getenv("LLM_BASE_URL")
	llmKey := envDefault("LLM_API_KEY", oaKey)

	var missing []string
	if oaKey == "" {
		missing = append(missing, "OPENAI_API_KEY")
	}
	if ghToken == "" {
		missing = append(missing, "GITHUB_TOKEN")
	}
	if repo == "" {
		missing = append(missing, "GITHUB_REPO")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}
	if _, err := os.Stat(dbPath); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%s not found — run `go run ./cmd/import-issues` first", dbPath)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Printf("loading corpus from %s", dbPath)
	start := time.Now()
	db, err := store.OpenDB(dbPath)
	if err != nil {
		return err
	}
	vs, err := store.Load(ctx, db)
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("load corpus: %w", err)
	}
	_ = db.Close()
	log.Printf("loaded %d issues in %s", vs.Len(), time.Since(start).Round(time.Millisecond))

	// Embeddings always go to OpenAI (or OPENAI_BASE_URL override).
	embCfg := openai.DefaultConfig(oaKey)
	if v := os.Getenv("OPENAI_BASE_URL"); v != "" {
		embCfg.BaseURL = v
	}
	embClient := openai.NewClientWithConfig(embCfg)
	embedder, err := embed.New(embClient)
	if err != nil {
		return err
	}

	// Chat completions use the LLM_* config, falls back to OpenAI when unset.
	chatCfg := openai.DefaultConfig(llmKey)
	if llmBase != "" {
		chatCfg.BaseURL = llmBase
	}
	chatClient := openai.NewClientWithConfig(chatCfg)

	ghClient, err := gh.New(ghToken, repo)
	if err != nil {
		return err
	}

	deps := tools.Deps{
		Store:     vs,
		GH:        ghClient,
		Embedder:  embedder,
		Chat:      chatClient,
		ChatModel: llmModel,
	}
	dispatcher := tools.NewDispatcher(deps)
	ag := agent.New(chatClient, llmModel, dispatcher)

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("frontend/dist")))
	mux.HandleFunc("/ws", wsHandler(ag, logs))
	mux.HandleFunc("/api/meta", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"repo":%q,"issue_count":%d}`, repo, vs.Len())
	})
	mux.HandleFunc("/api/triage", triageHandler(dispatcher))
	mux.HandleFunc("/api/issue", issueHandler(dispatcher))

	listenAddr := ":0"
	if portEnv != "" {
		listenAddr = ":" + portEnv
	}
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", listenAddr, err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://localhost:%d", port)

	srv := &http.Server{Handler: mux}
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		log.Printf("shutdown signal received")
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutCancel()
		_ = srv.Shutdown(shutCtx)
		cancel()
	}()

	log.Printf("listening on %s", url)
	go openBrowser(url)
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func openBrowser(url string) {
	// Give the server a moment to start accepting connections.
	time.Sleep(200 * time.Millisecond)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("open browser: %v (open %s manually)", err, url)
	}
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
