package fileutil

import (
	"fmt"
	"strings"
)

// UnifiedDiff returns a unified diff between old and new content for the given path.
// Returns an empty string if old and new are identical.
func UnifiedDiff(path, old, newContent string) string {
	if old == newContent {
		return ""
	}

	oldLines := splitLines(old)
	newLines := splitLines(newContent)

	if len(oldLines) == 0 && len(newLines) == 0 {
		return ""
	}

	// Compute the LCS-based edit script.
	edits := computeEdits(oldLines, newLines)
	hunks := groupHunks(edits, len(oldLines), len(newLines))

	if len(hunks) == 0 {
		return ""
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "--- a/%s\n", path)
	fmt.Fprintf(&sb, "+++ b/%s\n", path)

	for _, h := range hunks {
		sb.WriteString(h)
	}

	return sb.String()
}

// edit represents a single edit operation in the diff.
type edit struct {
	kind    editKind
	oldLine int // 0-indexed line in old (for context and deletions)
	newLine int // 0-indexed line in new (for context and insertions)
	text    string
}

type editKind int

const (
	editContext editKind = iota
	editDelete
	editInsert
)

// computeEdits returns the sequence of edits using LCS-based diff.
func computeEdits(oldLines, newLines []string) []edit {
	m := len(oldLines)
	n := len(newLines)

	// Compute LCS lengths via DP.
	// dp[i][j] = LCS length of oldLines[:i] and newLines[:j]
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if oldLines[i-1] == newLines[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to build the edit sequence.
	var edits []edit
	i, j := m, n
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && oldLines[i-1] == newLines[j-1] {
			edits = append(edits, edit{kind: editContext, oldLine: i - 1, newLine: j - 1, text: oldLines[i-1]})
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			edits = append(edits, edit{kind: editInsert, newLine: j - 1, text: newLines[j-1]})
			j--
		} else {
			edits = append(edits, edit{kind: editDelete, oldLine: i - 1, text: oldLines[i-1]})
			i--
		}
	}

	// Reverse to get forward order.
	for left, right := 0, len(edits)-1; left < right; left, right = left+1, right-1 {
		edits[left], edits[right] = edits[right], edits[left]
	}

	return edits
}

const contextLines = 3

// groupHunks groups edits into unified-diff hunks with context.
// Returns formatted hunk strings (each starts with "@@ ... @@\n").
func groupHunks(edits []edit, oldTotal, newTotal int) []string {
	if len(edits) == 0 {
		return nil
	}

	// Find positions of all changed edits.
	var changePosns []int
	for idx, e := range edits {
		if e.kind != editContext {
			changePosns = append(changePosns, idx)
		}
	}
	if len(changePosns) == 0 {
		return nil
	}

	// Group change positions into hunk ranges [startEdit, endEdit).
	type hunkRange struct{ start, end int }
	var ranges []hunkRange

	start := max(0, changePosns[0]-contextLines)
	end := min(len(edits), changePosns[0]+contextLines+1)

	for _, pos := range changePosns[1:] {
		newStart := max(0, pos-contextLines)
		if newStart <= end {
			// Overlapping or adjacent — extend current range.
			end = min(len(edits), pos+contextLines+1)
		} else {
			ranges = append(ranges, hunkRange{start, end})
			start = newStart
			end = min(len(edits), pos+contextLines+1)
		}
	}
	ranges = append(ranges, hunkRange{start, end})

	var hunks []string
	for _, r := range ranges {
		hunkEdits := edits[r.start:r.end]

		// Count old/new line numbers for the hunk header.
		oldStart, oldCount := -1, 0
		newStart, newCount := -1, 0

		for _, e := range hunkEdits {
			switch e.kind {
			case editContext:
				if oldStart < 0 {
					oldStart = e.oldLine
				}
				if newStart < 0 {
					newStart = e.newLine
				}
				oldCount++
				newCount++
			case editDelete:
				if oldStart < 0 {
					oldStart = e.oldLine
				}
				oldCount++
			case editInsert:
				if newStart < 0 {
					newStart = e.newLine
				}
				newCount++
			}
		}

		// Handle edge case: all-insert hunk (old file empty).
		if oldStart < 0 {
			oldStart = 0
		}
		// Handle edge case: all-delete hunk (new file empty).
		if newStart < 0 {
			newStart = 0
		}

		// Unified diff uses 1-based line numbers.
		// Special case: 0 lines → show as "0,0" for the absent side.
		oldHeader := formatHunkSide(oldStart+1, oldCount)
		newHeader := formatHunkSide(newStart+1, newCount)

		var sb strings.Builder
		fmt.Fprintf(&sb, "@@ -%s +%s @@\n", oldHeader, newHeader)
		for _, e := range hunkEdits {
			switch e.kind {
			case editContext:
				sb.WriteString(" ")
				sb.WriteString(e.text)
				sb.WriteString("\n")
			case editDelete:
				sb.WriteString("-")
				sb.WriteString(e.text)
				sb.WriteString("\n")
			case editInsert:
				sb.WriteString("+")
				sb.WriteString(e.text)
				sb.WriteString("\n")
			}
		}
		hunks = append(hunks, sb.String())
	}

	return hunks
}

// formatHunkSide formats a unified-diff hunk side "start,count".
// When count is 0, returns "start,0" (e.g., "0,0" for create from empty).
// When count is 1, returns just "start" per unified diff convention.
func formatHunkSide(start, count int) string {
	if count == 0 {
		// When the old side is empty (pure insert), convention is "-0,0".
		return fmt.Sprintf("%d,0", start-1)
	}
	if count == 1 {
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("%d,%d", start, count)
}

// splitLines splits content into lines, stripping the trailing newline so each
// element is a "bare" line without the newline character.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	// Trim a single trailing newline to avoid a phantom empty line.
	s = strings.TrimSuffix(s, "\n")
	return strings.Split(s, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
