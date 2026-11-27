package markdown

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

func pasteAndValidate(t *testing.T, s vocab.Slice) *doc.Snapshot {
	t.Helper()
	snap, err := profile.EmptyPage(1, 7, doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	tx := snap.Begin(doc.Origin{Class: doc.OriginPaste})
	slot, _ := vocab.FirstTextSlot(snap.Root())
	if err := vocab.PasteSlice(doc.At(slot, 0), s, nil)(tx); err != nil {
		t.Fatalf("PasteSlice: %v", err)
	}
	out, err := tx.Commit()
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := out.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return out
}

func TestMarkdownRoundTripAllKindsAndMarks(t *testing.T) {
	s := profile.Schema()
	strong := marksOf(profile.Strong)
	em := marksOf(profile.Emphasis)
	both := marksOf(profile.Strong, profile.Emphasis)
	strike := marksOf(profile.Strike)
	code := marksOf(profile.CodeMark)
	formula := doc.NewNode(profile.Formula, doc.Attrs{}, doc.InlineSlot(doc.TextItem("x^2 + y^2", nil)))

	h1 := vocab.NewBlock(s, profile.Heading, doc.MakeAttrs(doc.Field{Num: profile.FieldLevel, Value: doc.Int(2)}),
		doc.TextItem("Title", strong))
	p := profile.NewParagraph(
		doc.TextItem("plain ", nil),
		doc.TextItem("bold", strong),
		doc.TextItem(" ", nil),
		doc.TextItem("italic", em),
		doc.TextItem(" ", nil),
		doc.TextItem("both", both),
		doc.TextItem(" ", nil),
		doc.TextItem("gone", strike),
		doc.TextItem(" ", nil),
		doc.TextItem("`code`", code),
		doc.TextItem(" ", nil),
		doc.NodeItem(formula),
	)
	q := doc.NewNode(profile.Quote, doc.Attrs{}, doc.BlockSlot(
		profile.NewParagraph(doc.TextItem("Quoted", nil)),
		doc.NewNode(profile.Quote, doc.Attrs{}, doc.BlockSlot(profile.NewParagraph(doc.TextItem("Nested", em)))),
	))
	nestedBullet := doc.NewNode(profile.List, doc.Attrs{}, doc.BlockSlot(
		doc.NewNode(profile.Item, doc.Attrs{}, doc.InlineSlot(doc.TextItem("sub", nil)), doc.BlockSlot()),
	))
	list := doc.NewNode(profile.List, doc.MakeAttrs(
		doc.Field{Num: profile.FieldListStyle, Value: doc.Int(int64(profile.Ordered))},
		doc.Field{Num: profile.FieldListStart, Value: doc.Int(3)},
	), doc.BlockSlot(
		doc.NewNode(profile.Item, doc.MakeAttrs(doc.Field{Num: profile.FieldTask, Value: doc.Int(int64(profile.TaskOpen))}),
			doc.InlineSlot(doc.TextItem("open", nil)), doc.BlockSlot(nestedBullet)),
		doc.NewNode(profile.Item, doc.MakeAttrs(doc.Field{Num: profile.FieldTask, Value: doc.Int(int64(profile.TaskDone))}),
			doc.InlineSlot(doc.TextItem("done", nil)), doc.BlockSlot()),
	))
	codeBlock := doc.NewNode(profile.Code, doc.MakeAttrs(doc.Field{Num: profile.FieldInfo, Value: doc.Text("go")}),
		doc.InlineSlot(doc.TextItem("```go\nx := 1\n```", nil)))
	mathBlock := doc.NewNode(profile.Math, doc.Attrs{}, doc.InlineSlot(doc.TextItem("E = mc^2", nil)))
	divBlock := doc.NewNode(profile.Divider, doc.Attrs{})
	figBlock := profile.NewFigure(s, profile.Picture{Media: "/photos/bay photo.png", Width: 100, Height: 80}, "Bay [1]")

	table := profile.TableOf([][]string{{"Left", "Mid", "Right"}, {"a | b", "c", "d"}})
	shape := profile.TableShapeOf(table)
	shape.Columns[0].Align = profile.AlignStart
	shape.Columns[1].Align = profile.AlignCenter
	shape.Columns[2].Align = profile.AlignEnd
	rows := table.Slot(0).Children()
	rows[1] = doc.NewNode(profile.Row, doc.Attrs{}, doc.BlockSlot(
		vocab.NewBlock(s, profile.Cell, doc.Attrs{}, doc.TextItem("a | b", nil)),
		vocab.NewBlock(s, profile.Cell, doc.Attrs{}, doc.TextItem("c", strong)),
		vocab.NewBlock(s, profile.Cell, doc.Attrs{}, doc.TextItem("d", nil)),
	))
	table = doc.NewNode(profile.Table, shape.Attrs(), doc.BlockSlot(rows...))

	srcSlice := vocab.Slice{Nodes: []*doc.Node{h1, p, q, list, codeBlock, mathBlock, divBlock, figBlock, table}}
	md := Slice(srcSlice)
	parsed, ok := ParseWith(md, Options{
		Picture: func(source string) (profile.Picture, bool) {
			if source == "/photos/bay photo.png" {
				return profile.Picture{Media: source, Width: 100, Height: 80}, true
			}
			return profile.Picture{}, false
		},
	})
	if !ok || parsed.Empty() {
		t.Fatalf("ParseWith returned empty/unformatted on:\n%s", md)
	}
	pasteAndValidate(t, parsed)

	got := Slice(parsed)
	if got != md {
		t.Fatalf("round-trip mismatch:\ngot:\n%s\n\nwant:\n%s", got, md)
	}
}

func TestMarkdownReaderEdgeCasesAndNormalization(t *testing.T) {
	// Plain unformatted text returns formatted=false so S19/plain text path stays intact.
	if _, formatted := Parse("just plain text"); formatted {
		t.Fatal("expected formatted=false for plain text")
	}
	if _, formatted := Parse("line one\nline two"); formatted {
		t.Fatal("expected formatted=false for plain two-line text")
	}
	// Currency stays plain text; math becomes Formula.
	if _, formatted := Parse("costs $5 and $10"); formatted {
		t.Fatal("expected currency $5 and $10 to stay plain text")
	}
	s, ok := Parse("when $a * b$ and ~strike~ and **`code`** hold")
	if !ok || len(s.Nodes) != 0 || len(s.Inline) == 0 {
		t.Fatalf("unexpected inline slice: %+v ok=%v", s, ok)
	}
	pasteAndValidate(t, s)
	if got := Slice(s); got != "when $a * b$ and ~~strike~~ and `code` hold" {
		t.Fatalf("normalized inline = %q", got)
	}

	// Single-line $$ math $$, Typst fence, and CRLF + BOM + NodeByte sanitization.
	s2, ok := Parse("\uFEFF# Head\u0000\x1a\r\n\r\n$$a + b$$\r\n\r\n$$typst\nsum_(i=1)^n i\n$$")
	if !ok {
		t.Fatal("expected formatted slice")
	}
	snap := pasteAndValidate(t, s2)
	children := snap.Root().Slot(0).Children()
	if len(children) != 3 || children[0].Kind() != profile.Heading || children[1].Kind() != profile.Math || children[2].Kind() != profile.Math {
		t.Fatalf("unexpected blocks: %d", len(children))
	}
	if profile.MathDialectOf(children[2]) != profile.DialectTypst {
		t.Fatalf("expected Typst dialect on third block, got %v", profile.MathDialectOf(children[2]))
	}
	if strings.ContainsRune(snap.Root().Slot(0).Child(0).Text(), '\x1a') {
		t.Fatal("NodeByte leaked through sanitizer")
	}

	// Links degrade cleanly to their label while keeping inner marks.
	linkSlice, ok := Parse("See [**bold** link](https://example.com) and <https://go.dev>")
	if !ok {
		t.Fatal("expected formatted link slice")
	}
	pasteAndValidate(t, linkSlice)
	if got := Slice(linkSlice); got != "See **bold** link and https://go.dev" {
		t.Fatalf("links = %q", got)
	}

	// Deep nesting caps cleanly without overflowing or failing validation.
	deep := strings.Repeat("> ", 60) + "deep"
	deepSlice, ok := Parse(deep)
	if !ok {
		t.Fatal("expected formatted quote")
	}
	pasteAndValidate(t, deepSlice)
}

func TestMarkdownLocalPictures(t *testing.T) {
	home, _ := os.UserHomeDir()
	accept := func(p string) bool {
		return strings.HasSuffix(strings.ToLower(p), ".png")
	}
	md := strings.Join([]string{
		"Low tide",
		"![Bay](/photos/bay.png)",
		"![Web](https://example.com/web.png)",
		"Inline ![Small](/photos/small.png) stays inline",
		"```md",
		"![Code](/photos/code.png)",
		"```",
		"![Home](~/home.png)",
	}, "\n")
	pics := LocalPictures(md, accept)
	wantPaths := []string{"/photos/bay.png", filepath.Join(home, "home.png")}
	var gotPaths []string
	for _, p := range pics {
		gotPaths = append(gotPaths, p.Path)
	}
	if !slices.Equal(gotPaths, wantPaths) {
		t.Fatalf("LocalPictures = %v, want %v", gotPaths, wantPaths)
	}
}

func FuzzMarkdownParse(f *testing.F) {
	for _, seed := range []string{
		"",
		"# Heading\n\n- [ ] task\n- [x] **done**\n\n> quote\n\n$$\nx^2\n$$\n\n| a | b |\n| --- | ---: |\n| `c` | ~~d~~ |\n",
		strings.Repeat("> ", 80) + "*unclosed",
		"$$typst\nx\n",
		"$5 $x$ `a\rb\x00\x1ac`",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		slice, _ := Parse(input)
		if slice.Empty() {
			return
		}
		snap, err := profile.EmptyPage(1, 7, doc.NewInterner())
		if err != nil {
			t.Fatal(err)
		}
		tx := snap.Begin(doc.Origin{Class: doc.OriginPaste})
		slot, _ := vocab.FirstTextSlot(snap.Root())
		if err := vocab.PasteSlice(doc.At(slot, 0), slice, nil)(tx); err != nil {
			t.Fatalf("PasteSlice failed for %q: %v", input, err)
		}
		out, err := tx.Commit()
		if err != nil {
			t.Fatalf("Commit failed for %q: %v", input, err)
		}
		if err := out.Validate(); err != nil {
			t.Fatalf("Validate failed for %q: %v", input, err)
		}
	})
}
