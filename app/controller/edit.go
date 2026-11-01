package controller

import (
	"errors"
	"fmt"
	"time"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

// Replace performs one projected edit in an optional native range. A nil
// range uses the current selection. A policy Pass is returned without error;
// native input entry points record it as a controller error.
func (c *DocumentController) Replace(
	range16 *UTF16Range,
	text string,
	origin kernel.EventOrigin,
) (kernel.Outcome, error) {
	c.resetPreferredColumn()
	if c.preedit != nil {
		return kernel.OutcomePass, c.fail(editor.ErrCompositionActive)
	}
	if !model.ValidDocumentText(text) {
		return kernel.OutcomePass, c.fail(fmt.Errorf("controller: replacement is not valid document text"))
	}
	var (
		result editor.EditingOutcome
		err    error
	)
	// Only nil names the live selection. An explicit range, collapsed or
	// not, is the caller's exact intent.
	if range16 == nil {
		result, err = c.session.ReplaceSelection(text, origin)
	} else {
		normalized := range16.Normalized()
		if *range16 != normalized {
			return kernel.OutcomePass, c.fail(ErrInvalidUTF16Range)
		}
		target, targetErr := c.session.Projection().Target(model.NewUTF16Range(
			model.UTF16Offset(range16.Start),
			model.UTF16Offset(range16.End),
		))
		if targetErr != nil {
			return kernel.OutcomePass, c.fail(fmt.Errorf("%w: %v", ErrInvalidUTF16Range, targetErr))
		}
		edit, editErr := editor.FromProjectionTarget(c.session.Projection(), target, text, origin)
		if editErr != nil {
			return kernel.OutcomePass, c.fail(editErr)
		}
		result, err = c.session.ApplyProjectedEdit(edit)
	}
	if err != nil {
		return kernel.OutcomePass, c.fail(err)
	}
	if result.Outcome == kernel.OutcomeHandled {
		c.refreshSource(true)
		c.refreshVisible()
	}
	c.setStatus(result.Outcome, !result.Changes.Empty(), nil)
	return result.Outcome, nil
}

// ReplaceSelection applies text at the current selection.
func (c *DocumentController) ReplaceSelection(
	text string,
	origin kernel.EventOrigin,
) (kernel.Outcome, error) {
	return c.Replace(nil, text, origin)
}

// SelectedText returns the visible text covered by the primary selection.
func (c *DocumentController) SelectedText() (string, error) {
	selection := c.Selection().Range
	if selection.Empty() {
		return "", nil
	}
	aligned, _, ok := model.AlignUTF16RangeForNativeQuery(
		c.text,
		model.NewUTF16Range(
			model.UTF16Offset(selection.Start),
			model.UTF16Offset(selection.End),
		),
	)
	if !ok {
		return "", ErrInvalidUTF16Range
	}
	if int(aligned.Start) != selection.Start || int(aligned.End) != selection.End {
		return "", c.fail(fmt.Errorf("%w: selection does not align to text", ErrInvalidUTF16Range))
	}
	if c.preedit != nil {
		bytes, ok := model.UTF16RangeToByte(c.text, aligned)
		if !ok {
			return "", c.fail(ErrInvalidUTF16Range)
		}
		return c.text[bytes.Start:bytes.End], nil
	}
	// The page's text as a reader takes it: an atomic block's object is
	// no text.
	return c.session.ReadableText(aligned)
}

// SelectSemantic installs a version-valid semantic selection.
func (c *DocumentController) SelectSemantic(selection model.TextSelection) error {
	c.resetPreferredColumn()
	if c.preedit != nil {
		return c.fail(editor.ErrCompositionActive)
	}
	if err := c.session.Select(selection); err != nil {
		return c.fail(err)
	}
	c.refreshSource(false)
	c.refreshVisible()
	c.setStatus(kernel.OutcomeHandled, false, nil)
	return nil
}

// SelectProjected installs a directed projected selection.
func (c *DocumentController) SelectProjected(anchor, head int) error {
	c.resetPreferredColumn()
	return c.selectProjected(anchor, head)
}

func (c *DocumentController) selectProjected(anchor, head int) error {
	if c.preedit != nil {
		return c.fail(editor.ErrCompositionActive)
	}
	if anchor < 0 || head < 0 {
		return c.fail(ErrInvalidUTF16Range)
	}
	if err := c.session.SelectProjected(
		model.UTF16Offset(anchor),
		model.UTF16Offset(head),
	); err != nil {
		return c.fail(err)
	}
	c.refreshSource(false)
	c.refreshVisible()
	c.setStatus(kernel.OutcomeHandled, false, nil)
	return nil
}

// SelectPointer installs a pointer-derived projected selection and updates
// Neverland's return-to-edit navigation offer.
func (c *DocumentController) SelectPointer(anchor, head int, at time.Time) error {
	c.resetPreferredColumn()
	if c.preedit != nil {
		return c.fail(editor.ErrCompositionActive)
	}
	if anchor < 0 || head < 0 {
		return c.fail(ErrInvalidUTF16Range)
	}
	if err := c.session.SelectProjected(
		model.UTF16Offset(anchor),
		model.UTF16Offset(head),
	); err != nil {
		return c.fail(err)
	}
	c.refreshSource(false)
	c.refreshVisible()
	if node, ok := c.Runtime().Snapshot().Document().Node(c.session.Selection().Node()); ok {
		if err := c.session.Navigation().SelectPointer(
			node.Text().String(),
			c.session.Selection(),
			at,
		); err != nil {
			return c.fail(err)
		}
	}
	c.sourceCaret = int(c.session.Caret().RangeUTF16().Start)
	c.setStatus(kernel.OutcomeHandled, false, nil)
	return nil
}

// SetSelection installs a normalized visual range or edits the selected part
// of active marked text.
func (c *DocumentController) SetSelection(range16 UTF16Range) error {
	if range16.Start < 0 || range16.End < range16.Start {
		return c.fail(ErrInvalidUTF16Range)
	}
	if c.preedit != nil {
		marked, ok := c.MarkedRange()
		if !ok || range16.Start < marked.Start || range16.End > marked.End {
			return c.fail(ErrMarkedRange)
		}
		relative := UTF16Range{
			Start: range16.Start - marked.Start,
			End:   range16.End - marked.Start,
		}
		if err := c.validateTextRange(c.text, relative); err != nil {
			return c.fail(err)
		}
		c.preedit.selected = relative
		if err := c.refreshPreview(); err != nil {
			return c.fail(err)
		}
		c.setStatus(kernel.OutcomeHandled, false, nil)
		return nil
	}
	return c.SelectProjected(range16.Start, range16.End)
}

func (c *DocumentController) validateTextRange(text string, range16 UTF16Range) error {
	_, ok := model.UTF16RangeToByte(
		text,
		model.NewUTF16Range(
			model.UTF16Offset(range16.Start),
			model.UTF16Offset(range16.End),
		),
	)
	if !ok {
		return ErrInvalidUTF16Range
	}
	return nil
}

// IsCompositionActiveError reports whether err is the engine's composition
// gate. It is a convenience for application command routing.
func IsCompositionActiveError(err error) bool {
	return errors.Is(err, editor.ErrCompositionActive)
}
