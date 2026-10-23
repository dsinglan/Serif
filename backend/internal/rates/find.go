package rates

import (
	"cmp"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// CodeMatch is a code whose names match the words someone typed: its card,
// how many prices the file has for it, and whether its short name alone has
// every word (when it doesn't, the full name says why the code is there).
type CodeMatch struct {
	Code
	Count            int  `json:"count"` // prices of both kinds, as a search for the code finds them
	ShortNameMatches bool `json:"short_name_matches"`
}

// FindCodes finds the codes whose names contain every word typed, in any
// capitals, the ones with the most prices first (a tie goes by code). A
// short word, such as CT or IV, must start a word in the name, so CT
// doesn't match INTERACTIVE; a longer word or a number can be anywhere, so
// "therapy" finds PSYCHOTHERAPY. It looks at the names the page shows: the
// file's short and full names, CMS's names for HCPCS codes and the standard
// names of revenue codes, not the file's own names for revenue-code items,
// most of which are wrong. A code with no name, such as one found only
// inside hospital fees that nothing names, can't be found this way.
func (idx *Index) FindCodes(typed string) []CodeMatch {
	var words []*regexp.Regexp
	for _, w := range strings.Fields(strings.ToLower(typed)) {
		if strings.Trim(w, "-") != "" { // hyphens alone aren't a word
			words = append(words, wordPattern(w))
		}
	}
	matches := []CodeMatch{}
	if len(words) == 0 {
		return matches
	}
	for key, positions := range idx.byCode {
		card := idx.describe(key, positions)
		if card != nil && containsAll(strings.ToLower(card.ShortName+" "+card.Name), words) {
			short := containsAll(strings.ToLower(card.ShortName), words)
			matches = append(matches, CodeMatch{Code: *card, Count: len(positions), ShortNameMatches: short})
		}
	}
	slices.SortFunc(matches, func(a, b CodeMatch) int {
		return cmp.Or(b.Count-a.Count, strings.Compare(a.Display, b.Display))
	})
	return matches
}

// wordPattern finds a typed word in a name. A hyphen between letters
// doesn't matter: the word's are dropped, and the name may have one between
// two characters, so "xray" finds X-RAY and "x-ray" finds XRAY. Between two
// digits a hyphen is a range, so there it's kept as typed and never
// optional: "21" doesn't find AGE 12-17. A short word (up to 3 characters
// once its hyphens are dropped, with a letter: usually an abbreviation) must
// start a word in the name: after the start of the name, or after a
// character that isn't a letter or digit (\pL is any letter, \pN any
// digit), a hyphen included, so "ray" finds X-RAY but not XRAY. Anything
// else can be anywhere: a longer word, or a number ("covid 19" finds
// COVID19).
func wordPattern(w string) *regexp.Regexp {
	typed := []rune(w)
	var kept []rune // the word without its hyphens, but for one between two digits
	for i, r := range typed {
		isRange := i > 0 && i+1 < len(typed) && unicode.IsDigit(typed[i-1]) && unicode.IsDigit(typed[i+1])
		if r != '-' || isRange {
			kept = append(kept, r)
		}
	}
	var pattern strings.Builder
	for i, r := range kept {
		if i > 0 && hyphenMayJoin(kept[i-1], r) {
			pattern.WriteString("-?")
		}
		pattern.WriteString(regexp.QuoteMeta(string(r)))
	}
	short := len(kept) <= 3 && strings.IndexFunc(string(kept), unicode.IsLetter) >= 0
	if !short {
		return regexp.MustCompile(pattern.String())
	}
	return regexp.MustCompile(`(^|[^\pL\pN])` + pattern.String())
}

// hyphenMayJoin says a name may have a hyphen between two characters of a
// typed word: not between two digits (that's a range, such as 12-17), and
// not beside a range's hyphen the word kept.
func hyphenMayJoin(before, after rune) bool {
	return before != '-' && after != '-' && !(unicode.IsDigit(before) && unicode.IsDigit(after))
}

// containsAll reports whether s has every one of words.
func containsAll(s string, words []*regexp.Regexp) bool {
	for _, w := range words {
		if !w.MatchString(s) {
			return false
		}
	}
	return true
}
