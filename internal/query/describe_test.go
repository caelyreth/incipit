package query

import "testing"

func TestValuesAreSaidInWords(t *testing.T) {
	key := func(name string) *Key { k, _ := KeyNamed(name); return k }
	for _, tc := range []struct {
		key, value, want string
	}{
		{"updated", "7d", "in the last 7 days"},
		{"updated", "<3d", "in the last 3 days"},
		{"updated", "1w", "in the last week"},
		{"updated", ">3d", "more than 3 days ago"},
		{"updated", "this-week", "this week"},
		{"updated", "2026-11-14", "on 2026-11-14"},
		{"updated", "2026-11", "in 2026-11"},
		{"created", ">2026-01-01", "after 2026-01-01"},
		{"created", "<=2026-03", "up to 2026-03"},
		{"created", "2026-10..2026-11", "2026-10 to 2026-11"},
		{"created", "2026-10..", "from 2026-10"},
		{"created", "..yesterday", "up to yesterday"},
		{"words", "500", "500 or more"},
		{"words", "<100", "fewer than 100"},
		{"words", ">2k", "more than 2k"},
		{"words", "<=100", "100 or fewer"},
		{"words", "100..500", "100 to 500"},
		{"sort", "updated", "updated, oldest first"},
		{"sort", "-title", "title, Z to A"},
		{"sort", "relevance", "relevance"},
		{"title", "weekly notes", "“weekly notes”"},
		{"tag", "work", "work"},
		{"words", "lots", "lots"},
	} {
		if got := Describe(key(tc.key), tc.value); got != tc.want {
			t.Errorf("$%s:%s says %q, want %q", tc.key, tc.value, got, tc.want)
		}
	}
}

func TestAConditionIsSaidWhole(t *testing.T) {
	for text, want := range map[string]string{
		"-#work":         "Tag is not work",
		"$updated:7d":    "Updated in the last 7 days",
		"-$words:<100":   "Words not fewer than 100",
		"$title:budget":  "Title has “budget”",
		"$inbox":         "In the Inbox",
		"-$inbox":        "Not in the Inbox",
		"$trash":         "In the trash",
		"$has:":          "Content has …",
		"-$has:image":    "Content has no image",
		"$link:x.com":    "Links to x.com",
		"$sort:-updated": "Sorted by updated, newest first",
	} {
		cs := Conditions(Parse(text))
		if len(cs) != 1 {
			t.Fatalf("%q has %d conditions", text, len(cs))
		}
		if got := cs[0].Sentence(); got != want {
			t.Errorf("%q says %q, want %q", text, got, want)
		}
	}
}
