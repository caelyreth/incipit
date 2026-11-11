package profile

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
)

// The product's feature intents: what its commands, menus and controls
// ask for. Each is answered by the policy owning the feature; with that
// policy disabled, the intent does nothing.
var (
	intentFormat     = vocab.NewIntentKind("Format")
	intentRetype     = vocab.NewIntentKind("Retype")
	intentWrapQuote  = vocab.NewIntentKind("WrapQuote")
	intentMakeList   = vocab.NewIntentKind("MakeList")
	intentNest       = vocab.NewIntentKind("Nest")
	intentToggleTask = vocab.NewIntentKind("ToggleTask")
)

type (
	// Format toggles a mark over the selection, or the marks text typed at
	// a caret takes.
	Format struct{ Mark doc.MarkKind }
	// Retype gives the selected text blocks a kind and its attributes; the
	// blocks that already have them become paragraphs again.
	Retype struct {
		To    doc.Kind
		Attrs doc.Attrs
	}
	// WrapQuote wraps the selected blocks in a quote.
	WrapQuote struct{}
	// MakeList makes each selected block an item of a list with attrs.
	MakeList struct{ Attrs doc.Attrs }
	// Nest moves the caret's item in (Forward) or out.
	Nest struct{ Dir vocab.Direction }
	// ToggleTask completes or reopens a task: item ID, or the caret's.
	ToggleTask struct{ ID doc.ID }
)

func (Format) Kind() vocab.IntentKind     { return intentFormat }
func (Retype) Kind() vocab.IntentKind     { return intentRetype }
func (WrapQuote) Kind() vocab.IntentKind  { return intentWrapQuote }
func (MakeList) Kind() vocab.IntentKind   { return intentMakeList }
func (Nest) Kind() vocab.IntentKind       { return intentNest }
func (ToggleTask) Kind() vocab.IntentKind { return intentToggleTask }
