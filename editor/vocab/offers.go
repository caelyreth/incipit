package vocab

// Offers are the commands a policy puts forward for a selection, beside
// running them on their keys: each in a group, which a host draws in the
// surfaces the group names, as a bar over the selection, the writing
// dock, the page's menu or the menu bar. What is offered, whether it can
// run and whether it is on are the rules' answers (policy.Offer,
// policy.State); a host draws them and knows nothing of what they do.

// OfferShape is how a group offers its commands.
type OfferShape uint8

const (
	// OfferToggles are commands each turning something on or off.
	OfferToggles OfferShape = iota + 1
	// OfferChoice is one of its commands, the one on, a menu of them all.
	OfferChoice
	// OfferActions are commands each doing something once.
	OfferActions
)

// Surface is a set of the places a host draws offers in.
type Surface uint8

const (
	// SurfaceTools is the bar standing over the selection.
	SurfaceTools Surface = 1 << iota
	// SurfaceDock is the writing dock under the page.
	SurfaceDock
	// SurfaceMenu is the page's menu, of what is under the pointer or
	// selected.
	SurfaceMenu
	// SurfaceMenuBar is the app's menu bar.
	SurfaceMenuBar
)

// OfferGroup is a group of offers: a phrase key naming it, its shape,
// the key that opens a choice while its surface reads keys, its place
// among groups, and the surfaces it shows in.
type OfferGroup struct {
	ID    string
	Label string
	Shape OfferShape
	Key   string
	Order int
	In    Surface
}

// SelectionOffers grow and shrink the selection, whatever the profile.
var SelectionOffers = OfferGroup{ID: "selection", Label: "offers.selection", Shape: OfferActions,
	Order: 40, In: SurfaceTools}
