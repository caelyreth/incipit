package editor

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

// typeKeys types text one character at a time, a line break as Return.
func typeKeys(t *testing.T, session *Session, text string) {
	t.Helper()
	for _, r := range text {
		if r == '\n' {
			run(t, session, TextNewline)
			continue
		}
		if _, err := session.ReplaceSelection(string(r), kernel.EventOriginTyping); err != nil {
			t.Fatalf("type %q: %v", r, err)
		}
	}
}

func pageKinds(session *Session) []model.NodeTypeID {
	kinds := make([]model.NodeTypeID, 0)
	for _, block := range DocumentBlocks(session.Runtime().Snapshot().Document()) {
		kinds = append(kinds, block.Kind())
	}
	return kinds
}

func expectKinds(t *testing.T, session *Session, kinds ...model.NodeTypeID) {
	t.Helper()
	got := pageKinds(session)
	if len(got) != len(kinds) {
		t.Fatalf("kinds = %q, want %q", got, kinds)
	}
	for index := range got {
		if got[index] != kinds[index] {
			t.Fatalf("kinds = %q, want %q", got, kinds)
		}
	}
}

// Notation typed at a paragraph's start opens the block it reads as; the
// page keeps the kind and level, and the notation is projected.
func TestTypedNotationOpensBlocks(t *testing.T) {
	for _, test := range []struct {
		typed string
		kind  model.NodeTypeID
		view  string
		text  string
	}{
		{"## Title", HeadingNode, "## Title|", "Title"},
		{"> Said", QuoteNode, "> Said|", "Said"},
		{"- Item", BulletNode, "- Item|", "Item"},
		{"+ Item", BulletNode, "- Item|", "Item"},
		{"7. Item", OrderedNode, "1. Item|", "Item"},
	} {
		t.Run(test.typed, func(t *testing.T) {
			session, _ := newSessionFixture(t, "", false)
			typeKeys(t, session, test.typed)
			expectKinds(t, session, test.kind)
			expectPage(t, session, test.view, test.text)
		})
	}
}

// Return after a heading starts a paragraph; in a list it starts the next
// item, numbered by its place. An empty block is kept as what it is, as an
// empty heading is: Return on an empty item opens another item.
func TestReturnFollowsBlockKind(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "# Top\nbody")
	expectKinds(t, session, HeadingNode, ParagraphNode)
	expectPage(t, session, "# Top\nbody|", "Top", "body")

	session, _ = newSessionFixture(t, "", false)
	typeKeys(t, session, "1. one\ntwo\n")
	expectKinds(t, session, OrderedNode, OrderedNode, OrderedNode)
	expectPage(t, session, "1. one\n2. two\n3. |", "one", "two", "")
	typeKeys(t, session, "\nfour")
	expectKinds(t, session, OrderedNode, OrderedNode, OrderedNode, OrderedNode)
	expectPage(t, session, "1. one\n2. two\n3. \n4. four|", "one", "two", "", "four")

	// An empty heading stays a heading the same way.
	session, _ = newSessionFixture(t, "", false)
	typeKeys(t, session, "# \n")
	expectKinds(t, session, HeadingNode, ParagraphNode)
	expectPage(t, session, "# \n|", "", "")
}

// Return before a heading's text leaves a paragraph above it.
func TestReturnAtHeadingStartKeepsHeading(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "## Top")
	run(t, session, TextLineStart, TextNewline)
	expectKinds(t, session, ParagraphNode, HeadingNode)
	expectPage(t, session, "\n## |Top", "", "Top")
}

// Backspace at a block's text start peels its structure a level: a nested
// item steps out, anything else becomes a paragraph of its own text. The
// notation, a number included, never becomes text.
func TestBackspacePeelsBlockStructure(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "## Top")
	run(t, session, TextLineStart, TextBackspace)
	expectKinds(t, session, ParagraphNode)
	expectPage(t, session, "|Top", "Top")

	session = nestedList(t)
	text := session.Projection().Text()
	at(t, session, model.UTF16Offset(strings.Index(text, "deeper")))
	run(t, session, TextBackspace)
	if got := session.Projection().Text(); got != "intro\n1. one\n  1. inner\n2. deeper\n3. two\nouter" {
		t.Fatalf("stepping out shows %q", got)
	}
	run(t, session, TextBackspace)
	if got := session.Projection().Text(); got != "intro\n1. one\n  1. inner\ndeeper\n1. two\nouter" {
		t.Fatalf("leaving the list shows %q", got)
	}
	if got := pageTexts(session); !slices.Equal(got, []string{"intro", "one", "inner", "deeper", "two", "outer"}) {
		t.Fatalf("blocks = %q", got)
	}
}

// Selecting any of a block's notation selects the whole of it: deleting
// part of a number, or of a heading's hashes, takes the structure, never
// leaving notation behind as text.
func TestBlockNotationIsOneUnit(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "intro\n1. one")
	if err := session.SelectProjected(6, 7); err != nil {
		t.Fatal(err)
	}
	run(t, session, TextBackspace)
	expectKinds(t, session, ParagraphNode, ParagraphNode)
	expectPage(t, session, "intro\n|one", "intro", "one")

	session, _ = newSessionFixture(t, "", false)
	typeKeys(t, session, "### Top")
	if err := session.SelectProjected(1, 2); err != nil {
		t.Fatal(err)
	}
	run(t, session, TextBackspace)
	expectKinds(t, session, ParagraphNode)
	expectPage(t, session, "|Top", "Top")
}

// Backspace at code's start takes the code block away and leaves its lines
// as paragraphs; the fences go with it.
func TestBackspaceAtCodeStartLeavesItsLines(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "```\na\nb")
	run(t, session, TextDocumentStart)
	at(t, session, 4)
	run(t, session, TextBackspace)
	expectKinds(t, session, ParagraphNode, ParagraphNode)
	expectPage(t, session, "|a\nb", "a", "b")
}

// Three dashes make a rule and go on in a paragraph after it; Backspace
// there takes the rule away.
func TestRules(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "---")
	expectKinds(t, session, RuleNode, ParagraphNode)
	expectPage(t, session, "---\n|", "", "")
	run(t, session, TextBackspace)
	expectKinds(t, session, ParagraphNode)
	expectPage(t, session, "|", "")
}

// Backticks and tildes mark code and struck text as stars mark emphasis;
// inside code, markers are text.
func TestInlineCodeAndStrike(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "a `b*c` ~d~")
	expectPage(t, session, "a `b*c` ~d~|", "a b*c d")
	marks := DocumentBlocks(session.Runtime().Snapshot().Document())[0].Marks()
	if len(marks) != 2 || marks[0].Kind() != CodeMark || marks[1].Kind() != StrikeMark {
		t.Fatalf("marks = %#v", marks)
	}
}

// Writing settings gate what typing reads as syntax.
func TestWritingSettingsGateSyntax(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	if _, err := session.Reconfigure(func(configuration *kernel.PolicyConfiguration) error {
		configuration.SetSetting(NewWritingSettingKey(headingsSetting), kernel.BoolSetting(false))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	typeKeys(t, session, "# a")
	expectKinds(t, session, ParagraphNode)
	expectPage(t, session, "# a|", "# a")
}

// Kinds are built in: with the writing policy off, typed notation stays
// text, while a heading still shows its notation and Return after it still
// starts a paragraph.
func TestKindsOutliveTheWritingPolicy(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "## Top")
	if _, err := session.Reconfigure(func(configuration *kernel.PolicyConfiguration) error {
		configuration.SetEnabled(WritingPolicyID, false)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	typeKeys(t, session, "\n# *x*")
	expectKinds(t, session, HeadingNode, ParagraphNode)
	expectPage(t, session, "## Top\n# *x*|", "Top", "# *x*")
}

// Deleting across blocks takes the notation it covers as a deletion within
// a block does: Select All and Backspace leave an empty paragraph, not a
// heading's "# " or a mark's empty "**".
func TestSpanDeletionTakesCoveredNotation(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "# Title\nsome *emph* text")
	run(t, session, TextSelectAll, TextSelectAll, TextBackspace)
	expectKinds(t, session, ParagraphNode)
	expectPage(t, session, "|", "")
	if marks := DocumentBlocks(session.Runtime().Snapshot().Document())[0].Marks(); len(marks) != 0 {
		t.Fatalf("marks left = %#v", marks)
	}
	undoTo(t, session, "# Title\nsome *emph* text")
}

// undoTo undoes once and expects the page to show text again.
func undoTo(t *testing.T, session *Session, text string) {
	t.Helper()
	if _, err := session.ExecuteHistory(HistoryUndo); err != nil {
		t.Fatal(err)
	}
	if got := session.Projection().Text(); got != text {
		t.Fatalf("undo shows %q, want %q", got, text)
	}
}

// A span from inside one block into another keeps what stays before and
// after it, its notation with it, and drops a mark whose markers it cut.
func TestSpanDeletionBetweenBlocks(t *testing.T) {
	for _, test := range []struct {
		name     string
		from, to model.UTF16Offset
		view     string
		kinds    []model.NodeTypeID
		texts    []string
	}{
		// "# Title\nsome *emph* text": from inside the title into the mark.
		{"into a mark", 4, 16, "# Ti|ph text", []model.NodeTypeID{HeadingNode}, []string{"Tiph text"}},
		// From inside the title to past the mark: it goes whole.
		{"past a mark", 4, 19, "# Ti| text", []model.NodeTypeID{HeadingNode}, []string{"Ti text"}},
		// From the page's start, the heading's notation covered.
		{"from the start", 0, 13, "|*emph* text", []model.NodeTypeID{ParagraphNode}, []string{"emph text"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			session, _ := newSessionFixture(t, "", false)
			typeKeys(t, session, "# Title\nsome *emph* text")
			if err := session.SelectProjected(test.from, test.to); err != nil {
				t.Fatal(err)
			}
			run(t, session, TextBackspace)
			expectKinds(t, session, test.kinds...)
			expectPage(t, session, test.view, test.texts...)
			undoTo(t, session, "# Title\nsome *emph* text")
		})
	}
}

// Select All steps out scope by scope: a block's text, then the whole page,
// then no further. While a scope is selected, the next one is framed with
// the keys that select it; the page, the last, frames nothing.
func TestSelectAllStepsOutByScope(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "# Title\nsome text")
	at(t, session, 12)
	keys := func(kernel.BindingTarget) []string { return []string{"⌘", "A"} }
	frames := func() []model.UTF16Range {
		frame, err := session.SurfaceFrame(time.Unix(0, 0), keys)
		if err != nil {
			t.Fatal(err)
		}
		var area []model.UTF16Range
		for _, decoration := range frame.Decorations() {
			if decoration.Primitive.Kind == surface.PrimitiveArea {
				area = append(area, decoration.Primitive.Ranges...)
			}
		}
		return area
	}
	if len(frames()) != 0 {
		t.Fatal("a caret framed a scope")
	}
	for _, want := range []model.UTF16Range{model.NewUTF16Range(8, 17), model.NewUTF16Range(0, 17)} {
		run(t, session, TextSelectAll)
		if got, _ := session.VisualSelection(); got != want {
			t.Fatalf("selected %v, want %v", got, want)
		}
	}
	if area := frames(); len(area) != 0 {
		t.Fatalf("the page framed %v", area)
	}
	run(t, session, TextSelectAll)
	if got, _ := session.VisualSelection(); got != model.NewUTF16Range(0, 17) {
		t.Fatalf("selected %v past the page", got)
	}
	at(t, session, 3)
	run(t, session, TextSelectAll)
	if got, _ := session.VisualSelection(); got != model.NewUTF16Range(2, 7) {
		t.Fatalf("heading scope %v, want its text", got)
	}
	if area := frames(); len(area) != 1 || area[0] != model.NewUTF16Range(0, 17) {
		t.Fatalf("framed %v, want the page", area)
	}
}

// Generated notation belongs to the block whose text it brackets even when
// the range Select All chooses leaves that notation out. A caret past an
// inline mark's closer is still in its block, and in every group that
// contains that block.
func TestSelectAllFromGeneratedNotationKeepsSemanticScope(t *testing.T) {
	steps := func(t *testing.T, session *Session, caret model.UTF16Offset, want ...string) {
		t.Helper()
		page := session.Projection().Text()
		if err := session.SelectProjected(caret, caret); err != nil {
			t.Fatal(err)
		}
		for _, scope := range want {
			run(t, session, TextSelectAll)
			selected, err := session.VisualSelection()
			if err != nil {
				t.Fatal(err)
			}
			if got := string([]rune(page)[selected.Start:selected.End]); got != scope {
				t.Fatalf("selected %q, want %q", got, scope)
			}
		}
	}

	t.Run("paragraph", func(t *testing.T) {
		session := newMarkedSession(t, "inline emphasis", markSpec{start: 0, end: 15, level: 1})
		page := session.Projection().Text()
		steps(t, session, model.UTF16Offset(strings.LastIndex(page, "*")+1),
			"inline emphasis", page)
	})

	t.Run("list item", func(t *testing.T) {
		document, blocks, err := NewBlockDocument("alpha\nbeta\ngamma")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := document.Transact(func(draft *model.DocumentDraft) error {
			for _, block := range blocks[:2] {
				if err := shapeBlock(draft, block, blockShape{kind: BulletNode}); err != nil {
					return err
				}
			}
			_, err := draft.AddMark(blocks[0], model.NewUTF16Range(0, 5), EmphasisMark, 1)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		session := sessionFor(t, document, blocks[0])
		page := session.Projection().Text()
		steps(t, session, model.UTF16Offset(strings.Index(page, "\n")),
			"alpha", "- *alpha*\n- beta", page)
	})
}

// Items nest by two spaces a level, quote lines by quoting again; ordered
// items count at their own depth, and a nested list starts its own count.
func TestNestedItemsAndQuotes(t *testing.T) {
	for _, test := range []struct {
		text  string
		shape blockShape
	}{
		{"  - ", blockShape{kind: BulletNode, depth: 1}},
		{"    3. ", blockShape{kind: OrderedNode, depth: 2}},
		{"> > ", blockShape{kind: QuoteNode, depth: 1}},
	} {
		shape, ok := parseOpener(test.text)
		if !ok || shape.kind != test.shape.kind || shape.depth != test.shape.depth {
			t.Fatalf("parseOpener(%q) = %#v, %v", test.text, shape, ok)
		}
		open, _ := blockNotation(shape, 1)
		if test.shape.kind != OrderedNode && open != test.text {
			t.Fatalf("notation of %#v = %q", shape, open)
		}
	}
	for _, bad := range []string{"  # ", "  > ", " "} {
		if _, ok := parseOpener(bad); ok {
			t.Fatalf("parseOpener(%q) read as syntax", bad)
		}
	}
}

// nestedList is a page of a nested ordered list between two paragraphs.
func nestedList(t *testing.T) *Session {
	t.Helper()
	document, blocks, err := NewBlockDocument("intro\none\ninner\ndeeper\ntwo\nouter")
	if err != nil {
		t.Fatal(err)
	}
	shapes := []blockShape{
		{kind: ParagraphNode},
		{kind: OrderedNode},
		{kind: OrderedNode, depth: 1},
		{kind: OrderedNode, depth: 1},
		{kind: OrderedNode},
		{kind: ParagraphNode},
	}
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		for index, shape := range shapes {
			if err := shapeBlock(draft, blocks[index], shape); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return sessionFor(t, document, blocks[0])
}

func TestNestedListsNumberByDepth(t *testing.T) {
	session := nestedList(t)
	expectKinds(t, session, ParagraphNode, OrderedNode, OrderedNode, OrderedNode, OrderedNode, ParagraphNode)
	if got := session.Projection().Text(); got != "intro\n1. one\n  1. inner\n  2. deeper\n2. two\nouter" {
		t.Fatalf("page shows %q", got)
	}
}

// Select All steps out of a nested item level by level: its text, the
// nested list, the whole list, the page.
func TestSelectAllStepsOutThroughNestedGroups(t *testing.T) {
	session := nestedList(t)
	text := session.Projection().Text()
	at(t, session, model.UTF16Offset(strings.Index(text, "inner")+2))
	want := []string{
		"inner",
		"  1. inner\n  2. deeper",
		"1. one\n  1. inner\n  2. deeper\n2. two",
		text,
	}
	for _, scope := range want {
		run(t, session, TextSelectAll)
		selected, _ := session.VisualSelection()
		if got := string([]rune(text)[selected.Start:selected.End]); got != scope {
			t.Fatalf("selected %q, want %q", got, scope)
		}
	}
}

// Items nest by command, a level at a time, never more than one level past
// the item before them, and come back out the same way.
func TestNestingCommands(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "- one\ntwo")
	nest := func(command NestingCommand, view string) {
		t.Helper()
		if _, err := session.ExecuteNesting(command); err != nil {
			t.Fatal(err)
		}
		if got := session.Projection().Text(); got != view {
			t.Fatalf("page shows %q, want %q", got, view)
		}
	}
	nest(NestDeeper, "- one\n  - two")
	nest(NestDeeper, "- one\n  - two")
	nest(NestShallower, "- one\n- two")
	nest(NestShallower, "- one\n- two")
	run(t, session, TextSelectAll, TextSelectAll, TextSelectAll)
	nest(NestDeeper, "- one\n  - two")
	if _, err := session.ExecuteHistory(HistoryUndo); err != nil {
		t.Fatal(err)
	}
	if got := session.Projection().Text(); got != "- one\n- two" {
		t.Fatalf("undo shows %q", got)
	}
}

// An empty item is a scope like any other: Select All there selects its
// list, then the page.
func TestSelectAllFromAnEmptyItem(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "# Hello world\nhello world\n- hello?\nfoo\n")
	text := session.Projection().Text()
	for _, scope := range []string{"- hello?\n- foo\n- ", text} {
		run(t, session, TextSelectAll)
		selected, _ := session.VisualSelection()
		if got := string([]rune(text)[selected.Start:selected.End]); got != scope {
			t.Fatalf("selected %q, want %q", got, scope)
		}
	}
}

// Bullet and numbered items are different lists side by side, while a list
// of either kind nested in another belongs to it.
func TestListKindsAreSeparateScopes(t *testing.T) {
	document, blocks, err := NewBlockDocument("Hello world\nHello?\nfoo\n\ninner\nouter")
	if err != nil {
		t.Fatal(err)
	}
	shapes := []blockShape{
		{kind: HeadingNode, level: 1},
		{kind: OrderedNode},
		{kind: OrderedNode},
		{kind: BulletNode},
		{kind: OrderedNode, depth: 1},
		{kind: BulletNode},
	}
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		for index, shape := range shapes {
			if err := shapeBlock(draft, blocks[index], shape); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	session := sessionFor(t, document, blocks[0])
	text := session.Projection().Text()
	steps := func(word string, want ...string) {
		t.Helper()
		at := strings.Index(text, word)
		if err := session.SelectProjected(model.UTF16Offset(at), model.UTF16Offset(at)); err != nil {
			t.Fatal(err)
		}
		for step, scope := range want {
			run(t, session, TextSelectAll)
			selected, _ := session.VisualSelection()
			if got := string([]rune(text)[selected.Start:selected.End]); got != scope {
				t.Fatalf("step %d selected %q, want %q", step+1, got, scope)
			}
		}
	}
	steps("foo", "foo", "1. Hello?\n2. foo", text)
	steps("inner", "inner", "  1. inner", "- \n  1. inner\n- outer", text)
}

// Each block's text takes the rhythm the Writing Profile gives its kind,
// notation and the break after it included: headings open what follows,
// items and quote lines are entries, code keeps its lines.
func TestBlocksTakeTheirRhythm(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "# Top\nbody\n- one\n")
	run(t, session, TextBackspace)
	typeKeys(t, session, "> said\n")
	run(t, session, TextBackspace)
	typeKeys(t, session, "```\ncode")
	flow := session.Presentation().Flow
	lines := strings.Split(flow.Text(), "\n")
	want := []surface.Rhythm{
		surface.RhythmTitle, surface.RhythmProse, surface.RhythmEntry,
		surface.RhythmEntry, surface.RhythmVerbatim, surface.RhythmVerbatim,
	}
	if len(lines) < len(want) {
		t.Fatalf("page shows %q", flow.Text())
	}
	start := 0
	for index, line := range lines[:len(want)] {
		end := start + len(line) + 1
		flow.EachRun(func(run surface.StyleRun) bool {
			if int(run.Bytes.Start) < end && int(run.Bytes.End) > start &&
				run.Style.Rhythm != want[index] {
				t.Fatalf("line %q has a %v run, want rhythm %v: %q",
					line, run.Style.Rhythm, want[index], flow.Text())
			}
			return true
		})
		start = end
	}
}

// The Writing Profile sets what its blocks look like in the surface's own
// terms: a heading larger by its level and strong, code in the mono face a
// little smaller, inline code within a heading smaller than the heading.
func TestBlocksTakeTheirLook(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "# Top `x`\n```\ncode")
	flow := session.Presentation().Flow
	style := func(text string) surface.TextStyle {
		at := strings.Index(flow.Text(), text)
		found := surface.TextStyle{}
		flow.EachRun(func(run surface.StyleRun) bool {
			found = run.Style
			return int(run.Bytes.End) <= at
		})
		return found
	}
	if got := style("Top"); got.Size() != 1.6 || got.Color != surface.ColorStrong || !got.Bold {
		t.Fatalf("heading text style %#v in %q", got, flow.Text())
	}
	if got := style("x"); got.Face != surface.FaceMono || got.Size() != 1.6*codeScale {
		t.Fatalf("inline code in a heading %#v", got)
	}
	if got := style("code"); got.Face != surface.FaceMono || got.Size() != codeScale {
		t.Fatalf("code block text %#v", got)
	}
}
