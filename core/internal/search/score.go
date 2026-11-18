package search

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	tierNone = iota
	tierFuzzy
	tierPath
	tierSubstring
	tierPrefix
	tierExact
)

const (
	scoreExact     = 1000
	scorePrefix    = 800
	scoreSubstring = 600
	scorePath      = 400
	scoreFuzzy     = 200
	scoreFuzzyPath = 100
	bonusBoundary  = 50
	bonusRun       = 10
	penaltyDepth   = 3
	maxDepthPen    = 60
)

type term struct {
	text string
	glob bool
}

func parseTerms(query string) []term {
	var out []term
	for _, word := range strings.Fields(strings.ToLower(query)) {
		out = append(out, term{text: word, glob: strings.ContainsAny(word, "*?")})
	}
	return out
}

type matcher struct {
	terms []term
	fuzzy bool
	re    *regexp.Regexp
}

func newMatcher(query string, opts Options) (matcher, error) {
	if !opts.Regex || strings.TrimSpace(query) == "" {
		return matcher{terms: parseTerms(query), fuzzy: opts.Fuzzy}, nil
	}
	re, err := regexp.Compile("(?i)" + query)
	if err != nil {
		return matcher{}, err
	}
	return matcher{re: re}, nil
}

func (m matcher) selective() bool {
	return m.re != nil || len(m.terms) > 0
}

func (m matcher) score(name, nameLower, relLower string, depth int) (int, int) {
	if m.re == nil {
		return score(nameLower, relLower, depth, m.terms, m.fuzzy)
	}
	if !m.re.MatchString(name) {
		return 0, tierNone
	}
	return scoreSubstring - min(depth*penaltyDepth, maxDepthPen), tierSubstring
}

func (m matcher) scorePath(root, path string) (int, int) {
	rel := path[len(root)+1:]
	name := rel[strings.LastIndexByte(rel, '/')+1:]
	nameLower := strings.ToLower(name)
	relLower := strings.ToLower(rel[:len(rel)-len(name)]) + nameLower
	return m.score(name, nameLower, relLower, strings.Count(rel, "/"))
}

func (m matcher) fdPattern() string {
	if m.re != nil || m.fuzzy {
		return "."
	}
	longest := ""
	for _, t := range m.terms {
		for _, run := range strings.FieldsFunc(t.text, func(r rune) bool {
			return r >= utf8.RuneSelf || (t.glob && (r == '*' || r == '?'))
		}) {
			if len(run) > len(longest) {
				longest = run
			}
		}
	}
	if longest == "" {
		return "."
	}
	var pattern strings.Builder
	for _, letter := range longest {
		if letter == 'i' {
			pattern.WriteString("[iİ]")
			continue
		}
		pattern.WriteString(regexp.QuoteMeta(string(letter)))
	}
	return pattern.String()
}

func score(nameLower, relLower string, depth int, terms []term, fuzzy bool) (int, int) {
	total := 0
	weakest := tierExact
	parent := relLower[:len(relLower)-len(nameLower)]
	for _, t := range terms {
		points, tier := scoreTerm(nameLower, parent, t, fuzzy)
		if tier == tierNone {
			return 0, tierNone
		}
		total += points
		weakest = min(weakest, tier)
	}
	if len(terms) == 0 {
		weakest = tierSubstring
	}
	return total - min(depth*penaltyDepth, maxDepthPen), weakest
}

func scoreTerm(name, parent string, t term, fuzzy bool) (int, int) {
	switch {
	case t.glob:
		if wildcard(name, t.text) {
			return scoreSubstring + bonusBoundary, tierSubstring
		}
		return 0, tierNone
	case name == t.text:
		return scoreExact + bonusBoundary, tierExact
	case strings.HasPrefix(name, t.text):
		return scorePrefix + bonusBoundary, tierPrefix
	}
	if at, ok := boundaryIndex(name, t.text); ok {
		return scoreSubstring + boundaryBonus(name, at), tierSubstring
	}
	if at, ok := boundaryIndex(parent, t.text); ok {
		return scorePath + boundaryBonus(parent, at), tierPath
	}
	if !fuzzy {
		return 0, tierNone
	}
	if run, start, ok := subsequence(name, t.text); ok {
		return scoreFuzzy + run*bonusRun + boundaryBonus(name, start), tierFuzzy
	}
	if run, start, ok := subsequence(parent, t.text); ok {
		return scoreFuzzyPath + run*bonusRun + boundaryBonus(parent, start), tierFuzzy
	}
	return 0, tierNone
}

func boundaryIndex(hay, needle string) (int, bool) {
	first := strings.Index(hay, needle)
	if first < 0 {
		return 0, false
	}
	for at := first; at >= 0; {
		if atBoundary(hay, at) {
			return at, true
		}
		next := strings.Index(hay[at+1:], needle)
		if next < 0 {
			break
		}
		at += 1 + next
	}
	return first, true
}

func boundaryBonus(hay string, at int) int {
	if atBoundary(hay, at) {
		return bonusBoundary
	}
	return 0
}

func atBoundary(hay string, at int) bool {
	if at == 0 {
		return true
	}
	prev, _ := utf8.DecodeLastRuneInString(hay[:at])
	return !unicode.IsLetter(prev) && !unicode.IsDigit(prev)
}

func subsequence(hay, needle string) (run, start int, ok bool) {
	if needle == "" {
		return 0, 0, true
	}
	pos := 0
	best, current, last := 0, 0, -2
	for i := range len(needle) {
		next := strings.IndexByte(hay[pos:], needle[i])
		if next < 0 {
			return 0, 0, false
		}
		at := pos + next
		if i == 0 {
			start = at
		}
		switch at {
		case last + 1:
			current++
		default:
			current = 1
		}
		best = max(best, current)
		last = at
		pos = at + 1
	}
	return best - 1, start, true
}

func wildcard(nameText, patternText string) bool {
	name, pattern := []rune(nameText), []rune(patternText)
	at, p := 0, 0
	star, mark := -1, 0
	for at < len(name) {
		switch {
		case p < len(pattern) && (pattern[p] == '?' || pattern[p] == name[at]):
			at++
			p++
		case p < len(pattern) && pattern[p] == '*':
			star = p
			mark = at
			p++
		case star < 0:
			return false
		default:
			p = star + 1
			mark++
			at = mark
		}
	}
	for p < len(pattern) && pattern[p] == '*' {
		p++
	}
	return p == len(pattern)
}
