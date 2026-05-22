package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/PedroMosquera/squadai/internal/domain"
)

// formatPlannedActions renders a compact, grouped summary of planned actions.
// When verbose is true, every action is listed individually with its target path
// (the legacy format). When verbose is false, actions are grouped by agent and
// then by action type (create / update / delete / skip), with per-component
// counts shown inline.
//
// Example non-verbose output (62 actions across 4 agents):
//
//	Planned actions (62):
//	  claude-code (15)
//	    create: skills(8) agents(4) commands(3)
//	  opencode (12)
//	    create: skills(8) agents(4)
//	  vscode-copilot (5)
//	    create: skills(4) settings(1)
//	  windsurf (30)
//	    create: skills(8) agents(8) commands(6) memory(1) rules(1)
//	    update: settings(1)
//	    skip:   mcp(5)
func formatPlannedActions(actions []domain.PlannedAction, verbose bool) string {
	var b strings.Builder

	if verbose {
		for _, a := range actions {
			fmt.Fprintf(&b, "  %-8s %-40s %s\n", a.Action, a.Description, a.TargetPath)
		}
		return b.String()
	}

	// Group: agent -> action -> component -> count.
	type compCount struct {
		comp  domain.ComponentID
		count int
	}
	type actionGroup struct {
		action domain.ActionType
		total  int
		comps  []compCount
	}

	type agentBucket struct {
		agent   domain.AgentID
		total   int
		actions []actionGroup
	}

	// Build nested counts.
	agents := map[domain.AgentID]map[domain.ActionType]map[domain.ComponentID]int{}
	for _, a := range actions {
		am, ok := agents[a.Agent]
		if !ok {
			am = map[domain.ActionType]map[domain.ComponentID]int{}
			agents[a.Agent] = am
		}
		cm, ok := am[a.Action]
		if !ok {
			cm = map[domain.ComponentID]int{}
			am[a.Action] = cm
		}
		cm[a.Component]++
	}

	// Stable agent ordering: alphabetical by ID.
	agentIDs := make([]domain.AgentID, 0, len(agents))
	for id := range agents {
		agentIDs = append(agentIDs, id)
	}
	sort.Slice(agentIDs, func(i, j int) bool { return string(agentIDs[i]) < string(agentIDs[j]) })

	// Action order: create, update, delete, skip (then anything else alpha).
	actionOrder := func(a domain.ActionType) int {
		switch a {
		case domain.ActionCreate:
			return 0
		case domain.ActionUpdate:
			return 1
		case domain.ActionDelete:
			return 2
		case domain.ActionSkip:
			return 3
		default:
			return 4
		}
	}

	buckets := make([]agentBucket, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		am := agents[agentID]
		bucket := agentBucket{agent: agentID}

		actionTypes := make([]domain.ActionType, 0, len(am))
		for at := range am {
			actionTypes = append(actionTypes, at)
		}
		sort.Slice(actionTypes, func(i, j int) bool {
			oi, oj := actionOrder(actionTypes[i]), actionOrder(actionTypes[j])
			if oi != oj {
				return oi < oj
			}
			return string(actionTypes[i]) < string(actionTypes[j])
		})

		for _, at := range actionTypes {
			cm := am[at]
			grp := actionGroup{action: at}
			for c, n := range cm {
				grp.comps = append(grp.comps, compCount{comp: c, count: n})
				grp.total += n
			}
			// Component ordering: count desc, then alpha.
			sort.Slice(grp.comps, func(i, j int) bool {
				if grp.comps[i].count != grp.comps[j].count {
					return grp.comps[i].count > grp.comps[j].count
				}
				return string(grp.comps[i].comp) < string(grp.comps[j].comp)
			})
			bucket.actions = append(bucket.actions, grp)
			bucket.total += grp.total
		}
		buckets = append(buckets, bucket)
	}

	// Render.
	for _, bucket := range buckets {
		fmt.Fprintf(&b, "  %s (%d)\n", bucket.agent, bucket.total)
		for _, grp := range bucket.actions {
			parts := make([]string, 0, len(grp.comps))
			for _, c := range grp.comps {
				parts = append(parts, fmt.Sprintf("%s(%d)", c.comp, c.count))
			}
			fmt.Fprintf(&b, "    %-7s %s\n", string(grp.action)+":", strings.Join(parts, " "))
		}
	}
	return b.String()
}

// writePlannedActions writes the formatted summary to stdout.
func writePlannedActions(stdout io.Writer, actions []domain.PlannedAction, verbose bool) {
	fmt.Fprint(stdout, formatPlannedActions(actions, verbose))
}
