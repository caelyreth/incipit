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
	intentPick       = vocab.NewIntentKind("PickPictures")
	intentPlace      = vocab.NewIntentKind("PlaceFigures")
	intentResize     = vocab.NewIntentKind("ResizeFigure")
	intentAlign      = vocab.NewIntentKind("AlignFigure")
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
	// PickPictures asks the host for pictures to place where the selection
	// is; the host answers with PlaceFigures once it keeps them.
	PickPictures struct{}
	// PlaceFigures places kept pictures as figures, in order: at At, a
	// caret where they were dropped, or else where the selection is.
	PlaceFigures struct {
		Pictures []Picture
		At       *doc.Caret
	}
	// ResizeFigure sets a figure's span, as a resize gesture ends.
	ResizeFigure struct {
		ID   doc.ID
		Span int
	}
	// AlignFigure aligns the selected figure.
	AlignFigure struct{ To Align }
)

func (Format) Kind() vocab.IntentKind       { return intentFormat }
func (Retype) Kind() vocab.IntentKind       { return intentRetype }
func (WrapQuote) Kind() vocab.IntentKind    { return intentWrapQuote }
func (MakeList) Kind() vocab.IntentKind     { return intentMakeList }
func (Nest) Kind() vocab.IntentKind         { return intentNest }
func (ToggleTask) Kind() vocab.IntentKind   { return intentToggleTask }
func (PickPictures) Kind() vocab.IntentKind { return intentPick }
func (PlaceFigures) Kind() vocab.IntentKind { return intentPlace }
func (ResizeFigure) Kind() vocab.IntentKind { return intentResize }
func (AlignFigure) Kind() vocab.IntentKind  { return intentAlign }
