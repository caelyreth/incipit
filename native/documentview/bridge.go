package documentview

import (
	"errors"
	"time"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/egoist/mygo/ui"
)

// Services is the clipboard/workspace boundary used by the keymap executor.
// MyGo's Context.Services is adapted to this interface by the window.
type Services interface {
	ReadClipboard() string
	WriteClipboard(string)
}

// Bridge connects one controller to MyGo input, keymap dispatch, and document
// rendering. It is deliberately the only native owner of the controller.
type Bridge struct {
	Controller *controller.DocumentController
	Dispatcher *keymap.Dispatcher
	Services   Services
	External   func(kernel.CommandID) error
	Now        func() time.Time
	native     *NativeTextClient
	now        time.Time
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
	case keymap.ActionExternal:
		if b.External == nil {
			return nil
		}
		return b.External(action.ID)
	default:
		return nil
	}
}

func (b *Bridge) clipboard(command keymap.ClipboardCommand) error {
	if b.Services == nil {
		return errors.New("documentview: clipboard service unavailable")
	}
	switch command {
	case keymap.ClipboardCopy:
		text, err := b.Controller.SelectedText()
		if err != nil {
			return err
		}
		b.Services.WriteClipboard(text)
		return nil
	case keymap.ClipboardCut:
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
		text := b.Services.ReadClipboard()
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
