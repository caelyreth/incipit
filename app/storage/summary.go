package storage

import (
	"cmp"
	"slices"
	"strings"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/internal/wordseg"
)

// excerptRunes is how long an excerpt runs.
const excerptRunes = 120

// Summarize reads a page's summary in one walk of its text blocks: its
// first heading's text as its title, the first text after that as its
// excerpt, its words, and the tags its text writes (profile.TagsIn), most
// written first.
//
// TODO(perf): it reads the whole page after each change; as a summary
// analyzer memoised per node pointer (§11.5) every part of it would cost
// O(changed path).
func Summarize(root *doc.Node) Summary {
	var s Summary
	titled := false
	var tags tagCounts
	var walk func(n *doc.Node)
	walk = func(n *doc.Node) {
		for i, slot := range n.Slots() {
			if !slot.IsInline() {
				for _, c := range slot.Children() {
					walk(c)
				}
				continue
			}
			if spec, ok := profile.Schema().Kind(n.Kind()); ok && !spec.Slots[i].Literal && spec.Slots[i].Marks != nil {
				for _, tag := range profile.TagsIn(slot.Content(), 0, slot.Len()) {
					tags.add(tag.Name)
				}
			}
			text := strings.Join(strings.Fields(slot.Content().Text()), " ")
			for seg := range wordseg.Segments(text) {
				if seg.WordLike {
					s.Words++
				}
			}
			switch {
			case text == "" || i > 0:
			case !titled && n.Kind() == profile.Heading:
				s.Title, titled = text, true
			case s.Excerpt == "":
				if r := []rune(text); len(r) > excerptRunes {
					text = string(r[:excerptRunes])
				}
				s.Excerpt = text
			}
		}
	}
	walk(root)
	s.Tags = tags.ranked()
	return s
}

// tagCounts counts a page's tags by their identity, each named as first
// written.
type tagCounts struct {
	tags []Tag
	at   map[string]int
}

func (t *tagCounts) add(name string) {
	key := profile.TagKey(name)
	if i, ok := t.at[key]; ok {
		t.tags[i].Count++
		return
	}
	if t.at == nil {
		t.at = map[string]int{}
	}
	t.at[key] = len(t.tags)
	t.tags = append(t.tags, Tag{Name: name, Count: 1})
}

// ranked is the tags most written first, those written as often in the
// order they are first written.
func (t *tagCounts) ranked() []Tag {
	slices.SortStableFunc(t.tags, func(a, b Tag) int { return cmp.Compare(b.Count, a.Count) })
	return t.tags
}
