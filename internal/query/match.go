package query

import (
	"slices"
	"strings"
	"unicode/utf8"
)

// Matcher places what a query's words match in a text: the highlights of
// a hit's snippet, and a sheet's find. It reads texts with the index's
// analyzer, so it finds what the index found.
type Matcher struct {
	patterns []pattern
}

// pattern is one term to find: its terms at their positions, an empty
// slot taking any term.
type pattern struct {
	slots [][]string
	// prefix lets the last slot's term start a longer one; typos is how
	// many edits a lone word may be away.
	prefix bool
	typos  int
}

// NewMatcher reads the words a query looks for in text: its terms and the
// words of its text filters, but those it says must not match. Close takes
// words a typo away for the words that may, as a search that found too
// few did.
func NewMatcher(q Query, close bool) *Matcher {
	m := &Matcher{}
	issued := q.Issued()
	var visit func(n Node)
	visit = func(n Node) {
		switch n := n.(type) {
		case *Term:
			m.add(n, close)
		case *Filter:
			if n.Key == nil || issued[n.Span] {
				return
			}
			switch n.Key.Takes {
			case TakesText, TakesOptionalText:
				if n.Key.Name == "sheet" {
					return
				}
				for _, w := range strings.Fields(n.Value) {
					m.add(&Term{Text: w}, close)
				}
			}
		case *Group:
			visit(n.Part)
		case *All:
			for _, p := range n.Parts {
				visit(p)
			}
		case *Any:
			for _, p := range n.Parts {
				visit(p)
			}
		}
	}
	visit(q.Root)
	return m
}

func (m *Matcher) add(t *Term, close bool) {
	tokens := Tokens(t.Text, Queried)
	if len(tokens) == 0 {
		return
	}
	first := tokens[0].Pos
	p := pattern{slots: make([][]string, tokens[len(tokens)-1].Pos-first+1)}
	for _, tk := range tokens {
		p.slots[tk.Pos-first] = append(p.slots[tk.Pos-first], tk.Term)
	}
	if len(tokens) == 1 && !t.Phrase {
		term := tokens[0].Term
		runes := utf8.RuneCountInString(term)
		lone := !Unspaced([]rune(term)[0])
		p.prefix = t.Open && t.Match != Exact && (runes >= 2 || !lone)
		if lone && t.Match != Exact && runes >= 4 && (t.Match == Fuzzy || close) {
			p.typos = 1
			if runes >= 8 {
				p.typos = 2
			}
		}
	}
	m.patterns = append(m.patterns, p)
}

// Empty reports a matcher with nothing to find.
func (m *Matcher) Empty() bool { return len(m.patterns) == 0 }

// Find is the byte ranges of text the matcher's words match, in order,
// those that overlap joined.
func (m *Matcher) Find(text string) []Span {
	if len(m.patterns) == 0 {
		return nil
	}
	tokens := Tokens(text, Indexed)
	at := map[int][]Token{}
	for _, tk := range tokens {
		at[tk.Pos] = append(at[tk.Pos], tk)
	}
	var found []Span
	for _, p := range m.patterns {
		for _, start := range tokens {
			if s, ok := p.matchAt(start, at); ok {
				found = append(found, s)
			}
		}
	}
	slices.SortFunc(found, func(a, b Span) int { return a.From - b.From })
	var out []Span
	for _, s := range found {
		if n := len(out); n > 0 && s.From <= out[n-1].To {
			out[n-1].To = max(out[n-1].To, s.To)
			continue
		}
		out = append(out, s)
	}
	return out
}

// matchAt matches the pattern with its first slot at start.
func (p pattern) matchAt(start Token, at map[int][]Token) (Span, bool) {
	last := len(p.slots) - 1
	if !p.takes(p.slots[0], start.Term, last == 0) {
		return Span{}, false
	}
	span := Span{start.Start, start.End}
	for i := 1; i <= last; i++ {
		if p.slots[i][0] == "" {
			continue
		}
		var hit *Token
		for j, tk := range at[start.Pos+i] {
			if p.takes(p.slots[i], tk.Term, i == last) && (hit == nil || tk.End < hit.End) {
				hit = &at[start.Pos+i][j]
			}
		}
		if hit == nil {
			return Span{}, false
		}
		span.To = max(span.To, hit.End)
	}
	return span, true
}

// takes reports whether a token stands in a slot.
func (p pattern) takes(slot []string, token string, last bool) bool {
	for _, term := range slot {
		switch {
		case term == token:
			return true
		case last && p.prefix && strings.HasPrefix(token, term):
			return true
		case p.typos > 0 && distance(term, token, p.typos) <= p.typos:
			return true
		}
	}
	return false
}

// distance is the optimal string alignment distance of a and b (edits,
// a swap of neighbours counting one), counted up to limit+1.
func distance(a, b string, limit int) int {
	ra, rb := []rune(a), []rune(b)
	if d := len(ra) - len(rb); d > limit || -d > limit {
		return limit + 1
	}
	prev2 := make([]int, len(rb)+1)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		low := cur[0]
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				cur[j] = min(cur[j], prev2[j-2]+1)
			}
			low = min(low, cur[j])
		}
		if low > limit {
			return limit + 1
		}
		prev2, prev, cur = prev, cur, prev2
	}
	return prev[len(rb)]
}

// Snippet is a piece of text around its first match, about width
// characters long, and the matches it shows, placed in it. It is cut at
// spaces where it can; an ellipsis marks text left out.
func Snippet(text string, matches []Span, width int) (string, []Span) {
	text = strings.TrimSpace(text)
	if utf8.RuneCountInString(text) <= width || len(matches) == 0 {
		return clip(text, matches, 0, len(text))
	}
	// Start a quarter of the width before the first match.
	from := matches[0].From
	for n := 0; from > 0 && n < width/4; n++ {
		_, size := utf8.DecodeLastRuneInString(text[:from])
		from -= size
	}
	if sp := strings.IndexByte(text[from:matches[0].From], ' '); sp >= 0 && from > 0 {
		from += sp + 1
	}
	to := from
	for n := 0; to < len(text) && n < width; n++ {
		_, size := utf8.DecodeRuneInString(text[to:])
		to += size
	}
	if to < len(text) {
		if sp := strings.LastIndexByte(text[from:to], ' '); sp > (to-from)/2 {
			to = from + sp
		}
	}
	return clip(text, matches, from, to)
}

// clip cuts text to [from, to) and places the matches in it.
func clip(text string, matches []Span, from, to int) (string, []Span) {
	out := text[from:to]
	shift := -from
	if from > 0 {
		out = "…" + out
		shift += len("…")
	}
	if to < len(text) {
		out += "…"
	}
	var spans []Span
	for _, m := range matches {
		if m.To <= from || m.From >= to {
			continue
		}
		spans = append(spans, Span{max(m.From, from) + shift, min(m.To, to) + shift})
	}
	return out, spans
}

// Line is the line of text that holds the first of matches (Find), cut to
// about width characters around it, with the matches it shows; with no
// match, the text's first line.
func Line(text string, matches []Span, width int) (string, []Span) {
	at := 0
	if len(matches) > 0 {
		at = matches[0].From
	}
	from := strings.LastIndexByte(text[:at], '\n') + 1
	to := len(text)
	if i := strings.IndexByte(text[at:], '\n'); i >= 0 {
		to = at + i
	}
	var in []Span
	for _, m := range matches {
		if m.From >= from && m.To <= to {
			in = append(in, Span{m.From - from, m.To - from})
		}
	}
	return Snippet(text[from:to], in, width)
}
