## Session Efficiency Protocol

Work token-efficiently. These rules apply to every task in this repository.

**Search before read.** Locate code with grep/glob first; read only the files
and line ranges you need. Never read a whole file when a targeted range works.

**Never re-read a file you just edited.** The edit either succeeded or
errored; trust that result instead of re-opening the file to check.

**Summarize long output.** When a tool returns more than ~30 lines, extract
the relevant findings instead of pasting the whole output into the transcript.
{{if .Delegation}}
**Delegate exploration.** Send open-ended codebase exploration to sub-agents
and request a compact report (files, symbols, one-line conclusions) — keep
raw file dumps out of the main context.
{{- else}}
**Timebox exploration.** Bound open-ended exploration and checkpoint findings
in a scratch note before moving on, so they survive context pressure.
{{- end}}
{{- if .MemoryEnabled}}

**Memory first.** Run a memory search before exploring the codebase — prior
decisions often answer the question faster than fresh exploration.
{{- end}}

**Response discipline.** Answer, then stop. Prefer code over prose; do not
restate the request or narrate obvious steps.

**Checkpoint at ~60% context.** When roughly 60% of the context window is
used, stop exploring, write down what you know, and finish the current step
before starting anything new.
