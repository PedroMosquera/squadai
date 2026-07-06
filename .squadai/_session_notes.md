# Session working notes — 2026-07-06

## Phase
Exploration / orientation. No active task pipeline yet.

## Key facts (from .squadai/project.json)
- This repo IS squadai (Go CLI), running its own `full-squad` preset.
- Adapters on: claude-code, opencode, pi.
- Methodology: sdd. Team = orchestrator/explorer/designer/proposer/spec-writer/
  task-planner/implementer/verifier — each subagent + skill_ref + model tier.
- Memory: native backend, auto-capture, docs/memory/ (decisions, learnings,
  incidents, _inbox).
- Context profiles: cheap/default/debug/docs/feature/incident/review.
- Usage: 200k/day, 50k/session, enforcement=warn.
- Canonical cmds: go test ./... / go build ./... / go vet ./...

## Open
- No current task. Awaiting user direction.
