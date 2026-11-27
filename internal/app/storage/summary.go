package storage

import (
	"cmp"
	"slices"
	"strings"

	"github.com/caelyreth/incipit/internal/app/library"
	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

// excerptRunes is how long an excerpt runs.
const excerptRunes = 120

// Summarize reads a page's summary: its first heading's text as its
// title, the first other text as its excerpt, its words, the tags its
// text writes (profile.TagsIn), most written first, its tasks, and the
// picture of its first figure that has one.
func Summarize(root *doc.Node) Summary {
	var z Summarizer
	return z.Summarize(root)
}

// Summarizer reads the summaries of one page as it changes. Each node's
// share is kept by its pointer and the shares of a node's slots are
// combined in order, so after a change only its node and the path above
// it are read again.
type Summarizer struct {
	memo map[*doc.Node]*share
	// full is how many shares the last whole reading kept: past twice as
	// many, the shares no page holds any more are let go.
	full int
}

// Summarize is the summary of the page at root.
func (z *Summarizer) Summarize(root *doc.Node) Summary {
	if z.memo == nil || len(z.memo) > 2*z.full+256 {
		z.memo = map[*doc.Node]*share{}
		defer func() { z.full = len(z.memo) }()
	}
	sh := z.share(root)
	s := Summary{Title: sh.heading, Excerpt: sh.other, Words: sh.words, Tasks: sh.tasks, Picture: sh.picture}
	if r := []rune(s.Excerpt); len(r) > excerptRunes {
		s.Excerpt = string(r[:excerptRunes])
	}
	tags := tagCounts{tags: slices.Clone(sh.tags.tags)}
	s.Tags = tags.ranked()
	return s
}

// share is what a subtree adds to a summary: its words, tags and tasks,
// its first figure's picture, its first heading's text, its first text
// and its first text but that heading, each the text of a node's first
// slot, spaces folded.
type share struct {
	words          int
	tasks          library.Tasks
	tags           tagCounts
	picture        string
	titled         bool
	heading, first string
	other          string
}

func (z *Summarizer) share(n *doc.Node) *share {
	if sh, ok := z.memo[n]; ok {
		return sh
	}
	sh := &share{}
	switch n.Kind() {
	case profile.Item:
		switch profile.TaskOf(n) {
		case profile.TaskOpen:
			sh.tasks.Open++
		case profile.TaskDone:
			sh.tasks.Done++
		}
	case profile.Figure:
		sh.picture = profile.ShapeOf(n).Media
	}
	for i, slot := range n.Slots() {
		if !slot.IsInline() {
			for _, c := range slot.Children() {
				sh.then(z.share(c))
			}
			continue
		}
		sh.then(slotShare(n, i, slot))
	}
	z.memo[n] = sh
	return sh
}

// slotShare is what one inline slot of n adds.
func slotShare(n *doc.Node, i int, slot doc.Slot) *share {
	sh := &share{}
	if spec, ok := profile.Schema().Kind(n.Kind()); ok && !spec.Slots[i].Literal && spec.Slots[i].Marks != nil {
		for _, tag := range profile.TagsIn(slot.Content(), 0, slot.Len()) {
			sh.tags.add(tag.Name)
		}
	}
	text := strings.Join(strings.Fields(slot.Content().Text()), " ")
	sh.words = vocab.CountWords(text)
	if text != "" && i == 0 {
		sh.first = text
		if n.Kind() == profile.Heading {
			sh.titled, sh.heading = true, text
		} else {
			sh.other = text
		}
	}
	return sh
}

// then adds next, which follows what sh holds.
func (sh *share) then(next *share) {
	sh.words += next.words
	sh.tasks.Open += next.tasks.Open
	sh.tasks.Done += next.tasks.Done
	if sh.picture == "" {
		sh.picture = next.picture
	}
	for _, t := range next.tags.tags {
		sh.tags.addN(t.Name, t.Count)
	}
	switch {
	case sh.titled:
		// The title is ours: next's texts are all other.
		if sh.other == "" {
			sh.other = next.first
		}
	case sh.first == "":
		sh.titled, sh.heading, sh.first, sh.other = next.titled, next.heading, next.first, next.other
	default:
		// Our first text is other, and next's first heading, if any, the
		// title.
		sh.titled, sh.heading = next.titled, next.heading
	}
	if sh.first == "" {
		sh.first = next.first
	}
}

// tagCounts counts a page's tags by their identity, each named as first
// written.
type tagCounts struct {
	tags []Tag
	at   map[string]int
}

func (t *tagCounts) add(name string) { t.addN(name, 1) }

// addN counts a tag n times more.
func (t *tagCounts) addN(name string, n int) {
	key := profile.TagKey(name)
	if i, ok := t.at[key]; ok {
		t.tags[i].Count += n
		return
	}
	if t.at == nil {
		t.at = map[string]int{}
	}
	t.at[key] = len(t.tags)
	t.tags = append(t.tags, Tag{Name: name, Count: n})
}

// ranked is the tags most written first, those written as often in the
// order they are first written.
func (t *tagCounts) ranked() []Tag {
	slices.SortStableFunc(t.tags, func(a, b Tag) int { return cmp.Compare(b.Count, a.Count) })
	return t.tags
}
