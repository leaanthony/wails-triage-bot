package wsproto

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFrameOmitempty(t *testing.T) {
	f := Frame{Type: FrameToken, Data: "hi"}
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"type":"token"`) {
		t.Errorf("missing type: %s", s)
	}
	if !strings.Contains(s, `"data":"hi"`) {
		t.Errorf("missing data: %s", s)
	}
	for _, k := range []string{"name", "args", "ok", "msg", "tokens", "payload", "call_id", "actions"} {
		if strings.Contains(s, `"`+k+`"`) {
			t.Errorf("expected %q omitted, got %s", k, s)
		}
	}
}

func TestFrameRoundTrip(t *testing.T) {
	orig := Frame{
		Type:    FrameToolCall,
		Name:    "search_issues",
		Args:    `{"query":"x"}`,
		CallID:  "c1",
		OK:      true,
		Tokens:  42,
		Payload: json.RawMessage(`{"k":"v"}`),
		Actions: []QuickAction{{Label: "L", Prompt: "P"}},
	}
	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}
	var got Frame
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Type != orig.Type || got.Name != orig.Name || got.Args != orig.Args ||
		got.CallID != orig.CallID || !got.OK || got.Tokens != 42 {
		t.Errorf("mismatch: %+v", got)
	}
	if len(got.Actions) != 1 || got.Actions[0].Label != "L" || got.Actions[0].Prompt != "P" {
		t.Errorf("actions mismatch: %+v", got.Actions)
	}
}

func TestFrameTypeConstants(t *testing.T) {
	cases := map[FrameType]string{
		FrameToken:      "token",
		FrameToolCall:   "tool_call",
		FrameToolResult: "tool_result",
		FrameDone:       "done",
		FrameError:      "error",
		FrameUser:       "user",
		FrameLog:        "log",
		FrameActions:    "quick_actions",
	}
	for ft, want := range cases {
		if string(ft) != want {
			t.Errorf("FrameType=%q want %q", ft, want)
		}
	}
}
