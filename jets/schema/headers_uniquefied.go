package schema

import (
	"fmt"
	"strings"
)

// HeadersUniquefied is a struct that holds the uniquefied headers for a schema.
type HeadersUniquefied struct {
	Modified        bool
	OriginalHeaders []string
	UniqueHeaders   []string
}

// NewHeadersUniquefied creates a new HeadersUniquefied instance.
func NewHeadersUniquefied(originalHeaders []string) *HeadersUniquefied {
	uniqueHeaders, modified := uniquefyHeaders(originalHeaders)
	return &HeadersUniquefied{
		Modified:        modified,
		OriginalHeaders: originalHeaders,
		UniqueHeaders:   uniqueHeaders,
	}
}

// uniquefyHeaders takes a slice of strings and returns a new slice with duplicate
// strings made unique by adding a _n suffix where n is 1, 2 ,3...
// Preserving the length of the original slice.
func uniquefyHeaders(headers []string) ([]string, bool) {
	seen := make(map[string]int)
	modified := false
	unique := make([]string, 0, len(headers))
	for _, header := range headers {
		if _, ok := seen[header]; !ok {
			seen[header] = 1
			unique = append(unique, header)
		} else {
			seen[header]++
			unique = append(unique, fmt.Sprintf("%s_%d", header, seen[header]))
			modified = true
		}
	}
	return unique, modified
}

// String returns a string representation of the HeadersUniquefied instance.
func (h *HeadersUniquefied) String() string {
	var sb strings.Builder
	sb.WriteString("Original Headers:\n")
	for _, header := range h.OriginalHeaders {
		sb.WriteString(fmt.Sprintf(" - %s\n", header))
	}
	sb.WriteString("Unique Headers:\n")
	for _, header := range h.UniqueHeaders {
		sb.WriteString(fmt.Sprintf(" - %s\n", header))
	}
	return sb.String()
}
