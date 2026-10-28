package editor

import (
	"errors"
	"time"
	"unicode"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

const (
	// HistoryLimit bounds each history stack, matching Neverland's 200-frame
	// undo depth.
	HistoryLimit = 200
	// TypingGroupWindow is the maximum gap between contiguous typing edits
	// that may share an undo frame.
	TypingGroupWindow = 750 * time.Millisecond
)

// ErrHistoryRestoreRequired reports that a history transition had no
// transaction application callback.
var ErrHistoryRestoreRequired = errors.New("editor: history restore callback is required")

// Frame is the complete data needed to restore one editor history position.
//
// Node is captured by value from an immutable document snapshot. Restoring it
// with DocumentDraft.RestoreContent preserves semantic content, inline mark
// identities, and the node's monotonic mark-identity supply.
type Frame struct {
	node      model.Node
	selection model.TextSelection
	visual    projection.Selection
	active    *model.MarkID
}

// NewFrame captures one history position. It rejects a nil node so callers
// cannot accidentally record an unusable frame.
func NewFrame(
	node *model.Node,
	selection model.TextSelection,
	visual projection.Selection,
	active *model.MarkID,
) (Frame, bool) {
	if node == nil {
		return Frame{}, false
	}
	frame := Frame{
		node:      *node,
		selection: selection,
		visual:    visual,
	}
	if active != nil {
		value := *active
		frame.active = &value
	}
	return frame, true
}

// Node returns a copy of the immutable node snapshot captured by the frame.
func (f Frame) Node() model.Node {
	return f.node
}

// Selection returns the semantic selection captured by the frame.
func (f Frame) Selection() model.TextSelection {
	return f.selection
}

// Visual returns the projected selection captured by the frame.
func (f Frame) Visual() projection.Selection {
	return f.visual
}

// ActiveMark returns the active inline mark, if one was captured.
func (f Frame) ActiveMark() (model.MarkID, bool) {
	if f.active == nil {
		return 0, false
	}
	return *f.active, true
}

type historyGroup struct {
	visual projection.Selection
	at     time.Time
}

// History owns the bounded undo and redo stacks and the current typing group.
// It is deliberately independent from document transactions: Undo and Redo
// accept a restore callback, and a stack movement commits only after that
// callback succeeds.
type History struct {
	clock Clock
	undo  []Frame
	redo  []Frame
	group *historyGroup
}

// NewHistory creates an empty history using the supplied clock. A nil clock
// uses SystemClock.
func NewHistory(clock Clock) *History {
	return &History{clock: clockOrSystem(clock)}
}

// BreakGroup ends the current typing group. Selection changes, configuration
// changes, and other non-typing edits should call this method.
func (h *History) BreakGroup() {
	h.group = nil
}

// SelectionChanged ends typing grouping after a selection change.
func (h *History) SelectionChanged() {
	h.BreakGroup()
}

// ConfigurationChanged ends typing grouping after a policy or settings change.
func (h *History) ConfigurationChanged() {
	h.BreakGroup()
}

// CanUndo reports whether an undo frame is available.
func (h *History) CanUndo() bool {
	return len(h.undo) > 0
}

// CanRedo reports whether a redo frame is available.
func (h *History) CanRedo() bool {
	return len(h.redo) > 0
}

// UndoDepth returns the retained undo frame count.
func (h *History) UndoDepth() int {
	return len(h.undo)
}

// RedoDepth returns the retained redo frame count.
func (h *History) RedoDepth() int {
	return len(h.redo)
}

// Record adds a before frame after a successful change.
//
// before is the state prior to the edit. afterVisual is the projected selection
// after the edit and seeds the next grouping comparison. group should be true
// only for a single alphanumeric typing insertion; ShouldGroupTyping computes
// that condition. A no-op (changed false) never records a frame and always
// breaks the current group. Any non-joinable edit clears redo.
func (h *History) Record(before Frame, afterVisual projection.Selection, changed, group bool) {
	h.recordAt(before, afterVisual, changed, group, h.clock.Now())
}

func (h *History) recordAt(
	before Frame,
	afterVisual projection.Selection,
	changed bool,
	group bool,
	now time.Time,
) {
	if !changed {
		h.BreakGroup()
		return
	}
	join := group &&
		h.group != nil &&
		h.group.visual == before.visual &&
		now.Sub(h.group.at) < TypingGroupWindow
	if !join {
		h.undo = append(h.undo, before)
		if len(h.undo) > HistoryLimit {
			h.undo = h.undo[len(h.undo)-HistoryLimit:]
		}
	}
	h.redo = h.redo[:0]
	if group {
		h.group = &historyGroup{visual: afterVisual, at: now}
	} else {
		h.group = nil
	}
}

// ShouldGroupTyping reports whether an edit may join an existing typing group.
// Only one Alphabetic-or-Numeric Unicode scalar inserted by a typing-origin
// text edit qualifies, matching Rust's char::is_alphanumeric.
func ShouldGroupTyping(origin kernel.EventOrigin, appliesText bool, replacement string) bool {
	if origin != kernel.EventOriginTyping || !appliesText {
		return false
	}
	runes := []rune(replacement)
	if len(runes) != 1 {
		return false
	}
	r := runes[0]
	// Alphabetic is L plus Nl plus Other_Alphabetic, while Numeric is N.
	// IsNumber supplies N (including Nl); Other_Alphabetic adds combining
	// marks such as U+0345 and U+093E that general category alone omits.
	return unicode.IsLetter(r) ||
		unicode.IsNumber(r) ||
		unicode.Is(unicode.Other_Alphabetic, r)
}

// RestoreFunc applies a history frame to the caller-owned editor state. The
// stack movement is committed only when it returns nil.
type RestoreFunc func(Frame) error

// Undo restores the newest undo frame. It returns false without error when no
// undo frame exists. A failed restore leaves both stacks unchanged.
func (h *History) Undo(current Frame, restore RestoreFunc) (bool, error) {
	if len(h.undo) == 0 {
		return false, nil
	}
	source := h.undo[len(h.undo)-1]
	if restore == nil {
		return false, ErrHistoryRestoreRequired
	}
	if err := restore(source); err != nil {
		return false, err
	}
	h.undo = h.undo[:len(h.undo)-1]
	h.redo = append(h.redo, current)
	h.BreakGroup()
	return true, nil
}

// Redo restores the newest redo frame. It returns false without error when no
// redo frame exists. A failed restore leaves both stacks unchanged.
func (h *History) Redo(current Frame, restore RestoreFunc) (bool, error) {
	if len(h.redo) == 0 {
		return false, nil
	}
	source := h.redo[len(h.redo)-1]
	if restore == nil {
		return false, ErrHistoryRestoreRequired
	}
	if err := restore(source); err != nil {
		return false, err
	}
	h.redo = h.redo[:len(h.redo)-1]
	h.undo = append(h.undo, current)
	if len(h.undo) > HistoryLimit {
		h.undo = h.undo[len(h.undo)-HistoryLimit:]
	}
	h.BreakGroup()
	return true, nil
}
