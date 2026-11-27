package markdown

import (
	"testing"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

func marksOf(kinds ...doc.MarkKind) *doc.MarkSet {
	in := doc.NewInterner()
	ms := in.Empty()
	for _, k := range kinds {
		ms = ms.With(doc.Mark{Kind: k})
	}
	return ms
}

func TestMarkdownInlineMarksAndObjects(t *testing.T) {
	strong := marksOf(profile.Strong)
	em := marksOf(profile.Emphasis)
	both := marksOf(profile.Strong, profile.Emphasis)
	strike := marksOf(profile.Strike)
	code := marksOf(profile.CodeMark)

	formula := doc.NewNode(profile.Formula, doc.Attrs{}, doc.InlineSlot(doc.TextItem("x^2 + y^2", nil)))

	cases := []struct {
		name  string
		items []doc.Item
		want  string
	}{
		{
			name:  "basic marks",
			items: []doc.Item{{Text: "bold", Marks: strong}, {Text: " "}, {Text: "em", Marks: em}, {Text: " "}, {Text: "del", Marks: strike}, {Text: " "}, {Text: "fn()", Marks: code}},
			want:  "**bold** *em* ~~del~~ `fn()`",
		},
		{
			name:  "overlapping strong and emphasis",
			items: []doc.Item{{Text: "a ", Marks: strong}, {Text: "b", Marks: both}, {Text: " c", Marks: strong}},
			want:  "**a *b* c**",
		},
		{
			name:  "longer reach opens outer",
			items: []doc.Item{{Text: "foo ", Marks: both}, {Text: "bar", Marks: em}},
			want:  "***foo** bar*",
		},
		{
			name:  "boundary whitespace hoisted out",
			items: []doc.Item{{Text: "say "}, {Text: " bold ", Marks: strong}, {Text: "now"}},
			want:  "say  **bold** now",
		},
		{
			name:  "code with embedded backticks",
			items: []doc.Item{{Text: "`x`", Marks: code}, {Text: " and "}, {Text: "a ``b c", Marks: code}},
			want:  "`` `x` `` and ```a ``b c```",
		},
		{
			name:  "inline formula",
			items: []doc.Item{{Text: "let "}, {Node: formula}, {Text: " hold"}},
			want:  "let $x^2 + y^2$ hold",
		},
		{
			name:  "marked multi-line split cleanly",
			items: []doc.Item{{Text: "line1\nline2", Marks: strong}},
			want:  "**line1**\n**line2**",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Slice(vocab.Slice{Inline: tc.items}); got != tc.want {
				t.Fatalf("Markdown = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMarkdownBlocksAndPartialCuts(t *testing.T) {
	s := profile.Schema()
	h := vocab.NewBlock(s, profile.Heading, doc.MakeAttrs(doc.Field{Num: profile.FieldLevel, Value: doc.Int(2)}), doc.TextItem("Heading", nil))
	p1 := profile.NewParagraph(doc.TextItem("Alpha", nil))
	p2 := profile.NewParagraph(doc.TextItem("Beta", nil))
	q := doc.NewNode(profile.Quote, doc.Attrs{}, doc.BlockSlot(
		profile.NewParagraph(doc.TextItem("Quoted", nil)),
		doc.NewNode(profile.Quote, doc.Attrs{}, doc.BlockSlot(profile.NewParagraph(doc.TextItem("Deep", nil)))),
	))

	// Whole heading (OpenStart == 0) keeps ## prefix.
	if got, want := Slice(vocab.Slice{Nodes: []*doc.Node{h, p1}}), "## Heading\n\nAlpha"; got != want {
		t.Fatalf("whole heading = %q, want %q", got, want)
	}
	// Cut heading (OpenStart == 1) omits ## prefix on the leading cut block.
	if got, want := Slice(vocab.Slice{Nodes: []*doc.Node{h, p1}, OpenStart: 1}), "Heading\n\nAlpha"; got != want {
		t.Fatalf("cut heading = %q, want %q", got, want)
	}
	// Nested quotes.
	if got, want := Slice(vocab.Slice{Nodes: []*doc.Node{q, p2}}), "> Quoted\n>\n> > Deep\n\nBeta"; got != want {
		t.Fatalf("quotes = %q, want %q", got, want)
	}

	// Lists: ordered with start, tasks, and nested bullet list.
	nestedList := doc.NewNode(profile.List, doc.Attrs{}, doc.BlockSlot(
		doc.NewNode(profile.Item, doc.Attrs{}, doc.InlineSlot(doc.TextItem("sub", nil)), doc.BlockSlot()),
	))
	orderedList := doc.NewNode(profile.List, doc.MakeAttrs(
		doc.Field{Num: profile.FieldListStyle, Value: doc.Int(int64(profile.Ordered))},
		doc.Field{Num: profile.FieldListStart, Value: doc.Int(3)},
	), doc.BlockSlot(
		doc.NewNode(profile.Item, doc.MakeAttrs(doc.Field{Num: profile.FieldTask, Value: doc.Int(int64(profile.TaskOpen))}),
			doc.InlineSlot(doc.TextItem("open", nil)), doc.BlockSlot(nestedList)),
		doc.NewNode(profile.Item, doc.MakeAttrs(doc.Field{Num: profile.FieldTask, Value: doc.Int(int64(profile.TaskDone))}),
			doc.InlineSlot(doc.TextItem("done", nil)), doc.BlockSlot()),
	))
	wantList := "3. [ ] open\n   - sub\n4. [x] done"
	if got := Slice(vocab.Slice{Nodes: []*doc.Node{orderedList}}); got != wantList {
		t.Fatalf("list = %q, want %q", got, wantList)
	}

	// Code (backtick escalation), Math, Divider, Figure.
	codeBlock := doc.NewNode(profile.Code, doc.MakeAttrs(doc.Field{Num: profile.FieldInfo, Value: doc.Text("md")}),
		doc.InlineSlot(doc.TextItem("```go\nx\n```", nil)))
	mathBlock := doc.NewNode(profile.Math, doc.Attrs{}, doc.InlineSlot(doc.TextItem("E = mc^2", nil)))
	divBlock := doc.NewNode(profile.Divider, doc.Attrs{})
	figBlock := profile.NewFigure(s, profile.Picture{Media: "bay photo.png", Width: 100, Height: 80}, "Bay [1]")

	wantMisc := "````md\n```go\nx\n```\n````\n\n$$\nE = mc^2\n$$\n\n---\n\n![Bay \\[1\\]](<bay photo.png>)"
	if got := Slice(vocab.Slice{Nodes: []*doc.Node{codeBlock, mathBlock, divBlock, figBlock}}); got != wantMisc {
		t.Fatalf("misc = %q, want %q", got, wantMisc)
	}
}
