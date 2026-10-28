package controller

import (
	"fmt"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

// TextForRange returns visible text and the scalar-aligned actual range.
// Split-surrogate endpoints expand outward; the end is clamped to the text.
func (c *DocumentController) TextForRange(range16 UTF16Range) (string, UTF16Range) {
	if range16.Start < 0 || range16.Start > range16.End {
		return "", UTF16Range{}
	}
	aligned, bytes, ok := model.AlignUTF16RangeForNativeQuery(
		c.text,
		model.NewUTF16Range(
			model.UTF16Offset(range16.Start),
			model.UTF16Offset(range16.End),
		),
	)
	if !ok {
		return "", UTF16Range{}
	}
	return c.text[int(bytes.Start):int(bytes.End)], UTF16Range{
		Start: int(aligned.Start),
		End:   int(aligned.End),
	}
}

// ReplaceText is the MyGo native-input entry point. While preedit is active,
// text replaces the named marked range and commits as one IME edit.
func (c *DocumentController) ReplaceText(range16 *UTF16Range, text string) {
	if c.preedit != nil {
		outcome, err := c.commitPreedit(range16, &text)
		c.finishNativeOutcome(outcome, err)
		return
	}
	outcome, err := c.Replace(range16, text, kernel.EventOriginTyping)
	c.finishNativeOutcome(outcome, err)
}

// SetMarkedText begins, replaces, or partially edits transient preedit.
func (c *DocumentController) SetMarkedText(
	range16 *UTF16Range,
	text string,
	selected UTF16Range,
) {
	if !model.ValidDocumentText(text) {
		c.recordNativeError(fmt.Errorf("controller: marked text is not valid document text"))
		return
	}
	if text == "" {
		c.cancelComposition()
		c.setStatus(kernel.OutcomeHandled, false, nil)
		return
	}
	if c.metricsQueued {
		c.RefreshMetrics()
	}
	if c.preedit != nil {
		preview, err := c.updatedPreview(range16, text)
		if err != nil {
			c.recordNativeError(err)
			return
		}
		selected = selected.Normalized()
		if err := c.validateTextRange(preview, selected); err != nil {
			c.recordNativeError(fmt.Errorf("controller: invalid marked selection: %w", err))
			return
		}
		if err := c.session.UpdateComposition(preview); err != nil {
			c.recordNativeError(err)
			return
		}
		c.preedit.selected = selected
		if err := c.refreshPreview(); err != nil {
			c.recordNativeError(err)
			return
		}
		c.setStatus(kernel.OutcomeHandled, false, nil)
		return
	}

	original := c.sourceSelect
	if range16 != nil {
		if range16.Start < 0 || range16.End < range16.Start {
			c.recordNativeError(ErrInvalidUTF16Range)
			return
		}
		original = *range16
	}
	if err := c.validateTextRange(c.sourceText, original); err != nil {
		c.recordNativeError(err)
		return
	}
	target, err := c.compositionTarget(original)
	if err != nil {
		c.recordNativeError(err)
		return
	}
	if err := c.session.Composition().Begin(
		c.session.Projection(),
		target,
		c.Runtime().Snapshot().Document(),
	); err != nil {
		c.recordNativeError(err)
		return
	}
	if err := c.session.UpdateComposition(text); err != nil {
		c.session.CancelComposition()
		c.recordNativeError(err)
		return
	}
	selected = selected.Normalized()
	if err := c.validateTextRange(text, selected); err != nil {
		c.session.CancelComposition()
		c.recordNativeError(fmt.Errorf("controller: invalid marked selection: %w", err))
		return
	}
	c.preedit = &preedit{original: original, selected: selected}
	if err := c.refreshPreview(); err != nil {
		c.session.CancelComposition()
		c.preedit = nil
		c.previewFrame = nil
		c.recordNativeError(err)
		return
	}
	c.setStatus(kernel.OutcomeHandled, false, nil)
}

// UnmarkText commits preedit under Shoreward policy. Unlike a plain UI
// unmark, a declined or failed commit retains preedit for cancellation or
// retry and records the error instead of panicking.
func (c *DocumentController) UnmarkText() {
	if c.preedit == nil {
		return
	}
	outcome, err := c.commitPreedit(nil, nil)
	c.finishNativeOutcome(outcome, err)
}

// CancelComposition discards transient preedit and restores source text.
func (c *DocumentController) CancelComposition() {
	c.cancelComposition()
	c.setStatus(kernel.OutcomeHandled, false, nil)
}

// CommitComposition commits optional final text and selection under policy.
// A nil range replaces the entire preedit. It returns Pass without error when
// policies decline; native wrappers turn that into recorded controller state.
func (c *DocumentController) CommitComposition(
	range16 *UTF16Range,
	text *string,
) (kernel.Outcome, error) {
	return c.commitPreedit(range16, text)
}

func (c *DocumentController) commitPreedit(
	range16 *UTF16Range,
	text *string,
) (kernel.Outcome, error) {
	if c.preedit == nil {
		return kernel.OutcomePass, c.fail(ErrNoComposition)
	}
	if text != nil {
		preview, err := c.updatedPreview(range16, *text)
		if err != nil {
			return kernel.OutcomePass, c.fail(err)
		}
		end := model.StringLenUTF16(preview)
		if err := c.session.UpdateComposition(preview); err != nil {
			return kernel.OutcomePass, c.fail(err)
		}
		c.preedit.selected = UTF16Range{Start: end, End: end}
		if err := c.refreshPreview(); err != nil {
			return kernel.OutcomePass, c.fail(err)
		}
	}
	outcome, err := c.session.CommitComposition()
	if err != nil {
		c.recordError(err)
		c.status = Status{
			Outcome:   kernel.OutcomePass,
			Revision:  c.Runtime().Revision(),
			Composing: true,
			FrameAt:   c.now(),
		}
		return kernel.OutcomePass, err
	}
	if outcome == kernel.OutcomeHandled {
		c.preedit = nil
		c.previewFrame = nil
		c.refreshSource(true)
		c.refreshVisible()
		c.setStatus(outcome, true, nil)
		return outcome, nil
	}
	if err := c.refreshPreview(); err != nil {
		c.recordError(err)
	}
	c.status = Status{
		Outcome:   outcome,
		Revision:  c.Runtime().Revision(),
		Composing: true,
		FrameAt:   c.now(),
	}
	c.lastError = ErrPolicyDeclined
	return outcome, nil
}

func (c *DocumentController) updatedPreview(
	range16 *UTF16Range,
	text string,
) (string, error) {
	composition, ok := c.session.Composition().Preview()
	if !ok || c.preedit == nil {
		return "", ErrNoComposition
	}
	if !model.ValidDocumentText(text) {
		return "", fmt.Errorf("controller: marked text is not valid document text")
	}
	preview := composition.Text()
	if range16 == nil {
		return text, nil
	}
	if range16.Start < 0 || range16.End < range16.Start {
		return "", ErrInvalidUTF16Range
	}
	marked := UTF16Range{
		Start: c.preedit.original.Start,
		End:   c.preedit.original.Start + model.StringLenUTF16(preview),
	}
	if range16.Start < marked.Start || range16.End > marked.End {
		return "", ErrMarkedRange
	}
	relative := model.NewUTF16Range(
		model.UTF16Offset(range16.Start-marked.Start),
		model.UTF16Offset(range16.End-marked.Start),
	)
	bytes, ok := model.UTF16RangeToByte(preview, relative)
	if !ok {
		return "", ErrInvalidUTF16Range
	}
	updated := preview[:int(bytes.Start)] + text + preview[int(bytes.End):]
	if !model.ValidDocumentText(updated) {
		return "", fmt.Errorf("controller: marked replacement is not valid document text")
	}
	return updated, nil
}

func (c *DocumentController) compositionTarget(
	original UTF16Range,
) (projection.Target, error) {
	if original == c.sourceSelect {
		return c.session.SelectionTarget()
	}
	return c.session.Projection().Target(model.NewUTF16Range(
		model.UTF16Offset(original.Start),
		model.UTF16Offset(original.End),
	))
}

func (c *DocumentController) cancelComposition() {
	c.session.CancelComposition()
	c.preedit = nil
	c.previewFrame = nil
	c.text = c.sourceText
	c.textLength = model.StringLenUTF16(c.text)
}

func (c *DocumentController) finishNativeOutcome(outcome kernel.Outcome, err error) {
	if err != nil {
		c.recordError(err)
		return
	}
	if outcome != kernel.OutcomeHandled {
		c.lastError = ErrPolicyDeclined
	}
}

func (c *DocumentController) recordNativeError(err error) {
	if err == nil {
		return
	}
	c.setStatus(kernel.OutcomePass, false, err)
}

// BoundsForRange delegates to the installed retained-layout provider.
func (c *DocumentController) BoundsForRange(
	range16 UTF16Range,
) (Rect, UTF16Range, bool) {
	if c.geometry == nil {
		return Rect{}, range16, false
	}
	return c.geometry.BoundsForRange(c.text, range16)
}

// IndexForPoint delegates native hit testing to retained layout.
func (c *DocumentController) IndexForPoint(point Point) (int, bool) {
	if c.geometry == nil {
		return 0, false
	}
	return c.geometry.IndexForPoint(c.text, point)
}
