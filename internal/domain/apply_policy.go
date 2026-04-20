package domain

// ApplyPolicy carries per-apply decisions from the review screen (or CLI flags)
// down to the component installers. It is handed to installers that implement
// PolicyAware right before Apply is called.
//
// Overrides records the user's explicit consent to overwrite specific
// top-level keys on specific target files even though those keys are not
// currently tracked as SquadAI-managed. The key of the outer map is the
// target file's path as the installer reports it in PreviewEntry.TargetPath;
// the inner set lists keys within that file to overwrite.
//
// OverwriteAll is a coarse escape hatch for non-interactive paths (CI,
// --overwrite-unmanaged, --force): when true, installers should treat every
// conflict as overridden. It exists so non-TTY callers don't have to
// enumerate every (file, key) pair up front.
type ApplyPolicy struct {
	Overrides    map[string]map[string]bool
	OverwriteAll bool
}

// AllowOverride reports whether the policy permits overwriting key on the
// target file identified by targetPath. Nil-safe: a zero-value policy denies
// all overrides.
func (p ApplyPolicy) AllowOverride(targetPath, key string) bool {
	if p.OverwriteAll {
		return true
	}
	keys, ok := p.Overrides[targetPath]
	if !ok {
		return false
	}
	return keys[key]
}

// EffectiveOverrides returns the list of top-level keys that the installer
// should treat as "already managed" for this target. It folds OverwriteAll
// into the returned list by expanding to every incomingKey when set, so the
// caller can pass a single slice down to fileutil.MergeAndWriteJSON without
// special-casing the coarse mode.
//
// Returns nil when no override applies (zero-value or empty policy).
func (p ApplyPolicy) EffectiveOverrides(targetPath string, incomingKeys []string) []string {
	if p.OverwriteAll {
		out := make([]string, len(incomingKeys))
		copy(out, incomingKeys)
		return out
	}
	keys, ok := p.Overrides[targetPath]
	if !ok || len(keys) == 0 {
		return nil
	}
	out := make([]string, 0, len(keys))
	for k := range keys {
		out = append(out, k)
	}
	return out
}

// PolicyAware is an optional interface that component installers implement to
// receive an ApplyPolicy before Apply runs. The pipeline executor calls
// SetApplyPolicy in a single pass before Apply so each installer can factor
// user decisions into its merge behavior.
//
// Installers that do not implement this interface operate as if the policy
// were zero-valued — no overrides, no coarse overwrite — which matches the
// legacy all-or-nothing behavior.
type PolicyAware interface {
	SetApplyPolicy(ApplyPolicy)
}

// ReviewDecision is the return type of the review prompt hook. The TUI
// (or any other prompt implementation) builds this from the user's per-entry
// choices and hands it back to the apply command.
//
// When Confirmed is false the apply is canceled; Policy is ignored. When
// Confirmed is true the caller hands Policy to the pipeline so installers
// can honor per-key overwrite decisions.
type ReviewDecision struct {
	Confirmed bool
	Policy    ApplyPolicy
}
