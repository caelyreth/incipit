package controller

import (
	"errors"
	"testing"
	"time"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func newController(t *testing.T, text string) *DocumentController {
	t.Helper()
	controller, err := NewTextDocument(text, fixedClock{now: time.Unix(1_700_000_000, 0)})
	if err != nil {
		t.Fatal(err)
	}
	return controller
}

func mustSelect(t *testing.T, controller *DocumentController, anchor, head int) {
	t.Helper()
	if err := controller.SelectProjected(anchor, head); err != nil {
		t.Fatal(err)
	}
}

func TestTextForRangeAlignsSurrogatesAndClampsActualRange(t *testing.T) {
	t.Parallel()
	controller := newController(t, "a🦀中")

	text, actual := controller.TextForRange(UTF16Range{Start: 2, End: 2})
	if text != "🦀" || actual != (UTF16Range{Start: 1, End: 3}) {
		t.Fatalf("split start query = %q %#v", text, actual)
	}
	text, actual = controller.TextForRange(UTF16Range{Start: 3, End: 99})
	if text != "中" || actual != (UTF16Range{Start: 3, End: 4}) {
		t.Fatalf("clamped end query = %q %#v", text, actual)
	}
	text, actual = controller.TextForRange(UTF16Range{Start: 0, End: 99})
	if text != "a🦀中" || actual != (UTF16Range{Start: 0, End: 4}) {
		t.Fatalf("full query = %q %#v", text, actual)
	}
	if text, actual := controller.TextForRange(UTF16Range{Start: 5, End: 9}); text != "" || actual != (UTF16Range{}) {
		t.Fatalf("beyond-end query = %q %#v", text, actual)
	}
	if text, actual := controller.TextForRange(UTF16Range{Start: 3, End: 1}); text != "" || actual != (UTF16Range{}) {
		t.Fatalf("reversed query = %q %#v", text, actual)
	}
	if controller.TextLengthUTF16() != 4 {
		t.Fatalf("text length = %d", controller.TextLengthUTF16())
	}
}

func TestSelectionDirectionAndSelectedTextArePreserved(t *testing.T) {
	t.Parallel()
	controller := newController(t, "a界中z")
	mustSelect(t, controller, 4, 1)

	selection := controller.Selection()
	if selection.Range != (UTF16Range{Start: 1, End: 4}) || !selection.Reversed {
		t.Fatalf("selection = %#v", selection)
	}
	if controller.Caret() != 1 {
		t.Fatalf("caret = %d", controller.Caret())
	}
	selected, err := controller.SelectedText()
	if err != nil {
		t.Fatal(err)
	}
	if selected != "界中z" {
		t.Fatalf("selected text = %q", selected)
	}

	mustSelect(t, controller, 1, 4)
	selection = controller.Selection()
	if selection.Range != (UTF16Range{Start: 1, End: 4}) || selection.Reversed {
		t.Fatalf("forward selection = %#v", selection)
	}
	if controller.Caret() != 4 {
		t.Fatalf("forward caret = %d", controller.Caret())
	}
}

func TestReplacementUsesExplicitRangesAndPreservesOrigin(t *testing.T) {
	t.Parallel()
	controller := newController(t, "a🦀中")
	range16 := UTF16Range{Start: 1, End: 3}
	outcome, err := controller.Replace(&range16, "界", kernel.EventOriginTyping)
	if err != nil {
		t.Fatal(err)
	}
	if outcome != kernel.OutcomeHandled || controller.Text() != "a界中" {
		t.Fatalf("replace outcome=%v text=%q", outcome, controller.Text())
	}
	if controller.Caret() != 2 {
		t.Fatalf("caret = %d", controller.Caret())
	}
	if len(controller.Activity()) != 1 {
		t.Fatalf("activity = %#v", controller.Activity())
	}

	bad := UTF16Range{Start: 1, End: 99}
	if _, err := controller.Replace(&bad, "x", kernel.EventOriginTyping); !errors.Is(err, ErrInvalidUTF16Range) {
		t.Fatalf("invalid replacement error = %v", err)
	}
	if controller.Text() != "a界中" {
		t.Fatalf("invalid replacement changed text = %q", controller.Text())
	}

	mustSelect(t, controller, 1, 1)
	outcome, err = controller.Replace(nil, "[]", kernel.EventOriginPaste)
	if err != nil || outcome != kernel.OutcomeHandled || controller.Text() != "a[]界中" {
		t.Fatalf("selection replacement outcome=%v err=%v text=%q", outcome, err, controller.Text())
	}
}

func TestCollapsedReplacementAtCaretPreservesVisualCaretIdentity(t *testing.T) {
	t.Parallel()
	controller := newController(t, "abc")
	mustSelect(t, controller, 0, 3)
	if outcome, err := controller.Session().ExecuteMarkup(editor.MarkupEmphasis); err != nil ||
		outcome != kernel.OutcomeHandled {
		t.Fatalf("markup outcome=%v err=%v", outcome, err)
	}
	controller.Refresh()
	if controller.Text() != "*abc*" {
		t.Fatalf("marked text = %q", controller.Text())
	}

	mustSelect(t, controller, 3, 3)
	if controller.Caret() != 3 {
		t.Fatalf("caret before generated-marker insert = %d", controller.Caret())
	}
	atCaret := UTF16Range{Start: 3, End: 3}
	if outcome, err := controller.Replace(&atCaret, "d", kernel.EventOriginTyping); err != nil ||
		outcome != kernel.OutcomeHandled {
		t.Fatalf("caret insert outcome=%v err=%v", outcome, err)
	}
	if controller.Text() != "*abdc*" {
		t.Fatalf("caret insert text = %q", controller.Text())
	}
	if controller.Caret() != 4 {
		t.Fatalf("caret after generated-marker insert = %d", controller.Caret())
	}
}

func TestExecuteRoutesTextHistoryWordMarkupAndApplicationCommands(t *testing.T) {
	t.Parallel()
	controller := newController(t, "one two")

	if outcome, err := controller.Execute(Command{
		ID:     string(editor.TextSelectAll.ID()),
		Origin: kernel.EventOriginSemantic,
	}); err != nil || outcome != kernel.OutcomeHandled {
		t.Fatalf("select all outcome=%v err=%v", outcome, err)
	}
	if controller.Selection().Range != (UTF16Range{Start: 0, End: 7}) {
		t.Fatalf("select all selection = %#v", controller.Selection())
	}
	if outcome, err := controller.Execute(Command{
		ID:     string(editor.MarkupEmphasis.ID()),
		Origin: kernel.EventOriginSemantic,
	}); err != nil || outcome != kernel.OutcomeHandled {
		t.Fatalf("markup outcome=%v err=%v", outcome, err)
	}
	if controller.Text() != "*one two*" {
		t.Fatalf("markup text = %q", controller.Text())
	}
	if outcome, err := controller.Execute(Command{
		ID:     string(editor.WordPrevious.ID()),
		Origin: kernel.EventOriginSemantic,
	}); err != nil || outcome != kernel.OutcomeHandled {
		t.Fatalf("word outcome=%v err=%v", outcome, err)
	}
	if controller.Caret() != 5 {
		t.Fatalf("word caret = %d", controller.Caret())
	}

	outside := Command{ID: "workspace.next-page", Origin: kernel.EventOriginSemantic}
	if outcome, err := controller.Execute(outside); !errors.Is(err, ErrUnknownCommand) ||
		outcome != kernel.OutcomePass {
		t.Fatalf("outside command outcome=%v err=%v", outcome, err)
	}
	queued := controller.DrainCommands()
	if len(queued) != 1 || queued[0].ID != outside.ID {
		t.Fatalf("queued commands = %#v", queued)
	}
	if len(controller.DrainCommands()) != 0 {
		t.Fatal("draining commands was not destructive")
	}

	mustSelect(t, controller, 4, 4)
	if outcome, err := controller.Execute(Command{
		ID:     "text.insert",
		Origin: kernel.EventOriginTyping,
		Range:  &UTF16Range{Start: 4, End: 4},
		Text:   "!",
	}); err != nil || outcome != kernel.OutcomeHandled {
		t.Fatalf("explicit command outcome=%v err=%v", outcome, err)
	}
	if controller.Text() != "*one! two*" {
		t.Fatalf("explicit command text = %q", controller.Text())
	}
	if outcome, err := controller.Execute(Command{
		ID:     string(editor.HistoryUndo.ID()),
		Origin: kernel.EventOriginSemantic,
	}); err != nil || outcome != kernel.OutcomeHandled {
		t.Fatalf("undo outcome=%v err=%v", outcome, err)
	}
	if controller.Text() != "*one two*" {
		t.Fatalf("undo text = %q", controller.Text())
	}
}

func TestCompositionPreviewCommitsOneHandledEdit(t *testing.T) {
	t.Parallel()
	controller := newController(t, "a🦀z")
	mustSelect(t, controller, 1, 3)
	before := controller.Runtime().Revision()

	controller.SetMarkedText(nil, "ni", UTF16Range{Start: 1, End: 2})
	if !controller.IsComposing() || controller.Text() != "aniz" {
		t.Fatalf("preview composing=%v text=%q", controller.IsComposing(), controller.Text())
	}
	if marked, ok := controller.MarkedRange(); !ok || marked != (UTF16Range{Start: 1, End: 3}) {
		t.Fatalf("marked range = %#v %v", marked, ok)
	}
	if selection := controller.Selection(); selection.Range != (UTF16Range{Start: 2, End: 3}) || selection.Reversed {
		t.Fatalf("preview selection = %#v", selection)
	}
	if _, ok := controller.PreviewFrame(); !ok {
		t.Fatal("missing preview frame")
	}

	final := "你("
	outcome, err := controller.CommitComposition(nil, &final)
	if err != nil {
		t.Fatal(err)
	}
	if outcome != kernel.OutcomeHandled {
		t.Fatalf("commit outcome = %v", outcome)
	}
	if controller.IsComposing() || controller.Text() != "a你(z" {
		t.Fatalf("committed composing=%v text=%q", controller.IsComposing(), controller.Text())
	}
	if controller.Runtime().Revision() != before+1 {
		t.Fatalf("revision = %d, want %d", controller.Runtime().Revision(), before+1)
	}
	if controller.Caret() != 3 {
		t.Fatalf("commit caret = %d", controller.Caret())
	}
	if _, ok := controller.PreviewFrame(); ok {
		t.Fatal("handled commit retained preview frame")
	}
}

func TestMarkedTextReplacesExplicitSourceRange(t *testing.T) {
	t.Parallel()
	controller := newController(t, "a🦀z")
	controller.SetMarkedText(
		&UTF16Range{Start: 1, End: 3},
		"ni",
		UTF16Range{Start: 1, End: 2},
	)
	if !controller.IsComposing() || controller.Text() != "aniz" {
		t.Fatalf("explicit preview composing=%v text=%q", controller.IsComposing(), controller.Text())
	}
	if marked, ok := controller.MarkedRange(); !ok ||
		marked != (UTF16Range{Start: 1, End: 3}) {
		t.Fatalf("explicit marked = %#v, %v", marked, ok)
	}
	outcome, err := controller.CommitComposition(nil, nil)
	if err != nil || outcome != kernel.OutcomeHandled {
		t.Fatalf("explicit commit outcome=%v err=%v", outcome, err)
	}
	if controller.Text() != "aniz" || controller.IsComposing() {
		t.Fatalf("explicit committed text=%q composing=%v", controller.Text(), controller.IsComposing())
	}
}

func TestMarkedTextUpdatesOnlyInsideItsRange(t *testing.T) {
	t.Parallel()
	controller := newController(t, "abz")
	mustSelect(t, controller, 1, 1)
	controller.SetMarkedText(nil, "ni", UTF16Range{Start: 2, End: 2})
	if controller.Text() != "anibz" {
		t.Fatalf("initial preview = %q", controller.Text())
	}

	edit := UTF16Range{Start: 2, End: 3}
	controller.SetMarkedText(&edit, "hao", UTF16Range{Start: 4, End: 4})
	if controller.Text() != "anhaobz" {
		t.Fatalf("edited preview = %q", controller.Text())
	}
	if marked, _ := controller.MarkedRange(); marked != (UTF16Range{Start: 1, End: 5}) {
		t.Fatalf("edited marked range = %#v", marked)
	}
	if controller.LastError() != nil {
		t.Fatalf("marked edit error = %v", controller.LastError())
	}

	before := controller.Text()
	outside := UTF16Range{Start: 0, End: 2}
	controller.SetMarkedText(&outside, "x", UTF16Range{Start: 1, End: 1})
	if !errors.Is(controller.LastError(), ErrMarkedRange) {
		t.Fatalf("outside marked error = %v", controller.LastError())
	}
	if controller.Text() != before {
		t.Fatalf("failed marked edit changed text to %q", controller.Text())
	}
}

func TestFailedMarkedUpdateKeepsPriorPreviewAndSelection(t *testing.T) {
	t.Parallel()
	controller := newController(t, "abc")
	mustSelect(t, controller, 1, 1)
	controller.SetMarkedText(nil, "你", UTF16Range{Start: 1, End: 1})
	beforeText := controller.Text()
	beforeMarked, _ := controller.MarkedRange()
	beforeSelection := controller.Selection()

	controller.SetMarkedText(
		&UTF16Range{Start: 1, End: 2},
		"x",
		UTF16Range{Start: 2, End: 2},
	)
	if !errors.Is(controller.LastError(), ErrInvalidUTF16Range) {
		t.Fatalf("split update error = %v", controller.LastError())
	}
	if controller.Text() != beforeText {
		t.Fatalf("failed update text = %q", controller.Text())
	}
	if marked, _ := controller.MarkedRange(); marked != beforeMarked {
		t.Fatalf("failed update marked = %#v", marked)
	}
	if selection := controller.Selection(); selection != beforeSelection {
		t.Fatalf("failed update selection = %#v", selection)
	}
}

func TestReplaceTextNilCommitsMarkedTextThenUsesSelection(t *testing.T) {
	t.Parallel()
	controller := newController(t, "abc")
	mustSelect(t, controller, 1, 1)
	controller.SetMarkedText(nil, "你", UTF16Range{Start: 1, End: 1})
	if controller.Text() != "a你bc" {
		t.Fatalf("preedit = %q", controller.Text())
	}

	controller.ReplaceText(nil, "好")
	if controller.IsComposing() || controller.Text() != "a好bc" {
		t.Fatalf("committed preedit composing=%v text=%q", controller.IsComposing(), controller.Text())
	}
	controller.ReplaceText(nil, "!")
	if controller.Text() != "a好!bc" {
		t.Fatalf("selection replacement = %q", controller.Text())
	}
}

func TestReplaceTextExplicitRangeCommitsPreviewAsOneIMEEdit(t *testing.T) {
	t.Parallel()
	controller := newController(t, "abc")
	mustSelect(t, controller, 0, 0)
	controller.SetMarkedText(
		&UTF16Range{Start: 0, End: 0},
		"你",
		UTF16Range{Start: 1, End: 1},
	)
	before := controller.Runtime().Revision()
	controller.ReplaceText(&UTF16Range{Start: 0, End: 1}, "好")
	if controller.IsComposing() || controller.Text() != "好abc" {
		t.Fatalf("explicit native commit text=%q composing=%v", controller.Text(), controller.IsComposing())
	}
	if controller.Runtime().Revision() != before+1 {
		t.Fatalf("explicit native commit revisions=%d->%d", before, controller.Runtime().Revision())
	}
}

func TestSetMarkedTextUpdatesSelectedPreeditSlice(t *testing.T) {
	t.Parallel()
	controller := newController(t, "z")
	mustSelect(t, controller, 0, 0)
	controller.SetMarkedText(nil, "ni", UTF16Range{Start: 2, End: 2})
	controller.SetMarkedText(
		&UTF16Range{Start: 0, End: 1},
		"你",
		UTF16Range{Start: 1, End: 1},
	)
	if controller.Text() != "你iz" {
		t.Fatalf("partial preedit = %q", controller.Text())
	}
	if marked, _ := controller.MarkedRange(); marked != (UTF16Range{Start: 0, End: 2}) {
		t.Fatalf("partial marked = %#v", marked)
	}
	controller.CancelComposition()
	if controller.Text() != "z" {
		t.Fatalf("partial cancel = %q", controller.Text())
	}
}

func TestCompositionCancelAndFailureRestoreOrRetainSourcePreview(t *testing.T) {
	t.Parallel()
	controller := newController(t, "abc")
	mustSelect(t, controller, 1, 1)
	source := controller.Text()
	controller.SetMarkedText(nil, "🦀中", UTF16Range{Start: 2, End: 2})
	if controller.Text() != "a🦀中bc" || !controller.IsComposing() {
		t.Fatalf("preview = %q composing=%v", controller.Text(), controller.IsComposing())
	}
	controller.CancelComposition()
	if controller.Text() != source || controller.IsComposing() {
		t.Fatalf("cancel composing=%v text=%q", controller.IsComposing(), controller.Text())
	}
	if controller.Selection().Range != (UTF16Range{Start: 1, End: 1}) {
		t.Fatalf("cancel selection = %#v", controller.Selection())
	}
}

func TestDeclinedCompositionRetainsPreviewAndRecordsError(t *testing.T) {
	t.Parallel()
	controller := newController(t, "abc")
	if err := controller.Reconfigure(func(configuration *kernel.PolicyConfiguration) error {
		for _, policy := range editor.AllEditorPolicies {
			configuration.SetEnabled(policy.ID(), false)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	mustSelect(t, controller, 0, 0)
	controller.SetMarkedText(nil, "你", UTF16Range{Start: 1, End: 1})
	if !controller.IsComposing() || controller.Text() != "你abc" {
		t.Fatalf("preview = %q composing=%v", controller.Text(), controller.IsComposing())
	}

	controller.UnmarkText()
	if !controller.IsComposing() {
		t.Fatal("declined unmark cleared preview")
	}
	if controller.Text() != "你abc" {
		t.Fatalf("declined unmark text = %q", controller.Text())
	}
	if !errors.Is(controller.LastError(), ErrPolicyDeclined) {
		t.Fatalf("declined unmark error = %v", controller.LastError())
	}
	if controller.Status().Outcome != kernel.OutcomePass || !controller.Status().Composing {
		t.Fatalf("declined status = %#v", controller.Status())
	}
	controller.CancelComposition()
	if controller.Text() != "abc" {
		t.Fatalf("cancel restored %q", controller.Text())
	}
}

func TestStaleCompositionFailureRetainsPreviewForCancellation(t *testing.T) {
	t.Parallel()
	controller := newController(t, "abc")
	mustSelect(t, controller, 1, 1)
	controller.SetMarkedText(nil, "你", UTF16Range{Start: 1, End: 1})
	preview := controller.Text()

	if _, err := controller.Session().Reconfigure(func(configuration *kernel.PolicyConfiguration) error {
		configuration.SetEnabled(editor.MarkupPolicyID, false)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	controller.Refresh()
	outcome, err := controller.CommitComposition(nil, nil)
	if err == nil || outcome != kernel.OutcomePass {
		t.Fatalf("stale commit outcome=%v err=%v", outcome, err)
	}
	if controller.LastError() == nil {
		t.Fatal("stale commit did not record an error")
	}
	if !controller.IsComposing() {
		t.Fatal("stale commit cleared composition")
	}
	if controller.Text() != preview {
		t.Fatalf("stale preview = %q, want %q", controller.Text(), preview)
	}
	controller.CancelComposition()
	if controller.Text() != "abc" || controller.IsComposing() {
		t.Fatalf("cancel after stale text=%q composing=%v", controller.Text(), controller.IsComposing())
	}
}

func TestEditableSpanAndVisibleLengthFollowGeneratedNotationAndPreview(t *testing.T) {
	t.Parallel()
	controller := newController(t, "abc")
	if span := controller.EditableRange(); span != (UTF16Range{Start: 0, End: 3}) {
		t.Fatalf("plain editable span = %#v", span)
	}
	mustSelect(t, controller, 0, 3)
	if outcome, err := controller.Session().ExecuteMarkup(editor.MarkupEmphasis); err != nil ||
		outcome != kernel.OutcomeHandled {
		t.Fatalf("markup outcome=%v err=%v", outcome, err)
	}
	controller.Refresh()
	if controller.Text() != "*abc*" || controller.TextLengthUTF16() != 5 {
		t.Fatalf("marked text=%q length=%d", controller.Text(), controller.TextLengthUTF16())
	}
	if span := controller.EditableRange(); span != (UTF16Range{Start: 1, End: 4}) {
		t.Fatalf("marked editable span = %#v", span)
	}
	mustSelect(t, controller, 1, 2)
	controller.SetMarkedText(nil, "你🦀", UTF16Range{Start: 3, End: 3})
	if controller.Text() != "*你🦀bc*" || controller.TextLengthUTF16() != 7 {
		t.Fatalf("preview text=%q length=%d", controller.Text(), controller.TextLengthUTF16())
	}
	if marked, _ := controller.MarkedRange(); marked != (UTF16Range{Start: 1, End: 4}) {
		t.Fatalf("marked range = %#v", marked)
	}
	if span := controller.EditableRange(); span != (UTF16Range{Start: 1, End: 6}) {
		t.Fatalf("preview editable span = %#v", span)
	}
}

func TestMetricsQueueAndStaleInstall(t *testing.T) {
	t.Parallel()
	controller := newController(t, "one two\n\nthree")
	if controller.Metrics() != (Metrics{Characters: 14, Words: 3, Paragraphs: 2}) {
		t.Fatalf("initial metrics = %#v", controller.Metrics())
	}
	if controller.MetricsPending() {
		t.Fatal("initial metrics unexpectedly pending")
	}

	mustSelect(t, controller, 13, 13)
	if outcome, err := controller.ReplaceSelection(" four", kernel.EventOriginTyping); err != nil ||
		outcome != kernel.OutcomeHandled {
		t.Fatalf("append outcome=%v err=%v", outcome, err)
	}
	source, ok := controller.PendingMetricSource()
	if !ok || !controller.MetricsPending() {
		t.Fatal("replacement did not queue metrics")
	}
	computed, ok := source.Compute()
	if !ok || computed.Words != 4 {
		t.Fatalf("computed metrics = %#v, %v", computed, ok)
	}
	if controller.InstallMetrics(MetricSource{}, Metrics{Characters: 1}) {
		t.Fatal("stale empty source installed")
	}
	if !controller.InstallMetrics(source, computed) {
		t.Fatal("current source did not install")
	}
	if controller.MetricsPending() || controller.Metrics() != computed {
		t.Fatalf("installed pending=%v metrics=%#v", controller.MetricsPending(), controller.Metrics())
	}
}

func TestMetricsForCurrentTextTracksQueuedSemanticEdit(t *testing.T) {
	document, err := NewTextDocument("alpha beta", fixedClock{now: time.Unix(1_700_000_000, 0)})
	if err != nil {
		t.Fatal(err)
	}
	mustSelect(t, document, 0, 10)
	if _, err := document.ReplaceSelection("gamma", kernel.EventOriginTyping); err != nil {
		t.Fatal(err)
	}
	metrics, ok := document.MetricsForCurrentText()
	if !ok || metrics.Words != 1 || metrics.Paragraphs != 1 || metrics.Characters != 5 {
		t.Fatalf("metrics = %#v,%v", metrics, ok)
	}
}

func TestMetricSourceComputeAndStaleInstallAfterSecondEdit(t *testing.T) {
	t.Parallel()
	controller := newController(t, "one")
	mustSelect(t, controller, 3, 3)
	if outcome, err := controller.ReplaceSelection(" two", kernel.EventOriginTyping); err != nil ||
		outcome != kernel.OutcomeHandled {
		t.Fatalf("first edit outcome=%v err=%v", outcome, err)
	}
	first, ok := controller.PendingMetricSource()
	if !ok {
		t.Fatal("first edit did not queue metrics")
	}
	firstMetrics, ok := first.Compute()
	if !ok || firstMetrics.Words != 2 {
		t.Fatalf("first metrics = %#v, %v", firstMetrics, ok)
	}
	if outcome, err := controller.ReplaceSelection(" three", kernel.EventOriginTyping); err != nil ||
		outcome != kernel.OutcomeHandled {
		t.Fatalf("second edit outcome=%v err=%v", outcome, err)
	}
	if controller.InstallMetrics(first, firstMetrics) {
		t.Fatal("stale first metric source installed after second edit")
	}
	current, ok := controller.PendingMetricSource()
	if !ok || !controller.RefreshMetrics() {
		t.Fatal("current metrics did not refresh")
	}
	if current.Node == first.Node && current.Root.Same(first.Root) {
		t.Fatal("successive edits retained the same metric root")
	}
	if currentMetrics := controller.Metrics(); currentMetrics.Words != 3 {
		t.Fatalf("current metrics = %#v", currentMetrics)
	}
}

func TestReconfigureReprojectsAndRefreshesCaches(t *testing.T) {
	t.Parallel()
	controller := newController(t, "abc")
	mustSelect(t, controller, 0, 3)
	if outcome, err := controller.Session().ExecuteMarkup(editor.MarkupEmphasis); err != nil ||
		outcome != kernel.OutcomeHandled {
		t.Fatalf("markup outcome=%v err=%v", outcome, err)
	}
	controller.Refresh()
	if controller.Text() != "*abc*" {
		t.Fatalf("marked text = %q", controller.Text())
	}
	before := controller.Runtime().Revision()
	err := controller.Reconfigure(func(configuration *kernel.PolicyConfiguration) error {
		configuration.SetEnabled(editor.MarkupPolicyID, false)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if controller.Runtime().Revision() != before+1 {
		t.Fatalf("reconfigure revision = %d", controller.Runtime().Revision())
	}
	if controller.Text() != "abc" {
		t.Fatalf("reconfigure did not refresh projection: %q", controller.Text())
	}
	if controller.Selection().Range != (UTF16Range{Start: 0, End: 3}) {
		t.Fatalf("selection after reconfigure = %#v", controller.Selection())
	}
	if controller.LastError() != nil {
		t.Fatalf("reconfigure error = %v", controller.LastError())
	}
}

func TestReconfigureIsRejectedDuringComposition(t *testing.T) {
	t.Parallel()
	controller := newController(t, "abc")
	mustSelect(t, controller, 1, 1)
	controller.SetMarkedText(nil, "你", UTF16Range{Start: 1, End: 1})
	before := controller.Runtime().Revision()
	err := controller.Reconfigure(func(configuration *kernel.PolicyConfiguration) error {
		configuration.SetEnabled(editor.MarkupPolicyID, false)
		return nil
	})
	if !IsCompositionActiveError(err) {
		t.Fatalf("reconfigure composition error = %v", err)
	}
	if controller.Runtime().Revision() != before || !controller.IsComposing() {
		t.Fatal("rejected reconfigure changed revision or composition")
	}
	if !errors.Is(controller.LastError(), editor.ErrCompositionActive) {
		t.Fatalf("recorded reconfigure error = %v", controller.LastError())
	}
}

type testGeometry struct {
	text  string
	point Point
}

func (g *testGeometry) BoundsForRange(
	text string,
	range16 UTF16Range,
) (Rect, UTF16Range, bool) {
	g.text = text
	return Rect{X: float32(range16.Start), W: 1, H: 10}, range16, true
}

func (g *testGeometry) IndexForPoint(text string, point Point) (int, bool) {
	g.text = text
	g.point = point
	return int(point.X), true
}

func TestNativeGeometryDelegatesVisibleText(t *testing.T) {
	t.Parallel()
	controller := newController(t, "abc")
	geometry := &testGeometry{}
	controller.SetGeometry(geometry)
	mustSelect(t, controller, 1, 1)
	controller.SetMarkedText(nil, "你", UTF16Range{Start: 1, End: 1})

	if bounds, actual, ok := controller.BoundsForRange(UTF16Range{Start: 1, End: 2}); !ok ||
		actual != (UTF16Range{Start: 1, End: 2}) || bounds.W != 1 {
		t.Fatalf("bounds = %#v %#v %v", bounds, actual, ok)
	}
	if geometry.text != "a你bc" {
		t.Fatalf("bounds text = %q", geometry.text)
	}
	if index, ok := controller.IndexForPoint(Point{X: 3, Y: 4}); !ok || index != 3 {
		t.Fatalf("index = %d %v", index, ok)
	}
	if geometry.point != (Point{X: 3, Y: 4}) {
		t.Fatalf("geometry point = %#v", geometry.point)
	}
}

func TestFrameDeadlineExposesActiveAnimationWithoutPreview(t *testing.T) {
	t.Parallel()
	controller := newController(t, "abc")
	controller.Session().Feedback().PublishAt(nil, time.Unix(1_700_000_000, 0))
	if deadline, ok := controller.FrameDeadline(); ok {
		t.Fatalf("empty feedback deadline = %v", deadline)
	}
	mustSelect(t, controller, 0, 1)
	if outcome, err := controller.ReplaceSelection("x", kernel.EventOriginTyping); err != nil ||
		outcome != kernel.OutcomeHandled {
		t.Fatalf("replacement outcome=%v err=%v", outcome, err)
	}
	if _, ok := controller.FrameDeadline(); ok {
		t.Fatal("plain replacement unexpectedly scheduled an animation")
	}
	controller.Session().Feedback().PublishAt(
		controller.Session().Feedback().Targets(),
		time.Now(),
	)
	if _, ok := controller.FrameDeadline(); ok {
		t.Fatal("feedback without targets scheduled an animation")
	}
}
