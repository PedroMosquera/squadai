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

// HashOpenTag returns the opening hash-comment marker for a section. Used in
// files where HTML comments are not valid syntax (e.g. TOML configs).
//
//	# squadai:SECTION_ID:start
func HashOpenTag(sectionID string) string {
	return fmt.Sprintf("# %s:%s:start", prefix, sectionID)
}

// HashCloseTag returns the closing hash-comment marker for a section.
//
//	# squadai:SECTION_ID:end
func HashCloseTag(sectionID string) string {
	return fmt.Sprintf("# %s:%s:end", prefix, sectionID)
}

// InjectSection inserts or replaces content between marker tags in a document.
//
// Behavior:
//   - If both open and close markers exist: replaces content between them.
//   - If content is empty: removes the entire section including markers.
//   - If section not found: appends at end with markers.
//   - Content outside markers is never modified.
func InjectSection(document, sectionID, content string) string {
	return injectBetween(document, OpenTag(sectionID), CloseTag(sectionID), content)
}

// InjectHashSection is InjectSection for hash-comment markers (TOML/shell-style
// files where HTML comments are invalid syntax).
func InjectHashSection(document, sectionID, content string) string {
	return injectBetween(document, HashOpenTag(sectionID), HashCloseTag(sectionID), content)
}

// injectBetween inserts or replaces content between arbitrary open/close tags.
func injectBetween(document, open, close, content string) string {
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
	return extractBetween(document, OpenTag(sectionID), CloseTag(sectionID))
}

// ExtractHashSection is ExtractSection for hash-comment markers.
func ExtractHashSection(document, sectionID string) string {
	return extractBetween(document, HashOpenTag(sectionID), HashCloseTag(sectionID))
}

// extractBetween returns the content between arbitrary open/close tags.
func extractBetween(document, open, close string) string {
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

// HasHashSection reports whether a document contains hash-comment markers for a section.
func HasHashSection(document, sectionID string) bool {
	return strings.Contains(document, HashOpenTag(sectionID)) &&
		strings.Contains(document, HashCloseTag(sectionID))
}

// SectionIDs returns the IDs of the HTML-comment sections in document, each
// once, in order of first appearance.
func SectionIDs(document string) []string {
	openPrefix := fmt.Sprintf("<!-- %s:", prefix)
	seen := make(map[string]struct{})
	var ids []string
	search := document
	for {
		idx := strings.Index(search, openPrefix)
		if idx < 0 {
			break
		}
		rest := search[idx+len(openPrefix):]
		end := strings.Index(rest, " -->")
		if end < 0 {
			break
		}
		id := rest[:end]
		if !strings.HasPrefix(id, "/") {
			if _, exists := seen[id]; !exists {
				seen[id] = struct{}{}
				ids = append(ids, id)
			}
		}
		search = rest[end+len(" -->"):]
	}
	return ids
}

// StripAll removes all squadai marker blocks from document.
// Returns the stripped content and true if any blocks were found.
// Returns the original content and false if no blocks were found.
func StripAll(document string) (string, bool) {
	return StripAllExcept(document)
}

// StripAllExcept is StripAll leaving the keep sections in place. The bool
// reports whether anything was removed.
func StripAllExcept(document string, keep ...string) (string, bool) {
	kept := make(map[string]bool, len(keep))
	for _, id := range keep {
		kept[id] = true
	}
	result := document
	found := false
	for _, id := range SectionIDs(document) {
		if kept[id] {
			continue
		}
		result = InjectSection(result, id, "")
		found = true
	}
	for _, id := range discoverHashSectionIDs(document) {
		if kept[id] {
			continue
		}
		result = InjectHashSection(result, id, "")
		found = true
	}
	if !found {
		return document, false
	}
	return result, true
}

// StripScoped removes every adapter-scoped variant of base ("base:<agent>"),
// left behind when each adapter wrote its own copy of a shared block.
func StripScoped(document, base string) string {
	for _, id := range SectionIDs(document) {
		if strings.HasPrefix(id, base+":") {
			document = InjectSection(document, id, "")
		}
	}
	return document
}

// discoverHashSectionIDs scans a document for hash-comment open tags
// ("# squadai:<id>:start" on its own line) and returns the unique section IDs.
func discoverHashSectionIDs(document string) []string {
	openPrefix := fmt.Sprintf("# %s:", prefix)
	const openSuffix = ":start"

	seen := make(map[string]struct{})
	var ids []string

	for _, line := range strings.Split(document, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, openPrefix) || !strings.HasSuffix(trimmed, openSuffix) {
			continue
		}
		id := trimmed[len(openPrefix) : len(trimmed)-len(openSuffix)]
		if id == "" {
			continue
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	return ids
}
