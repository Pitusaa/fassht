package fuzzy

import "strings"

// Filter returns items from the list whose path contains all characters of
// query as a subsequence (case-insensitive).
// If query is empty, all items are returned.
func Filter(items []string, query string) []string {
	if query == "" {
		return items
	}
	q := strings.ToLower(query)
	var result []string
	for _, item := range items {
		if isSubsequence(q, strings.ToLower(item)) {
			result = append(result, item)
		}
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
