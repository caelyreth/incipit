package documentview

import (
	"errors"
	"time"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

// Services is the clipboard/workspace boundary used by the keymap executor.
// MyGo's Context.Services is adapted to this interface by the window.
type Services interface {
	ReadClipboard() string
	WriteClipboard(string)
	// ClipboardFormats lists what the clipboard holds without reading it,
	// and ReadClipboardData and WriteClipboardData carry pictures and
	// files as well as text.
	ClipboardFormats() []transfer.Format
	ReadClipboardData(formats ...transfer.Format) transfer.Data
	WriteClipboardData(transfer.Data)
}

// Bridge connects one controller to MyGo input, keymap dispatch, and document
// rendering. It is deliberately the only native owner of the controller.
type Bridge struct {
	Controller *controller.DocumentController
	Dispatcher *keymap.Dispatcher
	Services   Services
	External   func(kernel.CommandID) error
	// Images keeps the pictures a paste or a drop brings; without it the
	// page takes text alone.
	Images Images
	// Figures does what a figure's menu asks that the page cannot.
	Figures FigureHost
	// Cells are the cells of a table selected whole, if any are.
	Cells  *CellRange
	Now    func() time.Time
	native *NativeTextClient
	now    time.Time
}

var (
	_ keymap.Executor = (*Bridge)(nil)
	_ TextInput       = (*Bridge)(nil)
)

// NewBridge creates the controller/keymap/native adapter.
func NewBridge(
	document *controller.DocumentController,
	services Services,
	now func() time.Time,
) *Bridge {
	if now == nil {
		now = time.Now
	}
	bridge := &Bridge{
		Controller: document,
		Services:   services,
		Now:        now,
		now:        now(),
	}
	if document != nil {
		bridge.native = &NativeTextClient{Controller: document}
	}
	bridge.Dispatcher = keymap.NewDispatcher(bridge)
	return bridge
}

func (b *Bridge) textClient() ui.TextInputClient {
	if b == nil || b.native == nil {
		return nil
	}
	return b.native
}

// Detach prevents late native text callbacks from editing a page that is no
// longer mounted. A newly mounted page gets its own bridge and client.
func (b *Bridge) Detach() {
	if b != nil && b.native != nil {
		b.native.Controller = nil
	}
}

func (b *Bridge) Configuration() *kernel.PreparedPolicyConfiguration {
	if b == nil || b.Controller == nil {
		return nil
	}
	return b.Controller.Runtime().Configuration()
}

func (b *Bridge) IsComposing() bool {
	return b != nil && b.Controller != nil && b.Controller.IsComposing()
}

func (b *Bridge) CommitComposition() error {
	if b == nil || b.Controller == nil {
		return controller.ErrNoComposition
	}
	_, err := b.Controller.CommitComposition(nil, nil)
	return err
}

func (b *Bridge) CancelComposition() bool {
	if b == nil || b.Controller == nil {
		return false
	}
	composing := b.Controller.IsComposing()
	b.Controller.CancelComposition()
	return composing
}

func (b *Bridge) Execute(action keymap.Action) error {
	if b == nil || b.Controller == nil {
		return errors.New("documentview: bridge requires a controller")
	}
	switch action.Kind {
	case keymap.ActionText:
		outcome, err := b.Controller.Execute(controller.Command{ID: string(action.Text.ID())})
		return editorOutcome(outcome, err)
	case keymap.ActionHistory:
		outcome, err := b.Controller.Execute(controller.Command{ID: string(action.History.ID())})
		return editorOutcome(outcome, err)
	case keymap.ActionMarkup:
		outcome, err := b.Controller.Execute(controller.Command{ID: string(action.Markup.ID())})
		return editorOutcome(outcome, err)
	case keymap.ActionWord:
		outcome, err := b.Controller.Execute(controller.Command{ID: string(action.Word.ID())})
		return editorOutcome(outcome, err)
	case keymap.ActionNesting:
		outcome, err := b.Controller.Execute(controller.Command{ID: string(action.Nesting.ID())})
		return editorOutcome(outcome, err)
	case keymap.ActionNavigation:
		switch action.Navigation {
		case keymap.NavigationTab:
			outcome, err := b.Controller.ExecuteTab()
			return editorOutcome(outcome, err)
		default:
			return errors.New("documentview: unknown navigation action")
		}
	case keymap.ActionClipboard:
		return b.clipboard(action.Clipboard)
	case keymap.ActionSurface:
		return b.surface(action.Surface)
	case keymap.ActionExternal:
		if b.External == nil {
			return nil
		}
		return b.External(action.ID)
	default:
		return nil
	}
}

func (b *Bridge) surface(command editor.SurfaceCommand) error {
	switch command.Kind {
	case editor.SurfaceVisual:
		direction := controller.VerticalUp
		extend := command.ID == editor.SurfaceSelectUpCommandID ||
			command.ID == editor.SurfaceSelectDownCommandID
		if command.ID == editor.SurfaceDownCommandID ||
			command.ID == editor.SurfaceSelectDownCommandID {
			direction = controller.VerticalDown
		}
		return b.Controller.MoveVertical(direction, extend)
	default:
		if b.External == nil {
			return nil
		}
		return b.External(command.ID)
	}
}

// ExecuteExternal routes an app-owned command that is outside the editor's
// closed action set, such as workspace or window commands from the palette.
func (b *Bridge) ExecuteExternal(command kernel.CommandID) error {
	if b == nil || command == "" {
		return errors.New("documentview: external command is required")
	}
	if b.External == nil {
		return nil
	}
	return b.External(command)
}

func (b *Bridge) clipboard(command keymap.ClipboardCommand) error {
	if b.Services == nil {
		return errors.New("documentview: clipboard service unavailable")
	}
	switch command {
	case keymap.ClipboardCopy:
		if b.copyFigure() {
			return nil
		}
		if text, ok := b.cellsText(); ok {
			b.Services.WriteClipboard(text)
			return nil
		}
		text, err := b.Controller.SelectedText()
		if err != nil {
			return err
		}
		b.Services.WriteClipboard(text)
		return nil
	case keymap.ClipboardCut:
		if b.copyFigure() {
			_, err := b.Controller.ReplaceSelection("", kernel.EventOriginSemantic)
			return err
		}
		if text, ok := b.cellsText(); ok {
			b.Services.WriteClipboard(text)
			b.clearCells()
			return nil
		}
		text, err := b.Controller.SelectedText()
		if err != nil {
			return err
		}
		if text != "" {
			outcome, err := b.Controller.ReplaceSelection("", kernel.EventOriginSemantic)
			if err != nil {
				return err
			}
			if outcome != kernel.OutcomeHandled {
				return nil
			}
		}
		b.Services.WriteClipboard(text)
		return nil
	case keymap.ClipboardPaste:
		if pictures := clipboardPictures(b.Services); len(pictures) > 0 && b.Images != nil {
			b.TakeImages(pictures, b.PlaceImages)
			return nil
		}
		text := b.Services.ReadClipboard()
		if b.pasteTable(text) {
			return nil
		}
		outcome, err := b.Controller.ReplaceSelection(text, kernel.EventOriginPaste)
		return editorOutcome(outcome, err)
	default:
		return errors.New("documentview: unknown clipboard action")
	}
}

func (b *Bridge) Select(anchor, head int, at time.Time) error {
	if b == nil || b.Controller == nil {
		return errors.New("documentview: bridge requires a controller")
	}
	return b.Controller.SelectPointer(anchor, head, at)
}

func (b *Bridge) Scroll(float32) {}

func (b *Bridge) Keymap(event ui.InputEvent) bool {
	if b == nil || b.Dispatcher == nil {
		return false
	}
	return b.Dispatcher.HandleInput(event)
}

// Client returns the current native MyGo client.
func (b *Bridge) Client() ui.TextInputClient { return b.textClient() }

func editorOutcome(outcome kernel.Outcome, err error) error {
	if err != nil {
		return err
	}
	if outcome == kernel.OutcomePass {
		return nil
	}
	return nil
}
