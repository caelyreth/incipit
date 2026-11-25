package profile

import "github.com/caelyreth/neverland/internal/editor/vocab"

// The profile's offer groups (vocab.OfferGroup), which its policies put
// their commands forward in; the selection's own are vocab's
// (vocab.SelectionOffers).
var (
	// LookUpOffers look up what the text selected, or the word at the
	// caret, means: first in the page's menu, as the system's are.
	LookUpOffers = vocab.OfferGroup{ID: "lookUp", Label: "offers.lookUp", Shape: vocab.OfferActions,
		Order: 1, In: vocab.SurfaceMenu}
	// InsertOffers are what is inserted at the selection.
	InsertOffers = vocab.OfferGroup{ID: "insert", Label: "offers.insert", Shape: vocab.OfferActions,
		Order: 10, In: vocab.SurfaceDock | vocab.SurfaceMenuBar}
	// KindOffers are the kinds the blocks selected turn into.
	KindOffers = vocab.OfferGroup{ID: "kind", Label: "offers.kind", Shape: vocab.OfferChoice, Key: "t",
		Order: 20, In: vocab.SurfaceTools | vocab.SurfaceDock | vocab.SurfaceMenuBar}
	// MarkOffers are the marks of the text selected.
	MarkOffers = vocab.OfferGroup{ID: "marks", Label: "offers.marks", Shape: vocab.OfferToggles,
		Order: 30, In: vocab.SurfaceTools | vocab.SurfaceDock | vocab.SurfaceMenuBar}
	// PictureOffers use a figure's picture, PictureReplaceOffers replace
	// it, and PictureAlignOffers set it in the page.
	PictureOffers = vocab.OfferGroup{ID: "picture", Label: "offers.picture", Shape: vocab.OfferActions,
		Order: 50, In: vocab.SurfaceMenu}
	PictureReplaceOffers = vocab.OfferGroup{ID: "picture.replace", Label: "offers.picture", Shape: vocab.OfferActions,
		Order: 51, In: vocab.SurfaceMenu}
	PictureAlignOffers = vocab.OfferGroup{ID: "picture.align", Label: "offers.pictureAlign", Shape: vocab.OfferActions,
		Order: 52, In: vocab.SurfaceMenu}
	// TableLineOffers add a table's rows and columns, TableFormatOffers
	// set their alignment and header, and TableRemoveOffers remove them or
	// the table.
	TableLineOffers = vocab.OfferGroup{ID: "table.lines", Label: "offers.tableLines", Shape: vocab.OfferActions,
		Order: 60, In: vocab.SurfaceMenu}
	TableFormatOffers = vocab.OfferGroup{ID: "table.format", Label: "offers.tableFormat", Shape: vocab.OfferActions,
		Order: 61, In: vocab.SurfaceMenu}
	TableRemoveOffers = vocab.OfferGroup{ID: "table.remove", Label: "offers.tableRemove", Shape: vocab.OfferActions,
		Order: 62, In: vocab.SurfaceMenu}
	// FormulaOffers change a formula's notation.
	FormulaOffers = vocab.OfferGroup{ID: "formula", Label: "offers.formula", Shape: vocab.OfferActions,
		Order: 70, In: vocab.SurfaceMenu}
)
