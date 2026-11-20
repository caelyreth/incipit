package query

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Span is a byte range of a query's text.
type Span struct{ From, To int }

// Node is a part of a parsed query.
type Node interface{ span() Span }

// Match is how a term's words are matched.
type Match uint8

const (
	// Auto matches a word as written, then, when that finds little,
	// words a typo away.
	Auto Match = iota
	// Fuzzy (~word) always takes words a typo away.
	Fuzzy
	// Exact (=word) takes the word only, never a longer one it starts.
	Exact
)

// Term is words to find: one word, or a phrase in quotes.
type Term struct {
	Span
	// Text is the words as written, without quotes or marker.
	Text   string
	Match  Match
	Phrase bool
	// Open is a phrase whose closing quote is not written yet, or a word
	// the text ends in, being typed: it also finds the words it starts.
	Open bool
}

// Filter is a $ key, or a #tag.
type Filter struct {
	Span
	// Key is nil for a name the language does not have.
	Key  *Key
	Name string
	// Value is the value as meant, unquoted; HasValue is whether a colon
	// was written.
	Value     string
	HasValue  bool
	ValueSpan Span
	// Hash is a tag written as #name.
	Hash bool
}

// Not is a part that must not match (-part).
type Not struct {
	Span
	Part Node
}

// All is parts that must all match, written one after another.
type All struct {
	Span
	Parts []Node
}

// Any is parts of which one must match (a | b).
type Any struct {
	Span
	Parts []Node
}

// Group is a part in parentheses.
type Group struct {
	Span
	Part   Node
	Closed bool
}

func (s Span) span() Span { return s }

// IssueKind is what is wrong with a part of a query.
type IssueKind uint8

const (
	// Pending is a part not finished yet: a key's name or value, a quote
	// or a parenthesis still to come. It matches nothing until it is.
	Pending IssueKind = iota + 1
	// Unknown is a key the language does not have.
	Unknown
	// Invalid is a value the key does not take.
	Invalid
	// Inert is a key that means nothing where the query runs, as a date
	// in a sheet's find.
	Inert
)

// Issue is a part of a query that does not count, and why.
type Issue struct {
	Span
	Kind IssueKind
}

// Query is a parsed query: what its text says, and what of it does not
// count. A query never fails to parse: what is not understood is an issue
// and matches as if it were not written.
type Query struct {
	Text   string
	Root   Node
	Issues []Issue
	// Scope is where the query runs: a $sheet key runs it in a sheet.
	Scope Scope
}

// Parse reads a query. It runs in the notebook unless it holds $sheet.
func Parse(text string) Query {
	p := &parser{text: text}
	p.lex()
	q := Query{Text: text, Scope: InNotebook}
	q.Root = p.any(false)
	Walk(q.Root, func(n Node) {
		if f, ok := n.(*Filter); ok && f.Key != nil && f.Key.Name == "sheet" {
			q.Scope = InSheet
		}
	})
	Walk(q.Root, func(n Node) {
		switch n := n.(type) {
		case *Filter:
			if n.Key != nil && n.Key.Scope&q.Scope == 0 {
				p.issue(n.Span, Inert)
			}
		case *Not:
			// A setting holds of the whole query: there is no sheet it
			// does not hold of.
			if f, ok := n.Part.(*Filter); ok && f.Key != nil && f.Key.Setting {
				p.issue(f.Span, Invalid)
			}
		}
	})
	q.Issues = p.issues
	return q
}

// String is the query written out in its canonical form: what a filter
// built from controls writes back.
func (q Query) String() string {
	var b strings.Builder
	write(&b, q.Root)
	return b.String()
}

// Format writes a node as the language spells it.
func Format(n Node) string {
	var b strings.Builder
	write(&b, n)
	return b.String()
}

func write(b *strings.Builder, n Node) {
	switch n := n.(type) {
	case nil:
	case *Term:
		switch n.Match {
		case Fuzzy:
			b.WriteByte('~')
		case Exact:
			b.WriteByte('=')
		}
		if n.Phrase {
			b.WriteString(quote(n.Text))
		} else {
			b.WriteString(n.Text)
		}
	case *Filter:
		if n.Hash {
			b.WriteString("#" + n.Value)
			return
		}
		b.WriteString("$" + n.Name)
		if n.HasValue {
			// A value not written yet is left to be written.
			b.WriteByte(':')
			if n.Value != "" {
				b.WriteString(value(n.Value))
			}
		}
	case *Not:
		b.WriteByte('-')
		write(b, n.Part)
	case *All:
		for i, part := range n.Parts {
			if i > 0 {
				b.WriteByte(' ')
			}
			write(b, part)
		}
	case *Any:
		for i, part := range n.Parts {
			if i > 0 {
				b.WriteString(" | ")
			}
			write(b, part)
		}
	case *Group:
		b.WriteByte('(')
		write(b, n.Part)
		b.WriteByte(')')
	}
}

// Literal is text as a query writes it to find it as it is: bare when it
// is one word a query reads as a word, or else in quotes, as a phrase.
func Literal(s string) string {
	if s != "" && strings.ContainsRune("$#-~", rune(s[0])) {
		return quote(s)
	}
	return value(s)
}

// value is a filter's value as written: bare when it can be.
func value(s string) string {
	if s == "" || strings.ContainsFunc(s, func(r rune) bool { return unicode.IsSpace(r) || strings.ContainsRune(`"()|`, r) }) {
		return quote(s)
	}
	return s
}

func quote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// Walk visits n and every part under it.
func Walk(n Node, visit func(Node)) {
	if n == nil {
		return
	}
	visit(n)
	switch n := n.(type) {
	case *Not:
		Walk(n.Part, visit)
	case *Group:
		Walk(n.Part, visit)
	case *All:
		for _, p := range n.Parts {
			Walk(p, visit)
		}
	case *Any:
		for _, p := range n.Parts {
			Walk(p, visit)
		}
	}
}

// --- lexing ---------------------------------------------------------------

type lexKind uint8

const (
	lexOpen lexKind = iota
	lexClose
	lexOr
	lexNot
	lexAtom
)

type lexeme struct {
	kind lexKind
	Span
	atom Node
}

type parser struct {
	text   string
	lexes  []lexeme
	at     int
	issues []Issue
}

func (p *parser) issue(s Span, k IssueKind) { p.issues = append(p.issues, Issue{Span: s, Kind: k}) }

func (p *parser) lex() {
	s := p.text
	i := 0
	for i < len(s) {
		r, n := utf8.DecodeRuneInString(s[i:])
		switch {
		case unicode.IsSpace(r):
			i += n
		case r == '(':
			p.lexes = append(p.lexes, lexeme{kind: lexOpen, Span: Span{i, i + 1}})
			i++
		case r == ')':
			p.lexes = append(p.lexes, lexeme{kind: lexClose, Span: Span{i, i + 1}})
			i++
		case r == '|':
			p.lexes = append(p.lexes, lexeme{kind: lexOr, Span: Span{i, i + 1}})
			i++
		case r == '-' && i+1 < len(s) && !unicode.IsSpace(rune(s[i+1])) && s[i+1] != ')' && s[i+1] != '|':
			p.lexes = append(p.lexes, lexeme{kind: lexNot, Span: Span{i, i + 1}})
			i++
		default:
			atom, end := p.atom(i)
			p.lexes = append(p.lexes, lexeme{kind: lexAtom, Span: atom.span(), atom: atom})
			i = end
		}
	}
}

// atom reads the term or filter at i and returns where it ends.
func (p *parser) atom(i int) (Node, int) {
	s := p.text
	from := i
	match := Auto
	if s[i] == '~' || s[i] == '=' {
		if s[i] == '~' {
			match = Fuzzy
		} else {
			match = Exact
		}
		i++
	}
	if i < len(s) && s[i] == '"' {
		text, end, closed := quoted(s, i)
		t := &Term{Span: Span{from, end}, Text: text, Match: match, Phrase: true, Open: !closed}
		if !closed {
			p.issue(t.Span, Pending)
		}
		return t, end
	}
	if match == Auto && i < len(s) && s[i] == '$' {
		return p.filter(from)
	}
	end := bare(s, i)
	word := s[i:end]
	if match == Auto && s[from] == '#' && tagName(word[1:]) {
		return &Filter{Span: Span{from, end}, Key: mustKey("tag"), Name: "tag", Value: word[1:], HasValue: true, ValueSpan: Span{from + 1, end}, Hash: true}, end
	}
	t := &Term{Span: Span{from, end}, Text: word, Match: match, Open: end == len(s)}
	if word == "" {
		p.issue(t.Span, Pending)
	}
	return t, end
}

// filter reads $name or $name:value at i.
func (p *parser) filter(i int) (Node, int) {
	s := p.text
	from := i
	i++
	nameEnd := i
	for nameEnd < len(s) {
		r, n := utf8.DecodeRuneInString(s[nameEnd:])
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			break
		}
		nameEnd += n
	}
	f := &Filter{Name: s[i:nameEnd]}
	f.Key, _ = KeyNamed(f.Name)
	end := nameEnd
	if end < len(s) && s[end] == ':' {
		f.HasValue = true
		vfrom := end + 1
		if vfrom < len(s) && s[vfrom] == '"' {
			text, vend, closed := quoted(s, vfrom)
			f.Value, end = text, vend
			if !closed {
				p.issue(Span{vfrom, vend}, Pending)
			}
		} else {
			end = bare(s, vfrom)
			f.Value = s[vfrom:end]
		}
		f.ValueSpan = Span{vfrom, end}
	} else if end < len(s) && !unicode.IsSpace(rune(s[end])) && !strings.ContainsRune("()|", rune(s[end])) {
		// Letters a name cannot hold run on into the filter: it is not a
		// key the language has.
		end = bare(s, end)
		f.Name = s[i:end]
		f.Key = nil
	}
	f.Span = Span{from, end}
	p.check(f)
	return f, end
}

// startsKey reports whether name starts the name of a key.
func startsKey(name string) bool {
	name = Fold(name)
	for _, k := range Keys {
		if strings.HasPrefix(k.Name, name) {
			return true
		}
	}
	return false
}

// check notes what is wrong with a filter's key or value.
func (p *parser) check(f *Filter) {
	k := f.Key
	switch {
	case k == nil && f.To == len(p.text) && !f.HasValue && startsKey(f.Name):
		// A key's name being written at the query's end.
		p.issue(f.Span, Pending)
	case k == nil:
		p.issue(f.Span, Unknown)
	case !f.HasValue && k.Bare():
	case !f.HasValue || f.Value == "":
		if k.Takes != TakesNothing {
			p.issue(f.Span, Pending)
		} else {
			p.issue(f.Span, Invalid)
		}
	case k.Takes == TakesNothing:
		p.issue(f.ValueSpan, Invalid)
	case !k.Accepts(f.Value):
		p.issue(f.ValueSpan, Invalid)
	}
}

// Accepts reports whether the key takes a value.
func (k *Key) Accepts(v string) bool {
	switch k.Takes {
	case TakesDate:
		_, ok := parseDates(v)
		return ok
	case TakesNumber:
		_, ok := ParseCounts(v)
		return ok
	case TakesChoice:
		v = strings.TrimPrefix(Fold(v), "-")
		for _, c := range k.Choices {
			if c == v {
				return true
			}
		}
		return false
	case TakesName:
		if k.Name == "tag" && Fold(v) != "none" {
			return tagName(v)
		}
	}
	return true
}

func mustKey(name string) *Key {
	k, ok := KeyNamed(name)
	if !ok {
		panic("search: no key " + name)
	}
	return k
}

// tagName reports whether s is a tag's whole name, as a tag is written
// in text (profile.TagsIn): a letter, then letters, digits, "_" and "-",
// not ending in "-".
func tagName(s string) bool {
	for i, r := range s {
		if i == 0 && !unicode.IsLetter(r) {
			return false
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' {
			return false
		}
	}
	return s != "" && !strings.HasSuffix(s, "-")
}

// bare is where a word starting at i ends.
func bare(s string, i int) int {
	for i < len(s) {
		r, n := utf8.DecodeRuneInString(s[i:])
		if unicode.IsSpace(r) || strings.ContainsRune(`"()|`, r) {
			break
		}
		i += n
	}
	return i
}

// quoted reads a quoted text at i, its opening quote: its text unescaped,
// where it ends and whether its closing quote is written.
func quoted(s string, i int) (string, int, bool) {
	var b strings.Builder
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			if j+1 < len(s) {
				j++
				b.WriteByte(s[j])
			}
		case '"':
			return b.String(), j + 1, true
		default:
			b.WriteByte(s[j])
		}
	}
	return b.String(), len(s), false
}

// --- parsing --------------------------------------------------------------

func (p *parser) peek() (lexeme, bool) {
	if p.at < len(p.lexes) {
		return p.lexes[p.at], true
	}
	return lexeme{}, false
}

// any reads parts joined by |, up to the end or, inside parentheses, the
// closing one.
func (p *parser) any(inner bool) Node {
	var parts []Node
	for {
		parts = appendPart(parts, p.all(inner))
		l, ok := p.peek()
		if !ok || l.kind != lexOr {
			break
		}
		p.at++
		if next, ok := p.peek(); !ok || next.kind == lexOr || next.kind == lexClose && inner {
			p.issue(l.Span, Pending)
		}
	}
	switch len(parts) {
	case 0:
		return nil
	case 1:
		return parts[0]
	}
	return &Any{Span: Span{parts[0].span().From, parts[len(parts)-1].span().To}, Parts: parts}
}

// all reads parts written one after another.
func (p *parser) all(inner bool) Node {
	var parts []Node
	for {
		l, ok := p.peek()
		if !ok || l.kind == lexOr {
			break
		}
		if l.kind == lexClose {
			if inner {
				break
			}
			// A closing parenthesis with none open counts for nothing.
			p.at++
			p.issue(l.Span, Invalid)
			continue
		}
		parts = appendPart(parts, p.unary(inner))
	}
	switch len(parts) {
	case 0:
		return nil
	case 1:
		return parts[0]
	}
	return &All{Span: Span{parts[0].span().From, parts[len(parts)-1].span().To}, Parts: parts}
}

func (p *parser) unary(inner bool) Node {
	l, _ := p.peek()
	p.at++
	switch l.kind {
	case lexNot:
		next, ok := p.peek()
		if !ok || next.kind == lexOr || next.kind == lexClose {
			p.issue(l.Span, Pending)
			return nil
		}
		part := p.unary(inner)
		if part == nil {
			return nil
		}
		return &Not{Span: Span{l.From, part.span().To}, Part: part}
	case lexOpen:
		part := p.any(true)
		g := &Group{Part: part}
		end := len(p.text)
		if c, ok := p.peek(); ok && c.kind == lexClose {
			p.at++
			g.Closed, end = true, c.To
		}
		g.Span = Span{l.From, end}
		if !g.Closed || part == nil {
			p.issue(g.Span, Pending)
		}
		return g
	}
	return l.atom
}

func appendPart(parts []Node, n Node) []Node {
	if n == nil {
		return parts
	}
	return append(parts, n)
}

// Issued are the spans of the query's parts that do not count.
func (q Query) Issued() map[Span]bool {
	out := map[Span]bool{}
	for _, is := range q.Issues {
		out[is.Span] = true
	}
	return out
}
