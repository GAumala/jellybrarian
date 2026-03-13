package text

import (
	"strings"
	"unicode"
)

// accentFold maps common accented runes to ASCII equivalents for search.
var accentFold = func() map[rune]rune {
	// Latin-1 and common Latin Extended: á→a, ó→o, etc.
	m := map[rune]rune{
		'á': 'a', 'à': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a', 'å': 'a', 'ā': 'a',
		'é': 'e', 'è': 'e', 'ê': 'e', 'ë': 'e', 'ē': 'e',
		'í': 'i', 'ì': 'i', 'î': 'i', 'ï': 'i', 'ī': 'i',
		'ó': 'o', 'ò': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o', 'ō': 'o',
		'ú': 'u', 'ù': 'u', 'û': 'u', 'ü': 'u', 'ū': 'u',
		'ý': 'y', 'ÿ': 'y', 'ñ': 'n', 'ç': 'c', 'œ': 'o', 'ß': 's',
	}
	return m
}()

// NormalizeForSearch returns a lowercased, accent-folded form of s for matching.
// Used so that "one" matches "ONE PIECE" and "ó" matches "o".
func NormalizeForSearch(s string) string {
	var b strings.Builder
	for _, r := range s {
		if folded, ok := accentFold[r]; ok {
			b.WriteRune(folded)
		} else if unicode.Is(unicode.Mn, r) {
			continue // skip combining marks
		} else {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}
