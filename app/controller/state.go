package controller

import (
	"fmt"
	"time"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

func (c *DocumentController) refreshSource() {
	projection := c.session.Projection()
	version := projection.Version()
	changed := !c.hasVersion || version != c.version
	if changed {
		c.sourceText = projection.Text()
		c.textLength = model.StringLenUTF16(c.sourceText)
		c.version = version
		c.hasVersion = true
	}
	span, spanErr := c.sourceEditableSpan()
	if spanErr != nil {
		c.recordError(spanErr)
		span = UTF16Range{Start: 0, End: c.textLength}
	}
	c.editableSpan = span
	selection, selectionErr := c.session.VisualSelection()
	if selectionErr != nil {
		c.recordError(selectionErr)
		selection = model.UTF16Range{}
	}
	c.sourceSelect = UTF16Range{
		Start: int(selection.Start),
		End:   int(selection.End),
	}
	c.sourceCaret = int(c.session.Caret().RangeUTF16().Start)
}

func (c *DocumentController) refreshVisible() {
	if c.preedit != nil {
		if err := c.refreshPreview(); err != nil {
			c.recordError(err)
		}
		return
	}
	c.text = c.sourceText
	c.textLength = model.StringLenUTF16(c.text)
	c.previewFrame = nil
}

func (c *DocumentController) refreshPreview() error {
	if c.preedit == nil {
		return ErrNoComposition
	}
	composition, ok := c.session.Composition().Preview()
	if !ok {
		return ErrNoComposition
	}
	// Guides stay hidden while a composition is open: its marked text owns
	// the caret, and no shortcut lands until it commits.
	frame, err := c.session.SurfaceFrame(c.now(), nil)
	if err != nil {
		return err
	}
	previewFrame, err := frame.WithPreview(
		model.NewUTF16Range(
			model.UTF16Offset(c.preedit.original.Start),
			model.UTF16Offset(c.preedit.original.End),
		),
		composition.Text(),
		model.NewUTF16Range(
			model.UTF16Offset(c.preedit.selected.Start),
			model.UTF16Offset(c.preedit.selected.End),
		),
	)
	if err != nil {
		return err
	}
	c.previewFrame = previewFrame
	c.text = previewFrame.Flow().Text()
	c.textLength = int(previewFrame.Flow().UTF16Len())
	return nil
}

func (c *DocumentController) sourceEditableSpan() (UTF16Range, error) {
	span, err := c.session.TextSpan()
	if err != nil {
		return UTF16Range{}, fmt.Errorf("controller: document span has no correspondence: %w", err)
	}
	return UTF16Range{Start: int(span.Start), End: int(span.End)}, nil
}

func (c *DocumentController) setStatus(outcome kernel.Outcome, changed bool, err error) {
	c.status = Status{
		Outcome:   outcome,
		Revision:  c.session.Runtime().Revision(),
		Changed:   changed,
		Composing: c.preedit != nil,
		FrameAt:   c.now(),
	}
	c.lastError = err
}

func (c *DocumentController) recordError(err error) {
	if err != nil {
		c.lastError = err
	}
}

func (c *DocumentController) now() time.Time {
	return time.Now()
}

func (c *DocumentController) fail(err error) error {
	c.setStatus(kernel.OutcomePass, false, err)
	return err
}
