package markdown

import (
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/engine"
	"github.com/caelyreth/neverland/internal/editor/policies/core"
	"github.com/caelyreth/neverland/internal/editor/policies/lists"
	"github.com/caelyreth/neverland/internal/editor/policies/navigation"
	"github.com/caelyreth/neverland/internal/editor/policies/notation"
	"github.com/caelyreth/neverland/internal/editor/policies/pairs"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

type page struct {
	t   *testing.T
	e   *engine.Engine
	now time.Time
}

func open(t *testing.T) *page {
	t.Helper()
	prog, err := policy.Compile(policy.Manifest{Profile: profile.Profile,
		Enabled: []*policy.Policy{core.Policy, notation.Policy, pairs.Policy, Policy, lists.Policy, navigation.Policy},
		Strict:  true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := profile.EmptyPage(1, 5, doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	e := engine.New(snap, prog, engine.Options{Trace: true})
	e.OpenView(1)
	return &page{t: t, e: e, now: time.Unix(10, 0)}
}

func (p *page) do(in vocab.Intent) {
	p.t.Helper()
	p.now = p.now.Add(time.Second)
	if _, err := p.e.Do(1, vocab.Input{Intent: in, At: p.now}); err != nil {
		p.t.Fatal(err)
	}
}

func (p *page) typeText(s string) {
	p.t.Helper()
	for _, r := range s {
		p.do(vocab.Type{Text: string(r)})
	}
}

func outline(n *doc.Node) string {
	var b strings.Builder
	var walk func(*doc.Node)
	walk = func(n *doc.Node) {
		b.WriteString(profile.Schema().KindName(n.Kind()))
		b.WriteByte('(')
		for i, s := range n.Slots() {
			if i > 0 {
				b.WriteByte('|')
			}
			if s.IsInline() {
				s.Content().Each(func(_ int, it doc.Item) bool {
					if it.Node != nil {
						walk(it.Node)
					} else {
						b.WriteString(it.Text)
					}
					return true
				})
				continue
			}
			for j, c := range s.Children() {
				if j > 0 {
					b.WriteByte(' ')
				}
				walk(c)
			}
		}
		b.WriteByte(')')
	}
	walk(n)
	return b.String()
}

func (p *page) want(s string) {
	p.t.Helper()
	if got := outline(p.e.Snapshot().Root()); got != s {
		p.t.Fatalf("got  %s\nwant %s", got, s)
	}
}

func TestTypedListJoinsTheListBeforeIt(t *testing.T) {
	p := open(t)
	p.typeText("- one")
	p.do(vocab.Break{})
	p.do(vocab.Break{})
	p.typeText("- two")
	p.want("page(list(item(one|) item(two|)))")
}

func TestTypedListJoinsTheListAfterIt(t *testing.T) {
	p := open(t)
	p.typeText("one")
	p.do(vocab.Break{})
	p.typeText("- two")
	p.do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward})
	p.typeText("- ")
	p.want("page(list(item(one|) item(two|)))")
}

func TestTypedListKeepsADifferentStyleApart(t *testing.T) {
	p := open(t)
	p.typeText("- one")
	p.do(vocab.Break{})
	p.do(vocab.Break{})
	p.typeText("1. two")
	p.want("page(list(item(one|)) list(item(two|)))")
}

func TestTypedTaskMarkersBecomeItemState(t *testing.T) {
	for _, tc := range []struct {
		typed string
		want  profile.Task
	}{
		{"- [ ] ", profile.TaskOpen},
		{"- [x] ", profile.TaskDone},
		{"- [X] ", profile.TaskDone},
	} {
		p := open(t)
		p.typeText(tc.typed)
		item := p.e.Snapshot().Root().Slot(0).Child(0).Slot(0).Child(0)
		if item.Kind() != profile.Item || profile.TaskOf(item) != tc.want {
			t.Fatalf("%q made %v with task %v, want %v", tc.typed, item.Kind(), profile.TaskOf(item), tc.want)
		}
		if got := item.Slot(0).Content().Text(); got != "" {
			t.Fatalf("%q left text %q in the item head", tc.typed, got)
		}
	}
}

func TestATaskContinuesAsAnOpenTask(t *testing.T) {
	p := open(t)
	p.typeText("- [x] done")
	p.do(vocab.Break{})
	items := p.e.Snapshot().Root().Slot(0).Child(0).Slot(0).Children()
	if len(items) != 2 {
		t.Fatalf("continuation made %d items, want 2", len(items))
	}
	if got := profile.TaskOf(items[1]); got != profile.TaskOpen {
		t.Fatalf("continuation task %v, want open", got)
	}
}

func TestAnInputMethodCommitOpensAMark(t *testing.T) {
	p := open(t)
	p.typeText("*")
	// A commit from an input method arrives as one Type of its whole text.
	p.do(vocab.Type{Text: "你好"})
	content := p.e.Snapshot().Root().Slot(0).Child(0).Slot(0).Content()
	if got := content.Text(); got != "你好" {
		t.Fatalf("text = %q, want the commit without its delimiter", got)
	}
	if ms := content.MarksAt(1); ms == nil || !ms.Has(profile.Emphasis) {
		t.Fatal("the commit is not emphasised")
	}
}

func TestMidLineInlineMarkupOpensAndCloses(t *testing.T) {
	left := vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward}
	for _, tc := range []struct {
		name      string
		setup     string
		back      int
		typed     string
		wantPlain string
		wantMark  doc.MarkKind
	}{
		{"emphasis before space", "hello  world", 6, "*mid*", "hello mid world", profile.Emphasis},
		{"strong before space", "hello  world", 6, "**mid**", "hello mid world", profile.Strong},
		{"code before space", "hello  world", 6, "`mid`", "hello mid world", profile.CodeMark},
		{"strike before space", "hello  world", 6, "~mid~", "hello mid world", profile.Strike},
		{"emphasis inside parens", "hello  world", 6, "(*mid*", "hello (mid) world", profile.Emphasis},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := open(t)
			p.typeText(tc.setup)
			for range tc.back {
				p.do(left)
			}
			p.typeText(tc.typed)
			slot := p.e.Snapshot().Root().Slot(0).Child(0).Slot(0).Content()
			if got := slot.Text(); got != tc.wantPlain {
				t.Fatalf("text = %q, want %q", got, tc.wantPlain)
			}
			midAt := strings.Index(tc.wantPlain, "mid")
			if ms := slot.MarksAt(midAt + 1); ms == nil || !ms.Has(tc.wantMark) {
				t.Fatalf("marks at mid = %v, want %v", ms, tc.wantMark)
			}
		})
	}
}

func TestMidLineLiteralDelimitersStayLiteral(t *testing.T) {
	left := vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward}
	p := open(t)
	p.typeText("hello world")
	for range len("world") {
		p.do(left)
	}
	p.typeText("*a")
	p.want("page(paragraph(hello *aworld))")

	p = open(t)
	p.typeText("2 * 3 4*")
	p.want("page(paragraph(2 * 3 4*))")
}
