package kit

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// FuzzyScore scores how well query matches s as a subsequence, ignoring
// case: higher is better, -1 is no match. Matches at the start, at word
// starts and in runs score more; a prefix scores most. It allocates
// nothing, so it can run over every room on each keystroke.
func FuzzyScore(query, s string) int {
	if query == "" {
		return 0
	}
	score, run := 0, 0
	qi := 0
	prev := ' '
	q, qsize := utf8.DecodeRuneInString(query)
	q = unicode.ToLower(q)
	for i, r := range s {
		lr := unicode.ToLower(r)
		if lr == q {
			pts := 1
			if i == 0 {
				pts += 8
			} else if !unicode.IsLetter(prev) && !unicode.IsDigit(prev) {
				pts += 5
			}
			run++
			pts += run * 2
			score += pts
			qi += qsize
			if qi >= len(query) {
				if strings.HasPrefix(strings.ToLower(s), strings.ToLower(query)) {
					score += 20
				}
				return score - (utf8.RuneCountInString(s) / 8)
			}
			q, qsize = utf8.DecodeRuneInString(query[qi:])
			q = unicode.ToLower(q)
		} else {
			run = 0
		}
		prev = r
	}
	return -1
}
