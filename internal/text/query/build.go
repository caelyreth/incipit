package query

import (
	"strings"
	"unicode"
)

// Condition is a filter a query holds of every sheet it finds: one of the
// parts written one after another at its top, a $ key or a #tag, turned
// with - or not. A filter builder shows a query as its conditions and
// edits it through them, in its text: what it does not show, it leaves as
// written.
type Condition struct {
	// Span is the part in the query's text, its - included.
	Span Span
	Key  *Key
	Not  bool
	// Value is the value as meant, unquoted.
	Value string
	// Hash is a tag written as #name, which a tag's name keeps.
	Hash bool
}

// Conditions are the conditions a query holds at its top, in the order
// written. A key the language does not have is no condition.
func Conditions(q Query) []Condition {
	var parts []Node
	switch root := q.Root.(type) {
	case *All:
		parts = root.Parts
	case nil:
	default:
		parts = []Node{root}
	}
	var out []Condition
	for _, part := range parts {
		c := Condition{Span: part.span()}
		if n, ok := part.(*Not); ok {
			c.Not, part = true, n.Part
		}
		f, ok := part.(*Filter)
		if !ok || f.Key == nil {
			continue
		}
		c.Key, c.Value, c.Hash = f.Key, f.Value, f.Hash
		out = append(out, c)
	}
	return out
}

// String is the condition as the language writes it: a tag as #name
// where it was and its name can be, a value not written yet left to be.
func (c Condition) String() string {
	f := &Filter{Key: c.Key, Name: c.Key.Name, Value: c.Value, HasValue: c.Key.Takes != TakesNothing && !(c.Key.Alone && c.Value == "")}
	f.Hash = c.Hash && c.Key.Name == "tag" && tagName(c.Value)
	var n Node = f
	if c.Not && !c.Key.Setting {
		n = &Not{Part: f}
	}
	return Format(n)
}

// Replace is text with the part at a span in place of what is there.
func Replace(text string, at Span, part string) string {
	return text[:at.From] + part + text[at.To:]
}

// Cut is text without the part at a span, what stood either side of it
// joined by a space.
func Cut(text string, at Span) string {
	left := strings.TrimRightFunc(text[:at.From], unicode.IsSpace)
	right := strings.TrimLeftFunc(text[at.To:], unicode.IsSpace)
	if left != "" && right != "" {
		left += " "
	}
	return left + right
}

// And is text with a condition written after it, holding of what it finds
// as the conditions before it do. Where what text ends in would take the
// part in, as an open quote or parenthesis or a | does, or where the query
// is one of several parts (a | b), the query is written out whole first,
// closed and in parentheses as it needs to be.
func And(text, part string) string {
	head := strings.TrimRightFunc(text, unicode.IsSpace)
	if head == "" {
		return part
	}
	if out := head + " " + part; endsIn(out, part) {
		return out
	}
	root := Parse(head).Root
	if _, ok := root.(*Any); ok {
		root = &Group{Part: root, Closed: true}
	}
	if head = Format(root); head == "" {
		return part
	}
	return head + " " + part
}

// endsIn reports whether text's last condition is part, written last.
func endsIn(text, part string) bool {
	cs := Conditions(Parse(text))
	return len(cs) > 0 && cs[len(cs)-1].Span == Span{len(text) - len(part), len(text)}
}
