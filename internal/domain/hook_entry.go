package domain

import "encoding/json"

// HookEntry is a single Claude Code hook handler. SquadAI only writes
// "command" handlers.
type HookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Async   bool   `json:"async,omitempty"`
	Timeout int    `json:"timeout,omitempty"` // seconds; 0 = agent default
	// OnFailure "block" makes a hook that cannot start, times out or exits
	// with an unexpected code act like exit 2 (Claude Code v2.1.295+).
	OnFailure string `json:"onFailure,omitempty"`
	// Extra keeps handler fields SquadAI does not model (if, args,
	// statusMessage, ...) so merging into .claude/settings.json writes a
	// user's hook back intact instead of silently widening it.
	Extra map[string]json.RawMessage `json:"-"`
}

var hookEntryModeled = []string{"type", "command", "async", "timeout", "onFailure"}

func (h *HookEntry) UnmarshalJSON(data []byte) error {
	type plain HookEntry
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return err
	}
	for _, k := range hookEntryModeled {
		delete(all, k)
	}
	if len(all) > 0 {
		p.Extra = all
	}
	*h = HookEntry(p)
	return nil
}

func (h HookEntry) MarshalJSON() ([]byte, error) {
	type plain HookEntry
	data, err := json.Marshal(plain(h))
	if err != nil || len(h.Extra) == 0 {
		return data, err
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, err
	}
	for k, v := range h.Extra {
		if _, modeled := all[k]; !modeled {
			all[k] = v
		}
	}
	return json.Marshal(all)
}
