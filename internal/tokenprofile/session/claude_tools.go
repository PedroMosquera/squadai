package session

import (
	"bufio"
	"encoding/json"
	"os"
)

// ClaudeTranscriptToolCalls returns the number of tool_use blocks in one
// Claude Code transcript. An unreadable file counts as zero. The transcript
// is written asynchronously, so the latest turn may not be counted yet.
func ClaudeTranscriptToolCalls(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), claudeScanBufferSize)
	n := 0
	for scanner.Scan() {
		var ev struct {
			Type    string `json:"type"`
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(scanner.Bytes(), &ev) != nil || ev.Type != "assistant" {
			continue
		}
		// content is a string for plain text replies and an array of blocks otherwise.
		var blocks []struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(ev.Message.Content, &blocks) != nil {
			continue
		}
		for _, b := range blocks {
			if b.Type == "tool_use" {
				n++
			}
		}
	}
	return n
}
