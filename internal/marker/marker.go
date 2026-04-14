package marker

import (
	"fmt"
	"strings"
)

const prefix = "squadai"

// OpenTag returns the opening marker for a section.
//
//	<!-- squadai:SECTION_ID -->
func OpenTag(sectionID string) string {
	return fmt.Sprintf("<!-- %s:%s -->", prefix, sectionID)
}

// CloseTag returns the closing marker for a section.
//
//	<!-- /squadai:SECTION_ID -->
func CloseTag(sectionID string) string {
	return fmt.Sprintf("<!-- /%s:%s -->", prefix, sectionID)
}

// InjectSection inserts or replaces content between marker tags in a document.
//
// Behavior:
//   - If both open and close markers exist: replaces content between them.
//   - If content is empty: removes the entire section including markers.
//   - If section not found: appends at end with markers.
//   - Content outside markers is never modified.
func InjectSection(document, sectionID, content string) string {
	open := OpenTag(sectionID)
	close := CloseTag(sectionID)

	openIdx := strings.Index(document, open)
	closeIdx := strings.Index(document, close)

	// Section exists — replace or remove.
	if openIdx >= 0 && closeIdx >= 0 && closeIdx > openIdx {
		before := document[:openIdx]
		after := document[closeIdx+len(close):]

		// Remove section entirely if content is empty.
		if content == "" {
			result := strings.TrimRight(before, "\n") + after
			if strings.TrimSpace(result) == "" {
				return ""
			}
			return result
		}

		return before + open + "\n" + content + "\n" + close + after
	}

	// Section not found — append.
	if content == "" {
		return document
	}

	block := open + "\n" + content + "\n" + close + "\n"

	if document == "" {
		return block
	}

	// Ensure a blank line before the new section.
	if !strings.HasSuffix(document, "\n\n") {
		if strings.HasSuffix(document, "\n") {
			document += "\n"
		} else {
			document += "\n\n"
		}
	}

	return document + block
}

// ExtractSection returns the content between markers, or empty string if not found.
func ExtractSection(document, sectionID string) string {
	open := OpenTag(sectionID)
	close := CloseTag(sectionID)

	openIdx := strings.Index(document, open)
	closeIdx := strings.Index(document, close)

	if openIdx < 0 || closeIdx < 0 || closeIdx <= openIdx {
		return ""
	}

	between := document[openIdx+len(open) : closeIdx]
	return strings.TrimPrefix(strings.TrimSuffix(strings.TrimRight(between, "\n"), "\n"), "\n")
}

// HasSection reports whether a document contains the markers for a section.
func HasSection(document, sectionID string) bool {
	return strings.Contains(document, OpenTag(sectionID)) &&
		strings.Contains(document, CloseTag(sectionID))
}

// StripAll removes all squadai marker blocks from document.
// Returns the stripped content and true if any blocks were found.
// Returns the original content and false if no blocks were found.
func StripAll(document string) (string, bool) {
	openPrefix := fmt.Sprintf("<!-- %s:", prefix)

	// Discover all section IDs by scanning for open tags.
	// We collect unique IDs so we don't double-strip.
	seen := make(map[string]struct{})
	var sectionIDs []string

	search := document
	for {
		idx := strings.Index(search, openPrefix)
		if idx < 0 {
			break
		}
		// Advance past the prefix to find the end of the tag: " -->"
		rest := search[idx+len(openPrefix):]
		end := strings.Index(rest, " -->")
		if end < 0 {
			break
		}
		id := rest[:end]
		// Skip close tags (they start with "/").
		if !strings.HasPrefix(id, "/") {
			if _, exists := seen[id]; !exists {
				seen[id] = struct{}{}
				sectionIDs = append(sectionIDs, id)
			}
		}
		// Advance search past this tag.
		search = rest[end+len(" -->"):]
	}

	if len(sectionIDs) == 0 {
		return document, false
	}

	// Strip each discovered section by injecting empty content.
	result := document
	for _, id := range sectionIDs {
		result = InjectSection(result, id, "")
	}

	return result, true
}
