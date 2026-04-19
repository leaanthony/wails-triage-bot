package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"github.com/leaanthony/wails-triage-bot/internal/store"
	"github.com/leaanthony/wails-triage-bot/internal/wsproto"
)

type dupArgs struct {
	Number int    `json:"number"`
	Text   string `json:"text"`
}

type candidateVerdict struct {
	Number  int    `json:"number"`
	Title   string `json:"title,omitempty"`
	URL     string `json:"url,omitempty"`
	State   string `json:"state,omitempty"`
	Verdict string `json:"verdict"`
}

type dupResult struct {
	Tier        string             `json:"tier"` // recommend_auto_close | human_review | not_duplicate
	IsDuplicate bool               `json:"is_duplicate"`
	Confidence  float64            `json:"confidence"`
	Reasoning   string             `json:"reasoning"`
	Candidates  []candidateVerdict `json:"candidates"`
	Target      issueBrief         `json:"target,omitempty"`
}

func (t *Dispatcher) checkDuplicate(ctx context.Context, raw json.RawMessage, emit Emitter) (json.RawMessage, error) {
	var a dupArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, fmt.Errorf("check_duplicate args: %w", err)
	}
	if a.Number <= 0 && strings.TrimSpace(a.Text) == "" {
		return nil, errors.New("check_duplicate: need number or text")
	}

	// Stage 1: resolve target text + embed (or reuse stored vector).
	var (
		targetText    string
		targetBrief   issueBrief
		skipCandidate int
		vec           []float32
	)
	if a.Number > 0 {
		it, ok := t.deps.Store.GetByNumber(a.Number)
		if ok {
			if v, hit := t.deps.Store.GetVectorByNumber(a.Number); hit {
				vec = v
			}
		} else {
			fetched, err := t.deps.GH.GetIssue(ctx, a.Number)
			if err != nil {
				return nil, err
			}
			it = fetched
		}
		targetText = strings.TrimSpace(it.Title + "\n\n" + it.Body)
		targetBrief = toBrief(it)
		skipCandidate = it.Number
	} else {
		targetText = strings.TrimSpace(a.Text)
	}
	if targetText == "" {
		return nil, errors.New("check_duplicate: empty target text")
	}
	if vec == nil {
		v, _, err := t.deps.Embedder.Embed(ctx, targetText)
		if err != nil {
			return nil, fmt.Errorf("embed target: %w", err)
		}
		vec = v
	}
	matches, err := t.deps.Store.KNN(vec, 6) // take 6 so we can drop self if hit.
	if err != nil {
		return nil, err
	}
	candidates := make([]store.Match, 0, 5)
	for _, m := range matches {
		if m.Issue.Number == skipCandidate {
			continue
		}
		candidates = append(candidates, m)
		if len(candidates) == 5 {
			break
		}
	}

	// Stage 2: structured LLM reasoning with one retry on parse failure.
	candidateNums := make([]int, len(candidates))
	for i, c := range candidates {
		candidateNums[i] = c.Issue.Number
	}
	callID := fmt.Sprintf("llm-stage2-%d-%d", skipCandidate, time.Now().UnixNano())
	argSummary, _ := json.Marshal(map[string]any{
		"target_chars": len(targetText),
		"candidates":   candidateNums,
	})
	emit.Emit(wsproto.Frame{
		Type:   wsproto.FrameToolCall,
		Name:   "llm:duplicate_reasoning",
		Args:   string(argSummary),
		CallID: callID,
	})
	res, err := t.dupReasoning(ctx, targetText, candidates, false)
	if err != nil {
		emit.Emit(wsproto.Frame{
			Type:   wsproto.FrameToolCall,
			Name:   "llm:duplicate_reasoning.retry",
			Args:   `{"reason":"parse_error"}`,
			CallID: callID + "-retry",
		})
		res, err = t.dupReasoning(ctx, targetText, candidates, true)
		if err != nil {
			emit.Emit(wsproto.Frame{
				Type:   wsproto.FrameToolResult,
				Name:   "llm:duplicate_reasoning.retry",
				CallID: callID + "-retry",
				OK:     false,
				Msg:    err.Error(),
			})
			res = dupResult{
				Tier:       "not_duplicate",
				Reasoning:  "Stage-2 reasoning failed to parse: " + err.Error(),
				Candidates: briefVerdicts(candidates),
			}
		} else {
			emit.Emit(wsproto.Frame{
				Type:   wsproto.FrameToolResult,
				Name:   "llm:duplicate_reasoning.retry",
				CallID: callID + "-retry",
				OK:     true,
			})
		}
	}
	emit.Emit(wsproto.Frame{
		Type:   wsproto.FrameToolResult,
		Name:   "llm:duplicate_reasoning",
		CallID: callID,
		OK:     err == nil,
		Msg: func() string {
			if err != nil {
				return err.Error()
			}
			return ""
		}(),
	})
	res.Tier = tierFor(res.Confidence, res.IsDuplicate)
	res.Target = targetBrief
	res.Candidates = enrichCandidates(res.Candidates, candidates)

	// Emit quick actions derived deterministically from the candidates so the
	// UI can render them as Suggestion pills without asking the LLM to author
	// markdown (which it tends to garble).
	actions := make([]wsproto.QuickAction, 0, len(res.Candidates)+1)
	if targetBrief.Number > 0 {
		actions = append(actions, wsproto.QuickAction{
			Label:  fmt.Sprintf("Summarise #%d", targetBrief.Number),
			Prompt: fmt.Sprintf("Summarise issue #%d and recommend next steps.", targetBrief.Number),
		})
	}
	for _, c := range res.Candidates {
		left := targetBrief.Number
		if left <= 0 {
			continue
		}
		actions = append(actions, wsproto.QuickAction{
			Label:  fmt.Sprintf("Compare #%d vs #%d", left, c.Number),
			Prompt: fmt.Sprintf("Compare issue #%d against issue #%d in detail.", left, c.Number),
		})
	}
	if len(actions) > 0 {
		emit.Emit(wsproto.Frame{Type: wsproto.FrameActions, Actions: actions})
	}
	return json.Marshal(res)
}

// enrichCandidates merges the LLM's per-candidate verdict onto the matched
// store entries so the UI has title + url in addition to the verdict. Unknown
// candidates (returned by the LLM but not in Stage-1 KNN) are kept verbatim.
func enrichCandidates(fromLLM []candidateVerdict, matches []store.Match) []candidateVerdict {
	if len(fromLLM) == 0 {
		return briefVerdicts(matches)
	}
	byNum := make(map[int]store.Match, len(matches))
	for _, m := range matches {
		byNum[m.Issue.Number] = m
	}
	out := make([]candidateVerdict, 0, len(fromLLM))
	seen := map[int]bool{}
	for _, v := range fromLLM {
		seen[v.Number] = true
		if m, ok := byNum[v.Number]; ok {
			v.Title = m.Issue.Title
			v.URL = m.Issue.URL
			v.State = m.Issue.State
		}
		if v.Verdict == "" {
			v.Verdict = "unknown"
		}
		out = append(out, v)
	}
	// Include any KNN candidates the LLM silently dropped.
	for _, m := range matches {
		if seen[m.Issue.Number] {
			continue
		}
		out = append(out, candidateVerdict{
			Number: m.Issue.Number, Title: m.Issue.Title, URL: m.Issue.URL,
			State: m.Issue.State, Verdict: "unknown",
		})
	}
	return out
}

func (t *Dispatcher) dupReasoning(ctx context.Context, target string, candidates []store.Match, strictRetry bool) (dupResult, error) {
	sys := `You are a GitHub issue triage assistant. Compare the TARGET issue against the CANDIDATE issues and decide whether it duplicates any of them. Respond with JSON only, no prose, matching this schema:
{"is_duplicate": bool, "confidence": number between 0 and 1, "reasoning": string, "candidates":[{"number": int, "verdict": "duplicate"|"related"|"unrelated"}]}`
	if strictRetry {
		sys += "\nPrevious response was not valid JSON. Return valid JSON only."
	}
	var sb strings.Builder
	sb.WriteString("TARGET:\n")
	sb.WriteString(target)
	sb.WriteString("\n\nCANDIDATES:\n")
	for i, c := range candidates {
		fmt.Fprintf(&sb, "[%d] #%d %s\n%s\n---\n", i+1, c.Issue.Number, c.Issue.Title, c.Issue.Body)
	}

	resp, err := t.deps.Chat.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:          t.deps.ChatModel,
		Messages:       []openai.ChatCompletionMessage{{Role: "system", Content: sys}, {Role: "user", Content: sb.String()}},
		ResponseFormat: &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeJSONObject},
	})
	if err != nil {
		return dupResult{}, err
	}
	if len(resp.Choices) == 0 {
		return dupResult{}, errors.New("no choices")
	}
	content := resp.Choices[0].Message.Content
	var parsed dupResult
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return dupResult{}, fmt.Errorf("parse stage-2 response: %w", err)
	}
	return parsed, nil
}

func briefVerdicts(cs []store.Match) []candidateVerdict {
	out := make([]candidateVerdict, len(cs))
	for i, c := range cs {
		out[i] = candidateVerdict{
			Number:  c.Issue.Number,
			Title:   c.Issue.Title,
			URL:     c.Issue.URL,
			State:   c.Issue.State,
			Verdict: "unknown",
		}
	}
	return out
}

func tierFor(confidence float64, isDup bool) string {
	if !isDup {
		if confidence >= 0.60 {
			return "human_review"
		}
		return "not_duplicate"
	}
	switch {
	case confidence >= 0.85:
		return "recommend_auto_close"
	case confidence >= 0.60:
		return "human_review"
	default:
		return "not_duplicate"
	}
}
