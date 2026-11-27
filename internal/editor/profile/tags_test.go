package profile

import (
	"slices"
	"testing"

	"github.com/caelyreth/incipit/internal/editor/doc"
)

func TestTagsAreHashesStartingWords(t *testing.T) {
	cases := []struct {
		text string
		want []string
	}{
		{"#ocean", []string{"ocean"}},
		{"a #b c", []string{"b"}},
		{"# heading", nil},
		{"## two", nil},
		{"C# and #1", nil},
		{"x.com/#anchor", nil},
		{"(#moon) “#sun”", []string{"moon", "sun"}},
		{"#ocean-life-", []string{"ocean-life"}},
		{"#a#b", []string{"a"}},
		{"#海洋 и #море", []string{"海洋", "море"}},
		{"#snake_case2", []string{"snake_case2"}},
	}
	for _, c := range cases {
		var got []string
		in := doc.NewInline(doc.TextItem(c.text, nil))
		for _, tag := range TagsIn(in, 0, in.Len()) {
			if c.text[tag.From:tag.To] != "#"+tag.Name {
				t.Errorf("%q: extent %q for %q", c.text, c.text[tag.From:tag.To], tag.Name)
			}
			got = append(got, tag.Name)
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("%q: tags %q, want %q", c.text, got, c.want)
		}
	}
}
