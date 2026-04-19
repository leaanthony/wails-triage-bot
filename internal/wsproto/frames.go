// Package wsproto defines the JSON envelope exchanged between the agent and
// the browser over the WebSocket transport.
package wsproto

import "encoding/json"

type FrameType string

const (
	FrameToken      FrameType = "token"
	FrameToolCall   FrameType = "tool_call"
	FrameToolResult FrameType = "tool_result"
	FrameDone       FrameType = "done"
	FrameError      FrameType = "error"
	FrameUser       FrameType = "user" // inbound from client.
	FrameLog        FrameType = "log"  // server log line.
	FrameActions    FrameType = "quick_actions"
)

type QuickAction struct {
	Label  string `json:"label"`
	Prompt string `json:"prompt"`
}

type Frame struct {
	Type    FrameType       `json:"type"`
	Data    string          `json:"data,omitempty"`
	Name    string          `json:"name,omitempty"`
	Args    string          `json:"args,omitempty"`
	OK      bool            `json:"ok,omitempty"`
	Msg     string          `json:"msg,omitempty"`
	Tokens  int             `json:"tokens,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	CallID  string          `json:"call_id,omitempty"`
	Actions []QuickAction   `json:"actions,omitempty"`
}
