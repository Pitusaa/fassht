package fuzzy

import (
	"path/filepath"
	"sort"
	"strings"
)

// Filter returns items from the list that match query (case-insensitive),
// ranked so that the most relevant matches appear first:
//  1. Basename contains query as a substring
//  2. Full path contains query as a substring
//  3. Pure subsequence match
//
// If query is empty, all items are returned unchanged.
func Filter(items []string, query string) []string {
	if query == "" {
		return items
	}
	q := strings.ToLower(query)

	type ranked struct {
		item string
		tier int // lower = better
	}
	var matches []ranked
	for _, item := range items {
		lower := strings.ToLower(item)
		base := strings.ToLower(filepath.Base(item))
		var tier int
		switch {
		case strings.Contains(base, q):
			tier = 0
		case strings.Contains(lower, q):
			tier = 1
		case isSubsequence(q, lower):
			tier = 2
		default:
			continue
		}
		matches = append(matches, ranked{item, tier})
	}

	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].tier < matches[j].tier
	})

	result := make([]string, len(matches))
	for i, m := range matches {
		result[i] = m.item
	}
	return result
}

// isSubsequence returns true if every character of needle appears in haystack
// in order.
func isSubsequence(needle, haystack string) bool {
	haystackRunes := []rune(haystack)
	hi := 0
	for _, c := range needle {
		found := false
		for hi < len(haystackRunes) {
			if haystackRunes[hi] == c {
				hi++
				found = true
				break
			}
			hi++
		}
		if !found {
			return false
		}
	}
	return true
}
