package query

import (
	"slices"
	"testing"
)

func names(key string) []string {
	switch key {
	case "tag":
		return []string{"work", "weekly notes", "homework"}
	case "in":
		return []string{"Projects", "Journal"}
	}
	return nil
}

func texts(c Completion) []string {
	var out []string
	for _, s := range c.Items {
		out = append(out, s.Text)
	}
	return out
}

// complete completes text at its | and writes the first suggestion in.
func complete(text string, scope Scope) (Completion, string) {
	caret := len(text)
	for i := range text {
		if text[i] == '|' {
			caret = i
			text = text[:i] + text[i+1:]
			break
		}
	}
	c := Complete(text, caret, scope, names)
	if len(c.Items) == 0 {
		return c, text
	}
	return c, text[:c.Span.From] + c.Items[0].Text + text[c.Span.To:]
}

func TestADollarOffersTheKeysItStarts(t *testing.T) {
	c, got := complete("budget $ta", InNotebook)
	if !slices.Equal(texts(c), []string{"$tag:"}) || got != "budget $tag:" {
		t.Fatalf("%v → %q", texts(c), got)
	}
	c, _ = complete("$", InNotebook)
	if len(c.Items) != len(Keys)-2 {
		t.Fatalf("a $ alone offers the notebook's keys: %v", texts(c))
	}
	c, _ = complete("$", InSheet)
	for _, s := range c.Items {
		if k, _ := KeyNamed(s.Label[1:]); k.Scope&InSheet == 0 {
			t.Fatalf("a sheet's find offers %s", s.Label)
		}
	}
	if c, _ = complete("$inbox", InNotebook); len(c.Items) != 0 {
		t.Fatalf("a key written whole is no suggestion: %v", texts(c))
	}
	if c, got = complete("$up|:7d", InNotebook); got != "$updated:7d" {
		t.Fatalf("a key's name before its value: %v → %q", texts(c), got)
	}
	if c, _ = complete("budget", InNotebook); len(c.Items) != 0 {
		t.Fatalf("words are no keys: %v", texts(c))
	}
}

func TestAValueOffersWhatItsKeyTakes(t *testing.T) {
	for _, c := range []struct{ text, want, first string }{
		{"$tag:wo", "work homework", "$tag:work"},
		{"$tag:we", `"weekly notes"`, `$tag:"weekly notes"`},
		{"$tag:", `work "weekly notes" homework`, "$tag:work"},
		{"#we", `$tag:"weekly notes"`, `$tag:"weekly notes"`},
		{"#wo budget", "", ""},
		{"#wo| budget", "#work #homework", "#work budget"},
		{"$in:pro", "Projects", "$in:Projects"},
		{"$has:t", "table", "$has:table"},
		{"$updated:this", "this-week this-month this-year", "$updated:this-week"},
		{"$sort:", "relevance updated created title words", "$sort:relevance"},
		{"$words:5", "", ""},
		{"$tag:work", "homework", "$tag:homework"},
	} {
		got, written := complete(c.text, InNotebook)
		if s := joined(texts(got)); s != c.want || c.want != "" && written != c.first {
			t.Errorf("%q: %q → %q, want %q → %q", c.text, s, written, c.want, c.first)
		}
	}
}

func joined(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += " "
		}
		out += v
	}
	return out
}

func TestALineShowsTheMatchWhereItStands(t *testing.T) {
	text := "A title\nthe second line\nthe budget is here, and more budget"
	m := NewMatcher(Parse("budget"), false)
	line, spans := Line(text, m.Find(text), 80)
	if got := marked(line, spans); got != "the [budget] is here, and more [budget]" {
		t.Fatalf("%q", got)
	}
	if line, spans = Line(text, nil, 80); line != "A title" || len(spans) != 0 {
		t.Fatalf("no match: %q %v", line, spans)
	}
}
