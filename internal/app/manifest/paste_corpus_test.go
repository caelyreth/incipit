package manifest

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/incipit/internal/app/markdown"
	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/engine"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

func structuralForm(snap *doc.Snapshot) string {
	var b strings.Builder
	var walkNode func(*doc.Node)
	walkNode = func(n *doc.Node) {
		b.WriteString(profile.Schema().KindName(n.Kind()))
		switch n.Kind() {
		case profile.Heading:
			fmt.Fprintf(&b, "[%d]", profile.HeadingLevel(n))
		case profile.List:
			fmt.Fprintf(&b, "[%d,%d]", profile.ListStyleOf(n), profile.ListStart(n))
		case profile.Item:
			fmt.Fprintf(&b, "[%d]", profile.TaskOf(n))
		case profile.Code:
			fmt.Fprintf(&b, "[%s]", profile.Schema().Attr(n, profile.FieldInfo).Text())
		case profile.Math, profile.Formula:
			fmt.Fprintf(&b, "[%d]", profile.MathDialectOf(n))
		case profile.Table:
			shape := profile.TableShapeOf(n)
			fmt.Fprintf(&b, "[hdr=%t", shape.Header)
			for _, col := range shape.Columns {
				fmt.Fprintf(&b, ",%d", col.Align)
			}
			b.WriteByte(']')
		}
		b.WriteByte('(')
		for i, sl := range n.Slots() {
			if i > 0 {
				b.WriteByte('|')
			}
			if sl.IsInline() {
				sl.Content().Each(func(_ int, it doc.Item) bool {
					if it.Node != nil {
						walkNode(it.Node)
						return true
					}
					if it.Marks != nil && !it.Marks.Empty() {
						b.WriteByte('<')
						for k, m := range it.Marks.Marks() {
							if k > 0 {
								b.WriteByte(',')
							}
							fmt.Fprintf(&b, "%d", m.Kind)
						}
						b.WriteByte(':')
						b.WriteString(it.Text)
						b.WriteByte('>')
					} else {
						b.WriteString(it.Text)
					}
					return true
				})
				continue
			}
			for j, ch := range sl.Children() {
				if j > 0 {
					b.WriteByte(' ')
				}
				walkNode(ch)
			}
		}
		b.WriteByte(')')
	}
	walkNode(snap.Root())
	return b.String()
}

// Invariant 7: Markdown typed through the editor rules produces the same
// document structure as pasting the Markdown through the reader.
func TestTypedMarkdownEqualsPasted(t *testing.T) {
	prog, err := Compile(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	type step struct {
		text   string
		intent vocab.Intent
	}
	cases := []struct {
		name  string
		typed []step
		md    string
	}{
		{
			name:  "heading levels",
			typed: []step{{text: "## Section title"}},
			md:    "## Section title",
		},
		{
			name:  "inline marks and formula",
			typed: []step{{text: "Say *em* and **bold** and `code` and ~strike~ and $x^2$."}},
			md:    "Say *em* and **bold** and `code` and ~strike~ and $x^2$.",
		},
		{
			name:  "underscore marks",
			typed: []step{{text: "Say _em_ and __strong__."}},
			md:    "Say _em_ and __strong__.",
		},
		{
			name:  "blockquote",
			typed: []step{{text: "> quoted line"}},
			md:    "> quoted line",
		},
		{
			name:  "bullet list",
			typed: []step{{text: "- first"}, {intent: vocab.Break{}}, {text: "second"}},
			md:    "- first\n- second",
		},
		{
			name:  "ordered list with start",
			typed: []step{{text: "3. third"}, {intent: vocab.Break{}}, {text: "fourth"}},
			md:    "3. third\n4. fourth",
		},
		{
			name:  "task list open and done",
			typed: []step{{text: "- [ ] todo"}, {intent: vocab.Break{}}, {text: "[x] done"}},
			md:    "- [ ] todo\n- [x] done",
		},
		{
			name:  "fenced code block",
			typed: []step{{text: "```go"}, {intent: vocab.Break{}}, {text: "fmt.Println()"}},
			md:    "```go\nfmt.Println()\n```",
		},
		{
			name:  "display math block",
			typed: []step{{text: "$$E = mc^2"}},
			md:    "$$\nE = mc^2\n$$",
		},
		{
			name:  "divider",
			typed: []step{{text: "---"}, {intent: vocab.Break{}}, {text: "next"}},
			md:    "---\n\nnext",
		},
		{
			name:  "table row",
			typed: []step{{text: "| Col1 | Col2 |"}, {intent: vocab.Break{}}, {text: "a"}, {intent: vocab.Leave{}}, {text: "b"}},
			md:    "| Col1 | Col2 |\n| --- | --- |\n| a | b |",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Unix(100, 0)
			tick := func() time.Time {
				now = now.Add(time.Second)
				return now
			}

			// 1. Typed through the engine.
			typedSnap, err := profile.EmptyPage(1, 11, doc.NewInterner())
			if err != nil {
				t.Fatal(err)
			}
			te := engine.New(typedSnap, prog, engine.Options{})
			te.OpenView(1)
			for _, st := range tc.typed {
				if st.intent != nil {
					if _, err := te.Do(1, vocab.Input{Intent: st.intent, At: tick()}); err != nil {
						t.Fatalf("typed intent %T failed: %v", st.intent, err)
					}
					continue
				}
				for _, r := range st.text {
					if _, err := te.Do(1, vocab.Input{Intent: vocab.Type{Text: string(r)}, At: tick()}); err != nil {
						t.Fatalf("typed rune %q failed: %v", r, err)
					}
				}
			}
			if err := te.Snapshot().Validate(); err != nil {
				t.Fatalf("typed snapshot invalid: %v", err)
			}

			// 2. Pasted through markdown.Parse -> vocab.Paste.
			slice, ok := markdown.Parse(tc.md)
			if !ok {
				t.Fatalf("markdown.Parse(%q) returned formatted=false", tc.md)
			}
			pastedSnap, err := profile.EmptyPage(1, 11, doc.NewInterner())
			if err != nil {
				t.Fatal(err)
			}
			pe := engine.New(pastedSnap, prog, engine.Options{})
			pe.OpenView(1)
			if _, err := pe.Do(1, vocab.Input{Intent: vocab.Paste{Slice: slice, Text: tc.md}, At: tick()}); err != nil {
				t.Fatalf("pasted failed: %v", err)
			}
			if err := pe.Snapshot().Validate(); err != nil {
				t.Fatalf("pasted snapshot invalid: %v", err)
			}

			gotTyped := structuralForm(te.Snapshot())
			gotPasted := structuralForm(pe.Snapshot())
			if gotTyped != gotPasted {
				t.Fatalf("typed != pasted:\n  typed:  %s\n  pasted: %s", gotTyped, gotPasted)
			}
		})
	}
}
