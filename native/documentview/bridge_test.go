package documentview

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
)

type memoryClipboard struct{ text string }

func (c *memoryClipboard) ReadClipboard() string      { return c.text }
func (c *memoryClipboard) WriteClipboard(text string) { c.text = text }

type bridgeVerticalGeometry struct{}

func (bridgeVerticalGeometry) BoundsForRange(
	_ string,
	range16 controller.UTF16Range,
) (controller.Rect, controller.UTF16Range, bool) {
	return controller.Rect{W: 1, H: 10}, range16, true
}

func (bridgeVerticalGeometry) IndexForPoint(
	_ string,
	point controller.Point,
) (int, bool) {
	if point.Y >= 10 {
		return 1, true
	}
	return 0, true
}

func (bridgeVerticalGeometry) CaretLine(
	_ string,
	caret int,
) (controller.Rect, bool) {
	if caret != 0 {
		return controller.Rect{}, false
	}
	return controller.Rect{Y: 0, W: 1, H: 10}, true
}

func bridgeFixture(t *testing.T, text string) (*Bridge, *memoryClipboard) {
	t.Helper()
	document, err := controller.NewTextDocument(text, nativeClock{})
	if err != nil {
		t.Fatal(err)
	}
	clipboard := &memoryClipboard{}
	bridge := NewBridge(document, clipboard, func() time.Time {
		return time.Unix(1_700_000_000, 0)
	})
	return bridge, clipboard
}

func TestBridgeRoutesTypedEditorActions(t *testing.T) {
	bridge, _ := bridgeFixture(t, "abc")
	action, ok := keymap.ResolveEditorCommand("text.document-end")
	if !ok {
		t.Fatal("command missing")
	}
	if err := bridge.Execute(action); err != nil {
		t.Fatal(err)
	}
	if bridge.Controller.Caret() != 3 {
		t.Fatalf("caret = %d", bridge.Controller.Caret())
	}
}

func TestBridgeRoutesVisualSurfaceActionsThroughController(t *testing.T) {
	bridge, _ := bridgeFixture(t, "ab")
	bridge.Controller.SetGeometry(bridgeVerticalGeometry{})
	action, ok := keymap.ResolveEditorCommand(editor.SurfaceDownCommandID)
	if !ok || action.Kind != keymap.ActionSurface {
		t.Fatalf("surface action = %#v, %v", action, ok)
	}
	if err := bridge.Execute(action); err != nil {
		t.Fatal(err)
	}
	if bridge.Controller.Caret() != 1 {
		t.Fatalf("vertical caret = %d", bridge.Controller.Caret())
	}
}

func TestBridgeClipboardCopyCutPaste(t *testing.T) {
	bridge, clipboard := bridgeFixture(t, "abc")
	bridge.Controller.SelectProjected(0, 2)
	if err := bridge.Execute(keymap.Action{
		Kind:      keymap.ActionClipboard,
		Clipboard: keymap.ClipboardCopy,
	}); err != nil {
		t.Fatal(err)
	}
	if clipboard.text != "ab" {
		t.Fatalf("copy = %q", clipboard.text)
	}
	if err := bridge.Execute(keymap.Action{
		Kind:      keymap.ActionClipboard,
		Clipboard: keymap.ClipboardCut,
	}); err != nil {
		t.Fatal(err)
	}
	if bridge.Controller.Text() != "c" || clipboard.text != "ab" {
		t.Fatalf("cut text=%q clipboard=%q", bridge.Controller.Text(), clipboard.text)
	}
	if err := bridge.Execute(keymap.Action{
		Kind:      keymap.ActionClipboard,
		Clipboard: keymap.ClipboardPaste,
	}); err != nil {
		t.Fatal(err)
	}
	if bridge.Controller.Text() != "abc" {
		t.Fatalf("paste text = %q", bridge.Controller.Text())
	}
}

func TestBridgeRejectsExternalWithoutHost(t *testing.T) {
	bridge, _ := bridgeFixture(t, "abc")
	err := bridge.Execute(keymap.Action{
		Kind: keymap.ActionExternal,
		ID:   kernel.CommandID("workspace.new"),
	})
	if err != nil {
		t.Fatalf("external command without host = %v", err)
	}
}
