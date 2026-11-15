package shell

import (
	"time"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/scrollfx"
	"github.com/egoist/mygo/ui"
)

// browser is one list of sheets as it is browsed: the place it shows or
// the search it runs, its rows as they came in, what is picked in it and
// what is dragged over it. The sheet list is one; each list is its own, so
// one can be drawn as many times as there are lists, the library's places
// and its names (navigation) shared by them all.
type browser struct {
	place Place
	// searching shows search results instead of the place; query is the
	// search as written, and find the search's own state (search.go).
	searching bool
	query     string
	find      finding
	// shown is what the list shows, since shownAt while its rows come in;
	// fresh marks the frame it changed, and rank counts the rows drawn so
	// far in this one.
	shown   listView
	shownAt time.Time
	fresh   bool
	rank    int
	// side is where the list shown came in from: 1 the right, -1 the left,
	// 0 neither.
	side float32
	// scroll and overscroll are where the list is scrolled to.
	scroll     ui.ScrollState
	overscroll scrollfx.Overscroll
	// picking is the sheets picked together in the list.
	picking picking
	// menuRing is what a menu open on the list acts on.
	menuRing menuRing
	// sheetsGap calls up a create area above a sheet in a group.
	sheetsGap components.CreateGap
	// revealing is a chapter just made, to bring into view as it comes.
	revealing string
	// sheetsNear is the sheet whose top the pointer was by, -1 for none.
	sheetsNear int
	// sheetDropSlot is the insertion slot while a sheet is dragged over a
	// group's list, -1 when none; sheetDropRows is the last geometry used
	// to choose it, and sheetDropHeight is the dragged row's extent;
	// sheetDropRoom is the room a chapter takes above it where it lands.
	sheetDropSlot    int
	sheetDropHeight  float32
	sheetDropRoom    float32
	sheetDropVisible bool
	sheetDropRows    []sheetDropSpan
	// grab is where the pointer took the row it drags, landing the row a
	// drop is bringing to its place, and listTop the list's top as last
	// laid out.
	grab    grab
	landing landing
	listTop float32
}

func newBrowser(place Place) browser {
	return browser{place: place, sheetDropSlot: -1}
}

// listing is a browser as the window draws it: what a list does it does
// on its own browser, with the library and the window's state it shares.
// Its value is what a row's actions keep, so a click acts on the list it
// was drawn in.
type listing struct {
	*State
	browser *browser
}

// sheets is the sheet list's browser, as the window draws it.
func (s *State) sheets() listing { return listing{s, &s.list} }
