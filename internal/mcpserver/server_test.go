package mcpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func newTestServer() *Server {
	runners := map[string]ToolRunner{
		"verify": func(args []string, w io.Writer) error {
			_, _ = w.Write([]byte(`{"all_pass":true}`))
			return nil
		},
		"status": func(args []string, w io.Writer) error {
			_, _ = w.Write([]byte(`{"mode":"team"}`))
			return nil
		},
	}
	return New("test-version", runners)
}

func sendRequest(t *testing.T, s *Server, req map[string]any) map[string]any {
	t.Helper()
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	var out bytes.Buffer
	if err := s.Serve(bytes.NewReader(append(data, '\n')), &out); err != nil {
		t.Fatalf("serve: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v\nraw: %s", err, out.String())
	}
	return resp
}

func TestServer_Initialize(t *testing.T) {
	s := newTestServer()
	resp := sendRequest(t, s, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]any{"protocolVersion": "2024-11-05"},
	})

	if resp["error"] != nil {
		t.Fatalf("got error: %v", resp["error"])
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result is not an object: %v", resp["result"])
	}
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("protocolVersion = %v, want 2024-11-05", result["protocolVersion"])
	}
	si, ok := result["serverInfo"].(map[string]any)
	if !ok {
		t.Fatal("missing serverInfo")
	}
	if si["name"] != "squadai" {
		t.Errorf("serverInfo.name = %v, want squadai", si["name"])
	}
	if si["version"] != "test-version" {
		t.Errorf("serverInfo.version = %v, want test-version", si["version"])
	}
}

func TestServer_ToolsList(t *testing.T) {
	s := newTestServer()
	resp := sendRequest(t, s, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	})

	if resp["error"] != nil {
		t.Fatalf("got error: %v", resp["error"])
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result not object: %v", resp["result"])
	}
	tools, ok := result["tools"].([]any)
	if !ok || len(tools) == 0 {
		t.Fatalf("tools is empty or not an array: %v", result["tools"])
	}
	// Check a few expected tools are present.
	toolNames := make(map[string]bool)
	for _, t := range tools {
		tm, ok := t.(map[string]any)
		if !ok {
			continue
		}
		if n, ok := tm["name"].(string); ok {
			toolNames[n] = true
		}
	}
	for _, want := range []string{"plan", "apply", "verify", "status", "context"} {
		if !toolNames[want] {
			t.Errorf("tool %q not in tools/list response", want)
		}
	}
}

func TestServer_ToolsCall_Known(t *testing.T) {
	s := newTestServer()
	resp := sendRequest(t, s, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "verify",
			"arguments": map[string]any{"json": true},
		},
	})

	if resp["error"] != nil {
		t.Fatalf("got error: %v", resp["error"])
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result not object: %v", resp["result"])
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("content missing: %v", result)
	}
	first, _ := content[0].(map[string]any)
	text, _ := first["text"].(string)
	if !strings.Contains(text, "all_pass") {
		t.Errorf("expected verify output in content, got: %s", text)
	}
}

func TestServer_ToolsCall_Unknown(t *testing.T) {
	s := newTestServer()
	resp := sendRequest(t, s, map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "nonexistent",
			"arguments": map[string]any{},
		},
	})

	if resp["error"] == nil {
		t.Fatal("expected error for unknown tool, got nil")
	}
	errObj, _ := resp["error"].(map[string]any)
	msg, _ := errObj["message"].(string)
	if !strings.Contains(msg, "nonexistent") {
		t.Errorf("error message = %q, want to contain 'nonexistent'", msg)
	}
}

func TestServer_UnknownMethod(t *testing.T) {
	s := newTestServer()
	resp := sendRequest(t, s, map[string]any{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "foo/bar",
	})

	if resp["error"] == nil {
		t.Fatal("expected error for unknown method")
	}
	errObj, _ := resp["error"].(map[string]any)
	code, _ := errObj["code"].(float64)
	if code != -32601 {
		t.Errorf("error code = %v, want -32601", code)
	}
}

func TestFlattenArgs(t *testing.T) {
	tests := []struct {
		input    map[string]json.RawMessage
		contains []string
	}{
		{
			input:    map[string]json.RawMessage{"json": json.RawMessage(`true`)},
			contains: []string{"--json"},
		},
		{
			input:    map[string]json.RawMessage{"format": json.RawMessage(`"prompt"`)},
			contains: []string{"--format=prompt"},
		},
		{
			input:    map[string]json.RawMessage{"json": json.RawMessage(`false`)},
			contains: []string{},
		},
	}
	for _, tc := range tests {
		got := flattenArgs(tc.input)
		for _, want := range tc.contains {
			found := false
			for _, g := range got {
				if g == want {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("flattenArgs(%v) = %v, want to contain %q", tc.input, got, want)
			}
		}
	}
}
