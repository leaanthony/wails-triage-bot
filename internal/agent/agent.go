// Package agent implements the streaming ReAct loop that drives the chat UI.
package agent

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"github.com/leaanthony/wails-triage-bot/internal/tools"
	"github.com/leaanthony/wails-triage-bot/internal/wsproto"
)

//go:embed system_prompt.txt
var systemPromptTemplate string

const (
	MaxIterations = 8
	DefaultModel  = "gpt-4o"
)

// SystemPrompt returns the system prompt with today's UTC date substituted.
func SystemPrompt() string {
	return strings.ReplaceAll(systemPromptTemplate, "{{DATE}}", time.Now().UTC().Format("2006-01-02"))
}

// Emitter sends frames to the client.
type Emitter interface {
	Emit(wsproto.Frame)
}

// EmitterFunc adapts a function to the Emitter interface.
type EmitterFunc func(wsproto.Frame)

func (f EmitterFunc) Emit(frame wsproto.Frame) { f(frame) }

// toolsEmitter adapts an agent.Emitter to the tools.Emitter interface.
type toolsEmitter struct{ e Emitter }

func (t toolsEmitter) Emit(f wsproto.Frame) { t.e.Emit(f) }

// Agent wires up a chat client + dispatcher into a ReAct loop.
type Agent struct {
	chat       *openai.Client
	model      string
	dispatcher *tools.Dispatcher
	tools      []openai.Tool
}

func New(chat *openai.Client, model string, d *tools.Dispatcher) *Agent {
	if model == "" {
		model = DefaultModel
	}
	return &Agent{
		chat:       chat,
		model:      model,
		dispatcher: d,
		tools:      tools.Schemas(),
	}
}

// Run drives the ReAct loop for a single user turn, streaming frames to out.
func (a *Agent) Run(ctx context.Context, history []openai.ChatCompletionMessage, userMsg string, out Emitter) ([]openai.ChatCompletionMessage, error) {
	msgs := append([]openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem, Content: SystemPrompt()}}, history...)
	msgs = append(msgs, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: userMsg})

	for iter := 0; iter < MaxIterations; iter++ {
		stepID := fmt.Sprintf("llm-chat-%d", iter+1)
		stepLabel := "llm:thinking"
		stepArgs := fmt.Sprintf(`{"iteration":%d}`, iter+1)
		if iter > 0 {
			stepLabel = "llm:reviewing tool results"
		}
		out.Emit(wsproto.Frame{Type: wsproto.FrameToolCall, Name: stepLabel, Args: stepArgs, CallID: stepID})

		assistant, err := a.streamCompletion(ctx, msgs, out)
		if err != nil {
			out.Emit(wsproto.Frame{Type: wsproto.FrameToolResult, Name: stepLabel, CallID: stepID, OK: false, Msg: err.Error()})
			out.Emit(wsproto.Frame{Type: wsproto.FrameError, Msg: err.Error()})
			return history, err
		}
		out.Emit(wsproto.Frame{Type: wsproto.FrameToolResult, Name: stepLabel, CallID: stepID, OK: true})
		msgs = append(msgs, assistant)

		if len(assistant.ToolCalls) == 0 {
			out.Emit(wsproto.Frame{Type: wsproto.FrameDone})
			// Return history excluding the system message we prepended.
			return msgs[1:], nil
		}

		for _, tc := range assistant.ToolCalls {
			argSummary := truncate(tc.Function.Arguments, 200)
			log.Printf("tool call: %s args=%s", tc.Function.Name, truncate(tc.Function.Arguments, 400))
			out.Emit(wsproto.Frame{Type: wsproto.FrameToolCall, Name: tc.Function.Name, Args: argSummary, CallID: tc.ID})

			result, terr := a.dispatcher.Dispatch(ctx, tc.Function.Name, json.RawMessage(tc.Function.Arguments), toolsEmitter{out})
			if terr != nil {
				out.Emit(wsproto.Frame{Type: wsproto.FrameToolResult, Name: tc.Function.Name, CallID: tc.ID, OK: false, Msg: terr.Error()})
				msgs = append(msgs, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					ToolCallID: tc.ID,
					Content:    `{"error":` + jsonString(terr.Error()) + `}`,
				})
				continue
			}
			out.Emit(wsproto.Frame{Type: wsproto.FrameToolResult, Name: tc.Function.Name, CallID: tc.ID, OK: true})
			msgs = append(msgs, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				ToolCallID: tc.ID,
				Content:    string(result),
			})
		}
	}

	err := errors.New("agent hit max iterations without final response")
	out.Emit(wsproto.Frame{Type: wsproto.FrameError, Msg: err.Error()})
	return history, err
}

// streamCompletion runs one chat completion in streaming mode, forwarding
// assistant tokens as FrameToken, and assembles the final assistant message
// (including tool calls) from the stream.
func (a *Agent) streamCompletion(ctx context.Context, msgs []openai.ChatCompletionMessage, out Emitter) (openai.ChatCompletionMessage, error) {
	stream, err := a.chat.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:    a.model,
		Messages: msgs,
		Tools:    a.tools,
	})
	if err != nil {
		return openai.ChatCompletionMessage{}, fmt.Errorf("chat stream: %w", err)
	}
	defer stream.Close()

	var (
		contentBuf []byte
		toolCalls  []openai.ToolCall
	)
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return openai.ChatCompletionMessage{}, fmt.Errorf("stream recv: %w", err)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta
		if delta.Content != "" {
			contentBuf = append(contentBuf, delta.Content...)
			out.Emit(wsproto.Frame{Type: wsproto.FrameToken, Data: delta.Content})
		}
		for _, tc := range delta.ToolCalls {
			idx := 0
			if tc.Index != nil {
				idx = *tc.Index
			}
			for len(toolCalls) <= idx {
				toolCalls = append(toolCalls, openai.ToolCall{Type: openai.ToolTypeFunction})
			}
			if tc.ID != "" {
				toolCalls[idx].ID = tc.ID
			}
			if tc.Function.Name != "" {
				toolCalls[idx].Function.Name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				toolCalls[idx].Function.Arguments += tc.Function.Arguments
			}
		}
	}

	msg := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: string(contentBuf)}
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}
	return msg, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func jsonString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		log.Printf("agent: marshal string: %v", err)
		return `""`
	}
	return string(b)
}
