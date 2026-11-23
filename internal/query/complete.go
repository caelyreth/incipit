package query

import (
	"strings"
	"unicode/utf8"
)

// Suggestion is one way to go on with a query: Text takes the place of
// the completion's span. Label is what a list shows of it, Hint what a
// key takes after it, and Help what it finds. Of is the key a value is
// for, empty for a key. Done is a suggestion that finishes its part, as a
// value does: what is written next starts another. Written is a value as
// it is written, none of those the key offers.
type Suggestion struct {
	Label, Text, Hint, Help string
	Of                      string
	Done, Written           bool
}

// Completion is what may stand where the caret is in a query: Items, in
// order, each in place of Span.
type Completion struct {
	Span  Span
	Items []Suggestion
}

// DatePresets are the named dates the date keys take, as completion
// offers them.
var DatePresets = []string{"today", "yesterday", "this-week", "last-week", "this-month", "last-month", "this-year", "last-year"}

// CountPresets are the lengths $words offers: short, middling, long and
// longer.
var CountPresets = []string{"<300", "300..1000", "1000..3000", ">=3000"}

// choices are what a choice key offers: its Choices, and for $sort each
// order turned as well, as -updated.
func (k *Key) choices() []string {
	if k.Name != "sort" {
		return k.Choices
	}
	var out []string
	for _, c := range k.Choices {
		out = append(out, c)
		if c != "relevance" {
			out = append(out, "-"+c)
		}
	}
	return out
}

// Complete is what may stand at caret, a byte offset in text, for a query
// run where scope says: the keys a $ being written may name, or the
// values its key takes. names is a name key's values in the notebook, as
// the tags or the groups, most used first.
func Complete(text string, caret int, scope Scope, names func(key string) []string) Completion {
	var at *Filter
	// A key that stands alone says all it says once.
	written := map[string]bool{}
	Walk(Parse(text).Root, func(n Node) {
		if f, ok := n.(*Filter); ok {
			if f.From < caret && caret <= f.To {
				at = f
			} else if f.Key != nil && f.Key.Takes == TakesNothing {
				written[f.Key.Name] = true
			}
		}
	})
	if at == nil {
		return Completion{}
	}
	if at.HasValue && caret >= at.ValueSpan.From {
		return completeValue(text, caret, at, names)
	}
	nameEnd := at.From + 1 + len(at.Name)
	if caret > nameEnd {
		return Completion{}
	}
	typed := text[at.From:nameEnd]
	partial := Fold(text[at.From+1 : caret])
	c := Completion{Span: Span{at.From, nameEnd}}
	for _, k := range Keys {
		if k.Scope&scope == 0 || written[k.Name] || !strings.HasPrefix(k.Name, partial) {
			continue
		}
		s := Suggestion{Label: "$" + k.Name, Text: "$" + k.Name, Hint: k.Hint(), Help: k.Help()}
		switch {
		case k.Bare():
			s.Done = true
		case !at.HasValue:
			s.Text += ":"
		}
		if s.Text == typed && (s.Done || at.HasValue) {
			continue
		}
		c.Items = append(c.Items, s)
	}
	return c
}

// completeValue is the values the filter's key takes that start what is
// written of its value up to the caret, then those that hold it.
func completeValue(text string, caret int, at *Filter, names func(key string) []string) Completion {
	k := at.Key
	if k == nil {
		return Completion{}
	}
	var values []string
	switch k.Takes {
	case TakesChoice:
		values = k.choices()
	case TakesDate:
		values = DatePresets
	case TakesNumber:
		values = CountPresets
	case TakesName:
		if names != nil {
			values = names(k.Name)
		}
	default:
		return Completion{}
	}
	partial := Fold(strings.TrimPrefix(text[at.ValueSpan.From:caret], `"`))
	var starts, holds []string
	for _, v := range values {
		folded := Fold(v)
		switch {
		case folded == Fold(at.Value) && caret == at.ValueSpan.To:
			// What is written already.
		case strings.HasPrefix(folded, partial):
			starts = append(starts, v)
		case utf8.RuneCountInString(partial) > 1 && strings.Contains(folded, partial):
			// One letter is in too much to say anything.
			holds = append(holds, v)
		}
	}
	c := Completion{Span: at.ValueSpan}
	if at.Hash {
		// A # tag is written again whole: as #name, or as the key's where
		// a # cannot write it.
		c.Span = at.Span
	}
	for _, v := range append(starts, holds...) {
		s := Suggestion{Label: v, Text: value(v), Of: k.Name, Done: true}
		if at.Hash {
			s.Label = "#" + v
			if s.Text == v {
				s.Text = "#" + v
			} else {
				s.Text = "$tag:" + s.Text
			}
		}
		c.Items = append(c.Items, s)
	}
	return c
}
