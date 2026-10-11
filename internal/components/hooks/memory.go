package hooks

import "github.com/PedroMosquera/squadai/internal/domain"

// memoryHookPrefix marks the session-end capture hook, which SquadAI owns the
// same way it owns budget hooks: rewritten and removed to follow config.
const memoryHookPrefix = "squadai memory-hook "

// memoryHookTimeout bounds a Stop hook that only reads config and counts
// transcript lines, well under Claude Code's 600s default.
const memoryHookTimeout = 10

// MemoryHooks returns the Claude Code Stop entry that asks the agent to record
// the session in project memory, or nil unless captureOnStop is true. The
// caller decides captureOnStop from memory.capture_on_stop and whether the
// memory component is enabled. No onFailure: a failing capture hook must let
// the session end rather than keep Claude working.
func MemoryHooks(captureOnStop bool) domain.HooksConfig {
	if !captureOnStop {
		return nil
	}
	e := domain.HookEntry{Type: "command", Command: memoryHookPrefix + "--event Stop", Timeout: memoryHookTimeout}
	return domain.HooksConfig{"Stop": []domain.HookMatcher{{Hooks: []domain.HookEntry{e}}}}
}

// WithMemoryHooks returns base plus MemoryHooks(captureOnStop), leaving base
// unmodified.
func WithMemoryHooks(base domain.HooksConfig, captureOnStop bool) domain.HooksConfig {
	mem := MemoryHooks(captureOnStop)
	if len(mem) == 0 {
		return base
	}
	out := make(domain.HooksConfig, len(base)+len(mem))
	for ev, ms := range base {
		out[ev] = append([]domain.HookMatcher(nil), ms...)
	}
	for ev, ms := range mem {
		out[ev] = append(out[ev], ms...)
	}
	return out
}
