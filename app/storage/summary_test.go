package storage

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/internal/wordseg"
)

func para(text string) *doc.Node { return profile.NewParagraph(doc.TextItem(text, nil)) }

func page(blocks ...*doc.Node) *doc.Node {
	return doc.NewNode(profile.Page, doc.Attrs{}, doc.BlockSlot(blocks...))
}

func TestASummaryTitlesThePageByItsFirstHeading(t *testing.T) {
	h := doc.NewNode(profile.Heading, profile.Schema().Canonical(profile.Heading, doc.Attrs{}), doc.InlineSlot(doc.TextItem("  A   title ", nil)))
	item := doc.NewNode(profile.Item, doc.Attrs{}, doc.InlineSlot(doc.TextItem("x", nil)), doc.BlockSlot(h))
	list := doc.NewNode(profile.List, doc.Attrs{}, doc.BlockSlot(item))
	s := Summarize(page(para("intro words"), list))
	if !s.Equal(Summary{Title: "A title", Excerpt: "intro words", Words: 5}) {
		t.Fatalf("summary %+v", s)
	}
	if s := Summarize(page(para("none"))); s.Title != "" || s.Excerpt != "none" {
		t.Fatalf("summary %+v without a heading", s)
	}
}

// taskAttrs are an item's attributes for task.
func taskAttrs(task profile.Task) doc.Attrs {
	return doc.MakeAttrs(doc.Field{Num: profile.FieldTask, Value: doc.Int(int64(task))})
}

// A summary counts the page's task items, open and done, nested or not,
// and nothing else as one.
func TestASummaryCountsThePagesTasks(t *testing.T) {
	item := func(task profile.Task, inner ...*doc.Node) *doc.Node {
		return doc.NewNode(profile.Item, taskAttrs(task), doc.InlineSlot(doc.TextItem("do", nil)), doc.BlockSlot(inner...))
	}
	list := func(items ...*doc.Node) *doc.Node {
		return doc.NewNode(profile.List, doc.Attrs{}, doc.BlockSlot(items...))
	}
	h := doc.NewNode(profile.Heading, profile.Schema().Canonical(profile.Heading, doc.Attrs{}), doc.InlineSlot(doc.TextItem("Plan", nil)))
	s := Summarize(page(h, list(item(profile.TaskOpen, list(item(profile.TaskDone), item(profile.NoTask))), item(profile.TaskOpen))))
	if s.Tasks.Open != 2 || s.Tasks.Done != 1 {
		t.Fatalf("tasks %+v, want two open and one done", s.Tasks)
	}
}

func TestASummaryCountsTheTagsThePageWrites(t *testing.T) {
	code := doc.NewNode(profile.Code, doc.Attrs{}, doc.InlineSlot(doc.TextItem("#include", nil)))
	s := Summarize(page(para("#Moon tides #ocean"), code, para("#ocean and #moon, #ocean")))
	want := []Tag{{Name: "ocean", Count: 3}, {Name: "Moon", Count: 2}}
	if !slices.Equal(s.Tags, want) {
		t.Fatalf("tags %+v, want %+v", s.Tags, want)
	}
}

// walkSummary is a summary read in one walk of the page, the reading
// Summarizer combines from its nodes' shares.
func walkSummary(root *doc.Node) Summary {
	var s Summary
	titled := false
	var tags tagCounts
	var walk func(n *doc.Node)
	walk = func(n *doc.Node) {
		if n.Kind() == profile.Item {
			switch profile.TaskOf(n) {
			case profile.TaskOpen:
				s.Tasks.Open++
			case profile.TaskDone:
				s.Tasks.Done++
			}
		}
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

// A summary combined from its nodes' shares is the one a walk of the page
// reads, and a summarizer kept across a page's versions, which share the
// nodes they did not change, reads what a fresh one does.
func TestASummaryOfSharesIsTheWalksAcrossVersions(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 11))
	texts := []string{"", "  ", "intro words", "#moon tides", "#Ocean and #moon", "中文 English", strings.Repeat("long ", 40), "x"}
	block := func() *doc.Node {
		text := texts[rng.IntN(len(texts))]
		switch rng.IntN(5) {
		case 0:
			return doc.NewNode(profile.Heading, profile.Schema().Canonical(profile.Heading, doc.Attrs{}), doc.InlineSlot(doc.TextItem(text, nil)))
		case 1:
			return doc.NewNode(profile.Code, doc.Attrs{}, doc.InlineSlot(doc.TextItem("#include "+text, nil)))
		case 2:
			h := doc.NewNode(profile.Heading, profile.Schema().Canonical(profile.Heading, doc.Attrs{}), doc.InlineSlot(doc.TextItem(text, nil)))
			item := doc.NewNode(profile.Item, taskAttrs(profile.Task(rng.IntN(3))), doc.InlineSlot(doc.TextItem(texts[rng.IntN(len(texts))], nil)), doc.BlockSlot(h))
			return doc.NewNode(profile.List, doc.Attrs{}, doc.BlockSlot(item))
		}
		return para(text)
	}
	blocks := make([]*doc.Node, 12)
	for i := range blocks {
		blocks[i] = block()
	}
	var kept Summarizer
	for step := range 400 {
		root := page(slices.Clone(blocks)...)
		want := walkSummary(root)
		if got := Summarize(root); !got.Equal(want) {
			t.Fatalf("step %d: summary %+v, the walk reads %+v", step, got, want)
		}
		if got := kept.Summarize(root); !got.Equal(want) {
			t.Fatalf("step %d: a kept summarizer reads %+v, the walk %+v", step, got, want)
		}
		i := rng.IntN(len(blocks))
		switch rng.IntN(4) {
		case 0:
			blocks = slices.Delete(blocks, i, i+1)
		case 1:
			blocks = slices.Insert(blocks, i, block())
		default:
			blocks[i] = block()
		}
		if len(blocks) == 0 {
			blocks = append(blocks, block())
		}
	}
}
