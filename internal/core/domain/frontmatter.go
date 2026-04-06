package domain

import (
	"errors"
	"fmt"
	"strings"
)

// FrontMatterField describes a single key-value pair in a YAML
// front-matter header. ReadOnly fields are annotated with a comment
// so the user knows not to change them.
type FrontMatterField struct {
	Key      string
	Value    string
	ReadOnly bool
}

// ErrMalformedFrontMatter is returned when the front-matter block
// cannot be parsed (missing delimiters or invalid YAML).
var ErrMalformedFrontMatter = errors.New("malformed front-matter")

// FormatFrontMatter builds a YAML front-matter string from a list of
// fields and a markdown body:
//
//	---
//	key: value          # read-only
//	key2: value2
//	---
//
//	body text here
func FormatFrontMatter(fields []FrontMatterField, body string) string {
	var b strings.Builder
	b.WriteString("---\n")
	for _, f := range fields {
		if f.ReadOnly {
			fmt.Fprintf(&b, "%s: %s  # read-only\n", f.Key, f.Value)
		} else {
			fmt.Fprintf(&b, "%s: %s\n", f.Key, f.Value)
		}
	}
	b.WriteString("---\n")
	if body != "" {
		b.WriteString("\n")
		b.WriteString(body)
	}
	return b.String()
}

// ParseFrontMatter splits a YAML front-matter block from the body.
// It returns the key-value pairs (as a map) and the remaining body
// text. Lines starting with # inside the YAML block are treated as
// comments and stripped from values.
func ParseFrontMatter(content string) (map[string]string, string, error) {
	// Must start with ---
	if !strings.HasPrefix(content, "---\n") {
		return nil, "", fmt.Errorf("%w: missing opening ---", ErrMalformedFrontMatter)
	}

	// Find the closing ---
	rest := content[4:] // skip first "---\n"
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		// Check if it ends with \n---
		if strings.HasSuffix(rest, "\n---") {
			idx = len(rest) - 4
		} else {
			return nil, "", fmt.Errorf("%w: missing closing ---", ErrMalformedFrontMatter)
		}
	}

	yamlBlock := rest[:idx]
	body := ""
	closingEnd := idx + 5 // len("\n---\n")
	if closingEnd < len(rest) {
		body = rest[closingEnd:]
		// Strip leading blank line between --- and body
		body = strings.TrimPrefix(body, "\n")
	}

	fields := make(map[string]string)
	for _, line := range strings.Split(yamlBlock, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			return nil, "", fmt.Errorf("%w: line missing colon: %q", ErrMalformedFrontMatter, line)
		}
		key := strings.TrimSpace(line[:colonIdx])
		val := strings.TrimSpace(line[colonIdx+1:])
		// Strip inline comment (# read-only, etc.)
		if commentIdx := strings.Index(val, "  #"); commentIdx >= 0 {
			val = strings.TrimSpace(val[:commentIdx])
		}
		fields[key] = val
	}

	return fields, body, nil
}
