package controller

import (
	"fmt"
	"time"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

const activityLimit = 64

type preedit struct {
	original UTF16Range
	selected UTF16Range
}

// DocumentController owns one editor session and the application-facing
// native text view. It is not safe for concurrent use; MyGo callbacks and
// application commands run on the UI thread.
type DocumentController struct {
	session *editor.Session

	text          string
	textLength    int
	version       projection.Version
	hasVersion    bool
	sourceText    string
	editableSpan  UTF16Range
	sourceSelect  UTF16Range
	sourceCaret   int
	preedit       *preedit
	previewFrame  *surface.Frame
	geometry      Geometry
	metrics       Metrics
	metricsSource MetricSource
	metricsReady  bool
	pendingMetric MetricSource
	metricsQueued bool
	activity      []Activity
	status        Status
	lastError     error
	pending       []Command
}

// NewTextDocument constructs a semantic text document with the complete
// editor policy catalog and mounts a controller on its root node.
func NewTextDocument(text string, clock editor.Clock) (*DocumentController, error) {
	document, err := model.NewDocument("text")
	if err != nil {
		return nil, err
	}
	node := document.Snapshot().Root()
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		return draft.SetTextValue(node, text)
	}); err != nil {
		return nil, err
	}
	catalog, err := editor.EditorPolicyCatalog()
	if err != nil {
		return nil, err
	}
	runtime, err := kernel.NewPolicyRuntime(document, catalog)
	if err != nil {
		return nil, err
	}
	session, err := editor.NewSession(runtime, node, clock)
	if err != nil {
		return nil, err
	}
	return NewDocumentController(session)
}

// NewDocumentController takes ownership of an existing editor session.
func NewDocumentController(session *editor.Session) (*DocumentController, error) {
	if session == nil {
		return nil, fmt.Errorf("controller: session is required")
	}
	controller := &DocumentController{session: session}
	controller.initializeMetrics()
	controller.refreshSource(false)
	controller.refreshVisible()
	controller.setStatus(kernel.OutcomeHandled, false, nil)
	return controller, nil
}

// Session returns the owned engine session. Mutations made directly through
// this view require Refresh before relying on cached controller values.
func (c *DocumentController) Session() *editor.Session { return c.session }

// Runtime returns the session's policy runtime.
func (c *DocumentController) Runtime() *kernel.PolicyRuntime {
	return c.session.Runtime()
}

// Text returns the current visible projected text, including preedit.
func (c *DocumentController) Text() string { return c.text }

// TextLengthUTF16 returns the visible text length in native code units.
func (c *DocumentController) TextLengthUTF16() int { return c.textLength }

// Selection returns the primary visual selection and preserves direction.
func (c *DocumentController) Selection() UTF16Selection {
	if c.preedit != nil {
		start := c.preedit.original.Start + c.preedit.selected.Start
		end := c.preedit.original.Start + c.preedit.selected.End
		return UTF16Selection{Range: UTF16Range{Start: start, End: end}}
	}
	return UTF16Selection{
		Range:    c.sourceSelect,
		Reversed: !c.sourceSelect.Empty() && c.sourceCaret == c.sourceSelect.Start,
	}
}

// Caret returns the active UTF-16 caret offset.
func (c *DocumentController) Caret() int {
	if c.preedit != nil {
		return c.preedit.original.Start + c.preedit.selected.End
	}
	return c.sourceCaret
}

// MarkedRange returns transient preedit in current document coordinates.
func (c *DocumentController) MarkedRange() (UTF16Range, bool) {
	if c.preedit == nil {
		return UTF16Range{}, false
	}
	composition, ok := c.session.Composition().Preview()
	if !ok {
		return UTF16Range{}, false
	}
	return UTF16Range{
		Start: c.preedit.original.Start,
		End:   c.preedit.original.Start + model.StringLenUTF16(composition.Text()),
	}, true
}

// IsComposing reports whether transient preedit is visible.
func (c *DocumentController) IsComposing() bool { return c.preedit != nil }

// EditableRange returns the projected span of the current semantic node.
func (c *DocumentController) EditableRange() UTF16Range {
	span := c.editableSpan
	if c.preedit == nil {
		return span
	}
	marked, ok := c.MarkedRange()
	if !ok {
		return span
	}
	span.End += marked.Length() - c.preedit.original.Length()
	return span
}

// Status returns the latest controller outcome.
func (c *DocumentController) Status() Status {
	c.status.Composing = c.preedit != nil
	c.status.Revision = c.session.Runtime().Revision()
	return c.status
}

// FrameDeadline returns the earliest active animation deadline, if any.
func (c *DocumentController) FrameDeadline() (time.Time, bool) {
	if c.previewFrame != nil {
		return time.Time{}, false
	}
	frame, err := c.session.SurfaceFrame(c.now(), nil)
	if err != nil {
		c.recordError(err)
		return time.Time{}, false
	}
	var earliest time.Time
	found := false
	for _, decoration := range frame.Decorations() {
		deadline, ok := decoration.Deadline()
		if !ok || found && !deadline.Before(earliest) {
			continue
		}
		earliest, found = deadline, true
	}
	return earliest, found
}

// LastError returns the latest non-fatal controller error.
func (c *DocumentController) LastError() error { return c.lastError }

// ClearError removes retained error state without changing the document.
func (c *DocumentController) ClearError() { c.lastError = nil }

// Activity returns retained projection publications in chronological order.
func (c *DocumentController) Activity() []Activity {
	return append([]Activity(nil), c.activity...)
}

// SetGeometry installs retained-layout queries used by native text services.
func (c *DocumentController) SetGeometry(geometry Geometry) { c.geometry = geometry }

// Geometry returns the installed layout query provider.
func (c *DocumentController) Geometry() Geometry { return c.geometry }

// PreviewFrame returns the transient composition frame when one is active.
func (c *DocumentController) PreviewFrame() (*surface.Frame, bool) {
	if c.previewFrame == nil {
		return nil, false
	}
	return c.previewFrame, true
}

// SurfaceFrame returns the current preview frame or the session's retained
// decorated surface frame.
func (c *DocumentController) SurfaceFrame(at time.Time) (*surface.Frame, error) {
	if c.previewFrame != nil {
		return c.previewFrame, nil
	}
	frame, err := c.session.SurfaceFrame(at, nil)
	if err != nil {
		c.recordError(err)
		return nil, err
	}
	return frame, nil
}

// Refresh reconciles controller caches after direct session changes.
func (c *DocumentController) Refresh() {
	c.refreshSource(true)
	c.refreshVisible()
	c.setStatus(kernel.OutcomeHandled, false, nil)
}

// Reconfigure atomically changes editor policies and rebuilds presentation.
// Composition must be finished or cancelled first.
func (c *DocumentController) Reconfigure(operation func(*kernel.PolicyConfiguration) error) error {
	if c.preedit != nil {
		return c.fail(fmt.Errorf("controller: finish composition before changing settings: %w", editor.ErrCompositionActive))
	}
	outcome, err := c.session.Reconfigure(operation)
	if err != nil {
		return c.fail(err)
	}
	c.refreshSource(true)
	c.refreshVisible()
	c.setStatus(kernel.OutcomeHandled, outcome.Changed, nil)
	return nil
}

// Adopt installs a complete source policy configuration.
func (c *DocumentController) Adopt(source kernel.PolicyConfiguration) error {
	if c.Runtime().PolicyConfiguration().Equal(source) {
		return nil
	}
	return c.Reconfigure(func(configuration *kernel.PolicyConfiguration) error {
		*configuration = source.Clone()
		return nil
	})
}
