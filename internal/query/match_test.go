package query

import (
	"slices"
	"strings"
	"testing"
)

func marked(text string, spans []Span) string {
	var b strings.Builder
	at := 0
	for _, s := range spans {
		b.WriteString(text[at:s.From] + "[" + text[s.From:s.To] + "]")
		at = s.To
	}
	return b.String() + text[at:]
}

func TestTheMatcherPlacesWhatTheIndexFinds(t *testing.T) {
	for _, c := range []struct{ query, text, want string }{
		{"budget ", "Budget planning: the budget.", "[Budget] planning: the [budget]."},
		{"cafe ", "at the Café", "at the [Café]"},
		{"数据库", "设计关系数据库的索引", "设计关系[数据库]的索引"},
		{"库", "数据库", "数据[库]"},
		{`"budget review"`, "a budget review, a budget plan", "a [budget review], a budget plan"},
		{"plan", "planning ahead", "[planning] ahead"},
		{"~budgte ", "the budget", "the [budget]"},
		{"budget -plan", "budget plan", "[budget] plan"},
		{"$title:trip $heading:milk", "trip to buy milk", "[trip] to buy [milk]"},
		{"$updated:7d", "nothing to mark", "nothing to mark"},
		{"ラーメン", "ラーメンを食べた", "[ラーメン]を食べた"},
	} {
		m := NewMatcher(Parse(c.query), false)
		if got := marked(c.text, m.Find(c.text)); got != c.want {
			t.Errorf("%q in %q: %q, want %q", c.query, c.text, got, c.want)
		}
	}
}

func TestACloseMatcherTakesTyposOnlyWhenTold(t *testing.T) {
	q := Parse("recieve ")
	if len(NewMatcher(q, false).Find("receive it")) != 0 {
		t.Fatal("a word as written takes no typos")
	}
	if got := NewMatcher(q, true).Find("receive it"); !slices.Equal(got, []Span{{0, 7}}) {
		t.Fatalf("close %v", got)
	}
}

func TestASnippetShowsTheTextAroundItsFirstMatch(t *testing.T) {
	text := strings.Repeat("filler words go here ", 10) + "the budget is set " + strings.Repeat("and more words follow ", 10)
	m := NewMatcher(Parse("budget "), false)
	snip, spans := Snippet(text, m.Find(text), 60)
	if !strings.HasPrefix(snip, "…") || !strings.HasSuffix(snip, "…") || len(spans) != 1 || snip[spans[0].From:spans[0].To] != "budget" {
		t.Fatalf("snippet %q %v", snip, spans)
	}
	short, spans := Snippet("the budget", m.Find("the budget"), 60)
	if short != "the budget" || len(spans) != 1 {
		t.Fatalf("short %q %v", short, spans)
	}
}

func TestASheetsFindKeepsCaseWordsAndKeysWhenAsked(t *testing.T) {
	for _, c := range []struct{ query, text, want string }{
		{"$sheet go", "Go and go, gone", "[Go] and [go], [gone]"},
		{"$sheet $word go", "Go and go, gone", "[Go] and [go], gone"},
		{"$sheet $case Go ", "Go and go, gone", "[Go] and go, gone"},
		{"$sheet $case go", "Go and go, gone", "Go and [go], [gone]"},
		{"$sheet $case $word go", "Go and go, gone", "Go and [go], gone"},
	} {
		m := NewMatcher(Parse(c.query), false)
		if got := marked(c.text, m.Find(c.text)); got != c.want {
			t.Errorf("%q in %q: %q, want %q", c.query, c.text, got, c.want)
		}
	}
	m := NewMatcher(Parse("$sheet plan $heading:trip"), false)
	hits := m.Hits("a trip plan")
	if len(hits) != 2 || hits[0].Key != "heading" || hits[1].Key != "" {
		t.Fatalf("hits %+v", hits)
	}
}
