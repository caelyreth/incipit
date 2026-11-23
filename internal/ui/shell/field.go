package shell

import (
	"slices"
	"strings"
	"time"

	"github.com/caelyreth/neverland/internal/text/query"
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/motion"
	"github.com/egoist/mygo/ui"
)

// The field shows what its sources find for its query (cards.go), the
// library's in blocks (results.go, blocks.go), around the pill the query
// is written in (pill.go).
// Everything done on the field changes the query, and the field moves to
// what the changed query finds on one value (stageShift), from where the
// action says: a group's head pressed opens its blocks out of it, a
// condition taken away folds them back into the block it came from.

type fieldState struct {
	// rail is the list's dividers, railShown the one its card last named,
	// and railScroll how far the rail is scrolled (rail.go).
	rail       []railStop
	railShown  int
	railScroll ui.ScrollState
	// conditions are the query's conditions, each as the language writes
	// it, shown as ribbons on the pill; words is what is written after
	// them, and edit what the pill's input is to be set to.
	conditions []string
	words      string
	edit       *string
	// focus asks the pill's input for the keyboard; open is whether the
	// pill, holding it at its horizon, shows what it can narrow by.
	focus bool
	// quiet has the field take the keyboard itself, the pill let go of.
	quiet bool
	open  bool
	// acting is whether the place's actions stand open beside the pill
	// (actions.go), actingShown how far, and actionAt the one the keys
	// are on.
	acting      bool
	actingShown float32
	actionAt    int
	// notice is what was just done, to be undone (notice.go); told is how
	// far it is shown, and lastNotice its words as they fade.
	notice *notice
	// history is the places the field has been, at the one it is at
	// (history.go).
	history struct {
		visits []visit
		at     int
	}
	// undos are the changes done, the last last, and redos those taken
	// back, to be done again (notice.go).
	undos, redos []undoEntry
	told         float32
	lastNotice   string
	// completion is what may stand at the caret, caret bytes into the
	// words completedFor, offered unless the words are what it was put
	// away at, dismissed, suggestion the one of it the keys are on (-1 for
	// none), and allValues whether every value stands, not just the first.
	completion query.Completion
	caret      int
	suggestion int
	allValues  bool
	// allKeys is whether every key is offered, not just the first few.
	allKeys bool
	// opened is the key open in the pill, its values standing around it,
	// and held what was written before it opened.
	opened       *query.Key
	held         string
	completedFor string
	dismissed    string
	found        found
	// rerank has found ranked again, as the window comes back to the
	// field; settleAt is when the field settles as the writing in the pill
	// pauses, zero while nothing is being written, and settled that it
	// has, for the field to stand even again.
	rerank, settled bool
	settleAt        time.Time
	// shapes are the cards' shapes, by id and look, for the library and
	// the measures shapesKey names.
	shapes    map[shapeID]shape
	shapesKey shapesKey
	// shown are the cards drawn, those going out included, by id, as
	// shaped.
	shown      map[string]card
	shownShape map[string]shape
	// rest is the layout's frames as last laid out, at rest, drawn what
	// was last drawn, and from what the change under way started from.
	rest, drawn, from []stageFrame
	// next is how the next change moves, as the action making it says,
	// and now the change under way.
	next, now change
	// scroll is how far the field is scrolled down from its horizon at
	// rest (up, below 0), as far as lo and hi; home brings it back there,
	// homing once it has begun.
	scroll, lo, hi float32
	// poise is how far the field stands from the window's middle at rest,
	// as drawn: its cards across, its horizon up; and poiseTo as it goes
	// to (fieldView).
	poise, poiseTo stagePoint
	home, homing   bool
	// homeTo is where the field goes as it comes home: its horizon, 0,
	// or where something arriving stands in view (order.go).
	homeTo float32
	// arrivals are what was just made, moved or put back, by row or card
	// id, from when the field first showed each (order.go).
	arrivals map[string]arrival
	// away is how far what the pill offers around it is put away, as its
	// horizon goes past where it all fits.
	away float32
	// measured is how tall each card of the settings laid itself out, by
	// its section, and measures counts the changes.
	measured map[string]float32
	measures uint64
	// over is how far past its ends the field is pulled, as the panels and
	// tables pull, and viewH the field's height, for its reach.
	over  motion.Elastic
	viewH float32
	edges components.EdgeFade
	// stuck is how far the pill has gone into staying at the window's top
	// or foot, 0 to 1, which its backdrop comes in with; stuckFoot keeps
	// which edge it stood at while fading out, and backdrop / backdropSpan
	// record its active height and held ribbon scrim extent.
	stuck        float32
	stuckFoot    bool
	backdrop     float32
	backdropSpan float32
	// room is the room the field was last laid out in.
	room fieldRoom
	// ribbonHold is what the ribbons about the pill hold to while it
	// stands as it is (pill.go).
	ribbonHold ribbonHold
	// ribbonItems is the ribbons as they are flowed, kept to be filled
	// again each frame.
	ribbonItems []ribbonPlace
	// kept is the kept box's shelf; keptRows its bookmarks as it last
	// showed them, those going among them, and keptH each one's height.
	kept     shelfState
	keptRows []keptRow
	keptH    map[string]float32
	// stats animates the current field counts beside the search pill.
	stats fieldStatsState
	// hidden is whether the desk fills the window over the field, which
	// takes no focus meanwhile.
	hidden bool
	// hold keeps a change still for its first frame, which lays out and
	// draws what it brings, so the motion starts on the frame after.
	hold bool
	// picked is the groups and sheets selected, anchor where a
	// Shift-click's run starts, and pickedFor the query they were
	// selected under (picks.go).
	picked    []string
	anchor    string
	pickedFor string
	// gathering keeps what moves done together would undo, while they are
	// (notice.go).
	gathering *[]undoEntry
	// drag is what was dragged as the last frame drew, dragNext what is as
	// this one does, and spring the group it is held over (drop.go).
	drag, dragNext shellDrag
	spring         springing
	// ranked is the rows the field shows in its rank, order the same as
	// they are read on the screen, and chosen the one the keys are on;
	// rowRects where each stands in the field, drawing the card being
	// drawn, and shownChosen the row last brought into view (keys.go).
	ranked      []string
	order       []string
	chosen      string
	rowRects    map[string]stageRect
	drawing     cardPlace
	shownChosen string
	// shownRows is order as a set, kept to be filled again each frame.
	shownRows map[string]bool
	// wordsBefore is the pill's words as the last frame left them, for
	// Space on a row to be taken back from them.
	wordsBefore string
	// renaming is the group whose name is being written on its head.
	renaming, renameName string
	renameFocus          bool
	// lists are the places shown as a list, kept for the session (list.go);
	// listsRev counts their changes, and listedMemo is what listed last
	// found.
	lists      map[string]bool
	listsRev   int
	listedMemo listedMemo
	// named are the bands' names the field showed last; listed is whether
	// it last stood as a list.
	named  map[string]bool
	listed bool
	// swapFrom is where the field's origin stood as the views began to
	// swap: what goes out stays there while the pill moves on.
	swapFrom stagePoint
	// changeT is how far the change the field is making has gone, on its
	// one clock, below 0 while it makes none; swapT is the same of a swap
	// alone. starts are where what goes along with a change stood as it
	// began (along), by name.
	changeT, swapT float32
	starts         map[string]float32
	// heldAbove and heldBelow are the room the query's own ribbons take
	// about the pill, as drawn.
	heldAbove, heldBelow float32
	// parts are how far aside each card stands to open the gap a sheet
	// dragged in a list would go into, by its id.
	parts map[string]float32
	// parted is whether a card still stood aside as the field last drew.
	parted bool
	// landing is the gap a sheet dragged in a list would land in, its line
	// at its left, as last drawn, and landingShown how much its line
	// showed.
	landing      stageRect
	landingShown float32
	// offered is how much the ribbons around the pill show: all but while
	// the views turn (swapOffered).
	offered float32
	// The viewport: base is where the field's origin stands on screen,
	// scrolled to 0; viewTop and viewBottom are where what it holds shows
	// clear, between the top band and the pill with what stands about it.
	// origin is where the field drew its origin, scrolled as it was.
	base, viewTop, viewBottom float32
	origin                    stagePoint
	// place is the view and query the field shows; places are where each
	// it showed lately was scrolled to as it was left (scrolls).
	place  string
	places []placeScroll
	// pin is how far the pill stands at the window's foot, as a list's
	// does, rather than at its horizon; fieldPillY keeps where the pill
	// stood on screen in the field when turning to a list; pillY is where
	// the pill was last drawn, down from the window's top.
	pin, fieldPillY, pillY float32
}

// fieldRoom is the room the field is laid out in: its width and height,
// at the type's size.
type fieldRoom struct{ w, h, scale float32 }

// change is how the field moves to a new layout: which way, the place its
// new results come from, and the result its old ones go back into.
type change struct {
	depth stageDepth
	enter stageRect
	exit  string
}

// FieldQuery is the field's query: its conditions, then its words.
func (s *State) FieldQuery() string {
	return strings.TrimSpace(strings.Join(append(slices.Clone(s.field.conditions), s.field.words), " "))
}

// SetFieldQuery sets the field's query: its conditions to ribbons, the
// rest to the words written, ranked as it comes.
func (s *State) SetFieldQuery(text string) {
	s.field.conditions, s.field.words = nil, ""
	s.field.rerank = true
	s.takeConditions(text)
	s.setWords(s.field.words)
}

// takeConditions takes the conditions written in text as ribbons, and
// leaves the rest as the words, reporting whether it held any.
func (s *State) takeConditions(text string) bool {
	conds, words := splitQuery(text)
	for _, text := range conds {
		if c, ok := condition(text); ok {
			s.field.conditions, _ = put(s.field.conditions, c)
		}
	}
	s.field.words = words
	return len(conds) > 0
}

func (s *State) setWords(words string) {
	s.field.words = words
	s.field.edit = &words
}

type fieldMode uint8

const (
	modeLibrary fieldMode = iota
	modeSettings
	modeCommands
)

// commanding reports whether the pill asks the app's commands.
func (s *State) commanding() bool { return strings.HasPrefix(s.field.words, ">") }

// modeFor reports which source owns the field for text.
func (s *State) modeFor(text string) fieldMode {
	if s.commanding() {
		return modeCommands
	}
	if inSettings(text) {
		return modeSettings
	}
	return modeLibrary
}

// mode reports which source owns the field for its current query.
func (s *State) mode() fieldMode {
	if s.commanding() {
		return modeCommands
	}
	if s.InSettings() {
		return modeSettings
	}
	return modeLibrary
}

// narrow puts cond in the query, in place of any of the same key, going
// down into what was chosen at from.
func (s *State) narrow(cond string, from stageRect) {
	c, ok := condition(cond)
	if !ok {
		return
	}
	key := written(c).Key.Name
	s.field.conditions = slices.DeleteFunc(s.field.conditions, func(text string) bool { return conditionKey(text) == key })
	s.field.conditions, _ = put(s.field.conditions, c)
	s.field.next = change{depth: stageDown, enter: from}
	s.field.chosen = ""
}

// broaden takes the query's condition at i away, coming back up into the
// block of the group it was in, if it was one.
func (s *State) broaden(i int) {
	if i < 0 || i >= len(s.field.conditions) {
		return
	}
	next := change{depth: stageUp}
	inSettings := s.InSettings()
	if c, ok := condition(s.field.conditions[i]); ok {
		switch {
		case c.Key.Name == "settings":
			// Leaving settings clears any settings-scoped category/section too.
			s.field.conditions = slices.DeleteFunc(slices.Clone(s.field.conditions), func(text string) bool {
				k := conditionKey(text)
				return k == "settings" || k == "in"
			})
			s.field.next = next
			s.field.chosen = ""
			return
		case c.Key.Name == "in" && inSettings:
			cat, _, _ := strings.Cut(c.Value, "/")
			next.exit = "settings-cat:" + cat
		case c.Key.Name == "in" && s.Library != nil:
			if g, ok := s.groupNamed(c.Value); ok {
				next.exit = "group:" + g.ID
			}
		}
	}
	s.field.conditions = slices.Delete(s.field.conditions, i, i+1)
	s.field.next = next
	s.field.chosen = ""
}

// into narrows the query to a group, or to what a smart folder finds.
func (s *State) into(g GroupItem, from stageRect) {
	if g.Smart() {
		s.SetFieldQuery(g.Query)
		s.field.next = change{depth: stageDown, enter: from}
		return
	}
	listed := s.listed()
	s.narrow((&query.Condition{Key: mustKey("in"), Value: g.Name}).String(), from)
	// Gone into from a list, a group is read as one too.
	if listed && !s.listed() {
		s.toggleList()
	}
}

// Search goes to the field, its pill taking the keyboard.
func (s *State) Search() { s.ShowField() }

// condition reads a condition as the language writes it.
func condition(text string) (query.Condition, bool) {
	conds := query.Conditions(query.Parse(text))
	if len(conds) != 1 {
		return query.Condition{}, false
	}
	return conds[0], true
}

func conditionKey(text string) string {
	if c, ok := condition(text); ok {
		return written(c).Key.Name
	}
	return text
}

func mustKey(name string) *query.Key {
	k, _ := query.KeyNamed(name)
	return k
}
