package query

import "testing"

func TestConditionsAreTheFiltersAtTheTop(t *testing.T) {
	text := `budget #work -$has:image ($tag:a | $tag:b) $words:<100 $nope "a phrase"`
	got := Conditions(Parse(text))
	want := []string{"#work", "-$has:image", "$words:<100"}
	if len(got) != len(want) {
		t.Fatalf("conditions %+v", got)
	}
	for i, c := range got {
		if text[c.Span.From:c.Span.To] != want[i] || c.String() != want[i] {
			t.Errorf("condition %d spans %q and writes %q, want %q", i, text[c.Span.From:c.Span.To], c, want[i])
		}
	}
	if !got[1].Not || got[1].Key.Name != "has" || got[1].Value != "image" || !got[0].Hash {
		t.Fatalf("read %+v", got)
	}
	if one := Conditions(Parse("$inbox")); len(one) != 1 || one[0].Key.Name != "inbox" {
		t.Fatalf("a lone filter is %+v", one)
	}
	if none := Conditions(Parse("$tag:a | $tag:b")); len(none) != 0 {
		t.Fatalf("parts of an | are conditions: %+v", none)
	}
}

func TestAConditionWritesItself(t *testing.T) {
	tag, _ := KeyNamed("tag")
	title, _ := KeyNamed("title")
	sort, _ := KeyNamed("sort")
	trash, _ := KeyNamed("trash")
	for _, tc := range []struct {
		c    Condition
		want string
	}{
		{Condition{Key: tag, Value: "work", Hash: true}, "#work"},
		{Condition{Key: tag, Value: "two words", Hash: true}, `$tag:"two words"`},
		{Condition{Key: tag, Value: "work", Not: true}, "-$tag:work"},
		{Condition{Key: title}, "$title:"},
		{Condition{Key: title, Value: "weekly notes"}, `$title:"weekly notes"`},
		{Condition{Key: sort, Value: "updated", Not: true}, "$sort:updated"},
		{Condition{Key: trash}, "$trash"},
	} {
		if got := tc.c.String(); got != tc.want {
			t.Errorf("%+v writes %q, want %q", tc.c, got, tc.want)
		}
	}
}

func TestEditsKeepWhatTheyDoNotTouch(t *testing.T) {
	text := `budget  #work  $words:<100 "a phrase"`
	cs := Conditions(Parse(text))
	if got := Replace(text, cs[0].Span, "-#work"); got != `budget  -#work  $words:<100 "a phrase"` {
		t.Fatalf("replace gave %q", got)
	}
	if got := Cut(text, cs[0].Span); got != `budget $words:<100 "a phrase"` {
		t.Fatalf("cut gave %q", got)
	}
	if got := Cut("#work", Conditions(Parse("#work"))[0].Span); got != "" {
		t.Fatalf("cutting all gave %q", got)
	}
}

func TestAndAddsAConditionOfTheWholeQuery(t *testing.T) {
	for text, want := range map[string]string{
		"":                 "$inbox",
		"budget ":          "budget $inbox",
		"a | b":            "(a | b) $inbox",
		`budget "open`:     `budget "open" $inbox`,
		"(a b":             "(a b) $inbox",
		"a |":              "a $inbox",
		"-":                "- $inbox",
		"#work $words:500": "#work $words:500 $inbox",
	} {
		got := And(text, "$inbox")
		if got != want {
			t.Errorf("And(%q) = %q, want %q", text, got, want)
		}
		if cs := Conditions(Parse(got)); len(cs) == 0 || cs[len(cs)-1].Key.Name != "inbox" {
			t.Errorf("And(%q) = %q: the condition is not the query's", text, got)
		}
	}
}

func TestASettingCannotBeTurned(t *testing.T) {
	for _, text := range []string{"-$sort:title", "-$trash", "-$sheet"} {
		if kinds(Parse(text))[text[1:]] != Invalid {
			t.Errorf("%s is not invalid: %+v", text, Parse(text).Issues)
		}
	}
	if q := Parse("-$tag:a"); len(q.Issues) != 0 {
		t.Fatalf("a condition turned is an issue: %+v", q.Issues)
	}
}
