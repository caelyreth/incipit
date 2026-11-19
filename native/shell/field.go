package shell

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/plugins/glass"
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
	notice     *notice
	told       float32
	lastNotice string
	// completion is what may stand at the caret, caret bytes into the
	// words completedFor, offered unless the words are what it was put
	// away at, dismissed, suggestion the one of it the keys are on (-1 for
	// none), and allValues whether every value stands, not just the first.
	completion query.Completion
	caret      int
	suggestion int
	allValues  bool
	// opened is the key open in the pill, its values standing around it,
	// and held what was written before it opened.
	opened       *query.Key
	held         string
	completedFor string
	dismissed    string
	found        found
	// lead is the settings section that, pressed, opened its category, to
	// stand first in it.
	lead string
	// rerank has found ranked again, as the window comes back to the
	// field; settleAt is when the field settles as the writing in the pill
	// pauses, zero while nothing is being written, and settled that it
	// has, for the field to stand even again.
	rerank, settled bool
	settleAt        time.Time
	// shapes are found's cards' shapes, by id, for the measures shapesKey
	// names.
	shapes    map[string]shape
	shapesKey string
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
	// or foot, 0 to 1, which its backdrop comes in with.
	stuck float32
	// room is the room the field was last laid out in.
	room fieldRoom
	// ribbonHold is what the ribbons about the pill hold to while it
	// stands as it is (pill.go).
	ribbonHold ribbonHold
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
	gathering *[]func()
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
	// wordsBefore is the pill's words as the last frame left them, for
	// Space on a row to be taken back from them.
	wordsBefore string
	// renaming is the group whose name is being written on its head.
	renaming, renameName string
	renameFocus          bool
	// lists are the places shown as a list, kept for the session (list.go).
	lists map[string]bool
	// named are the bands' names the field showed last; listed is whether
	// it last stood as a list.
	named  map[string]bool
	listed bool
	// swapFrom is where the field's origin stood as the views began to
	// swap: what goes out stays there while the pill moves on. swapPoise
	// and swapPin are where the pill was held and pinned then; swapT is
	// how far the swap has gone, below 0 while none is going.
	swapFrom, swapPoise stagePoint
	swapPin, swapT      float32
	// parts are how far aside each card stands to open the gap a sheet
	// dragged in a list would go into, by its id.
	parts map[string]float32
	// pin is how far the pill stands at the window's foot, as a list's
	// does, rather than at its horizon.
	pin float32
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

// fieldView draws the field between the field lane and the desk lane, x
// from the window's left, w by h, as the scene stands it: what the query finds in slots on either
// side of its horizon (stageHorizon), the pill in the horizon, and the
// whole scrolling up and down as one, the pill keeping near the window's
// top or foot once its horizon has gone past them; pressed there, the
// pill brings the field back to its horizon. Hidden, as the desk fills the
// window, it is still built, so it keeps where everything stands and
// comes back as it was.
func (s *State) fieldView(c *ui.Context, semantic theme.Theme, look stageLook, x, w, h float32, hidden, animate bool) {
	m := s.blockMetrics(w, h)
	k := m.k
	// The window resized, or the type, the field stands at once as its new
	// measures lay it out: nothing of it springs from where it stood, so
	// each frame of a resize is drawn whole and at its own size. Measures
	// the query changes, as the settings' wider columns, move as any change
	// of the query does.
	if room := (fieldRoom{w, h, m.scale}); room != s.field.room {
		animate = false
		s.field.room = room
	}
	view := ui.Box(c).Key("field").Absolute().Top(0).Left(x).Size(w, h).Clip().Hidden(hidden).
		Role(ui.RoleGroup).Label("Field")
	if look.Offset.X != 0 {
		view.Translate(look.Offset.X, 0)
	}
	view.ContextMenu(s.fieldMenu)
	view.HandleInput(s.fieldInput)
	s.field.hidden = hidden
	// The field takes the keyboard from the pill let go of, and as it
	// comes back from the desk, for that frame alone: after it, nothing
	// holds the keyboard, and Escape goes on with the field.
	if s.field.quiet && !hidden {
		s.field.quiet = false
		view.Focusable().FocusRing(false).Focus()
	}
	if !s.OnDesk() && !c.FocusTakesText() {
		switch {
		case c.Shortcut(0, ui.KeyEscape):
			s.escapeField()
		case c.Shortcut(ui.Cmd, ui.KeyZ):
			s.undoNotice()
		case s.field.acting && c.Shortcut(0, ui.KeyUp):
			s.actionKey(ui.KeyUp)
		case s.field.acting && c.Shortcut(0, ui.KeyDown):
			s.actionKey(ui.KeyDown)
		case s.field.acting && c.Shortcut(0, ui.KeyEnter):
			s.actionKey(ui.KeyEnter)
		}
	}
	f := s.results(c.Now())
	if !s.field.settleAt.IsZero() {
		c.After(s.field.settleAt.Sub(c.Now()))
	}
	// What fits in the row under the pill stands there alone, the field
	// over it empty: its blocks may stand as tall as the window holds.
	if len(f.cards) <= m.horizon.Cols {
		m.tall = m.alone
	}
	shapes := s.shapes(c, f, m)
	heights := make([]float32, len(shapes))
	for i, sh := range shapes {
		heights[i] = sh.size.H
	}
	// Its band reaches past the pill as far as the ribbons around it do.
	maxPillW := max(0, w-2*(m.pad+horizonSide*k))
	idlePillW, openPillW := min(280*k, maxPillW), min(440*k, maxPillW)
	activePill := s.field.open || (s.field.focus && !s.OnDesk() && !s.field.hidden)
	targetPillW := idlePillW
	pillAnim := components.Spring(components.MotionNormal)
	if activePill {
		targetPillW = openPillW
		pillAnim = components.Bouncy(components.MotionEnter)
	}
	pillW := targetPillW
	if animate {
		pillW = max(0, min(w, view.AnimateTo("pill-w", targetPillW, pillAnim)))
	} else {
		view.Jump("pill-w", targetPillW)
	}
	pill := stageSize{W: pillW, H: 40 * k}
	rl := s.ribbons(c, k, w/2, targetPillW, pill.H)
	z := m.horizon
	z.Above += rl.above
	z.Below += rl.below
	// Each band chosen past the last, the ring's room between holding its
	// name.
	bands := cardBands(f)
	if bands != nil {
		z.Named = true
		z.Ring = theme.TextXSmall.LineHeight(m.scale) + z.Gap
	}
	rects, marks := z.Place(heights, bands)
	listed := s.listed()
	if listed {
		rects, marks = z.Rows(heights, listWide(f.cards)), nil
	}
	// Turning between the views is a swap: one view goes, the other comes.
	if listed != s.field.listed && s.field.rest != nil {
		s.field.next = change{depth: stageSwap}
		s.field.swapFrom = stagePoint{X: w/2 + s.field.poise.X, Y: h/2 - s.field.poise.Y - (s.field.scroll - s.field.over.Offset())}
		s.field.swapPoise, s.field.swapPin = s.field.poise, s.field.pin
		// The new view starts at its home: nothing scrolls on a clock of
		// its own while the views turn.
		s.field.scroll, s.field.home = 0, false
		view.Jump("scroll", 0)
	}
	s.field.listed = listed
	frames := make([]stageFrame, len(rects))
	for i, r := range rects {
		frames[i] = stageFrame{ID: f.cards[i].id, Size: shapes[i].size, Rect: r, Shown: 1}
	}
	below, above := stageReach(rects)
	below, above = max(below, z.Below), max(above, z.Above)
	top, foot := 48*k, 24*k
	// At rest, the pill stands at the window's middle, its ribbons with it,
	// however many stand on either side: only cards standing on one side
	// more than the other move it, the horizon rising or falling into the
	// room they leave empty over or under it; and the cards,
	// where a ring of columns is taken on one side alone, go across to
	// stand even about the pill. While the pill is open it keeps still,
	// under the pointer and the caret, and the field with it, until the
	// writing in it pauses; but never so far that something of the field
	// would leave the window or its columns, which brings it in as far as
	// needs be. Let go of, the pill goes where the field is even.
	first := s.field.rest == nil
	up, down := max(0, h/2-top-above), max(0, h/2-foot-below)
	lo, hi := stageExtent(rects)
	reach := (float32(z.Cols)*z.Col + float32(z.Cols-1)*z.Gap) / 2
	if !s.field.open || first || s.field.settled {
		s.field.settled = false
		cardsBelow, cardsAbove := max(0, below-z.Below), max(0, above-z.Above)
		s.field.poiseTo = stagePoint{X: -(lo + hi) / 2, Y: (cardsBelow - cardsAbove) / 2}
	}
	// What it is held to stays as it was: brought in only as far as the
	// field needs now, it goes back as far as the field lets it.
	poise := stagePoint{
		X: max(-reach-lo, min(reach-hi, s.field.poiseTo.X)),
		Y: max(-down, min(up, s.field.poiseTo.Y)),
	}
	// A list rests on the pill at the window's foot, however little it
	// holds, open or not.
	if listed {
		poise.Y = -down
	}
	lift := poise.Y
	s.field.lo, s.field.hi = -max(0, above+3*m.pad-h/2+lift), max(0, below+3*m.pad-h/2-lift)
	// The keys start from what ranks first, nearest the pill; they go on,
	// and a Shift-click takes its run, in the order the field is read on
	// the screen (field-and-desk.md §4.7.7).
	s.field.ranked, s.field.order = s.field.ranked[:0], s.field.order[:0]
	for _, cd := range f.cards {
		s.field.ranked = append(s.field.ranked, cd.kind.rows()...)
	}
	if listed {
		// A list is read as it is ranked.
		s.field.order = append(s.field.order, s.field.ranked...)
	} else {
		for _, i := range stageScreenOrder(rects) {
			s.field.order = append(s.field.order, f.cards[i].kind.rows()...)
		}
	}
	s.keepPicked()
	s.keepRowPlaces()
	// What the query shows changed, the field comes back to its horizon on
	// the change's own spring; a card only growing or shrinking, as a
	// setting's fold opening, leaves it where it is.
	// A list arranged keeps its cards, only their order changed, and stays.
	if s.field.rest != nil && !sameCards(frames, s.field.rest) && !(listed && sameSet(frames, s.field.rest)) && s.field.scroll != 0 {
		s.field.home, s.field.homeTo = true, 0
	}
	// What arrives is brought into view, wherever the change left it.
	s.reveal(c, frames, f, h, poise.Y, top+rl.above, foot+rl.below)
	// Coming home, the field goes from wherever it was, past its new reach
	// too; otherwise it is kept within it.
	if !s.field.home {
		s.field.scroll = max(s.field.lo, min(s.field.scroll, s.field.hi))
	}
	drawn := s.shift(view, frames, f, shapes, animate)
	// The field goes there on the change's own spring, from the frame
	// after its first, as the cards do.
	switch {
	case !animate || first:
		view.Jump("poise-x", poise.X)
		view.Jump("poise-y", poise.Y)
		s.field.poise = poise
	case s.field.swapT >= 0:
		// Turning views, the pill goes on the swap's own clock, so it
		// arrives as the cards do and nothing is left to catch up.
		e := swapEase(s.field.swapT)
		from := s.field.swapPoise
		s.field.poise = stagePoint{X: view.Jump("poise-x", from.X+(poise.X-from.X)*e), Y: view.Jump("poise-y", from.Y+(poise.Y-from.Y)*e)}
	case !s.field.hold:
		spring := components.Spring(components.MotionScene)
		s.field.poise = stagePoint{X: view.AnimateTo("poise-x", poise.X, spring), Y: view.AnimateTo("poise-y", poise.Y, spring)}
	}
	s.comeHome(view, animate)
	if s.field.hold {
		s.field.hold = false
		c.AnimationFrame()
	}
	live := make(map[string]bool, len(frames))
	for _, fr := range frames {
		live[fr.ID] = true
	}
	// The pill stands at the horizon, moving with the field as it
	// scrolls, but no further than near the window's top or foot: there it
	// stays, and what the field holds passes under it.
	s.field.viewH = h
	_, pulling := s.field.over.Step(c.Now(), animate)
	if pulling {
		c.AnimationFrame()
	}
	horizon := h/2 - s.field.poise.Y - (s.field.scroll - s.field.over.Offset())
	// The pill follows its horizon while all around it fits in the window.
	// Past that, what is offered around it is put away, and it goes on
	// with the query's own conditions alone, as far as they still fit, and
	// stays there while what the field holds passes under it.
	// Whether it fits goes by where the field is going, not where its
	// spring has it on the way: the pill making room for what it offers,
	// as at a list's top, never puts it away while it comes.
	free := horizon - pill.H/2
	bound := h/2 - poise.Y - (s.field.scroll - s.field.over.Offset()) - pill.H/2
	away := bound < top+rl.above || bound > h-foot-pill.H-rl.below
	py := max(top+rl.heldAbove, min(free, h-foot-pill.H-rl.heldBelow))
	// A list's pill stands at the window's foot, its ground, and does not
	// go with the list as it scrolls; it eases there and back as the
	// views turn.
	pin := float32(0)
	if listed {
		pin, away = 1, false
	}
	switch {
	case animate && s.field.swapT >= 0:
		pin = view.Jump("pin", s.field.swapPin+(pin-s.field.swapPin)*swapEase(s.field.swapT))
	case animate:
		pin = components.Follow(view, "pin", pin, components.MotionScene)
	}
	s.field.pin = pin
	py += (h - foot - pill.H - rl.heldBelow - py) * pin
	s.field.away = map[bool]float32{true: 1}[away]
	if animate {
		s.field.away = components.Follow(view, "away", s.field.away, components.MotionQuick)
	}
	var drawnAbove, drawnBelow float32
	for _, fr := range drawn {
		if _, ok := s.field.shown[fr.ID]; !ok {
			continue
		}
		if fr.Rect.Y < 0 {
			drawnAbove = max(drawnAbove, -fr.Rect.Y)
		}
		if fr.Rect.Y+fr.Rect.H > 0 {
			drawnBelow = max(drawnBelow, fr.Rect.Y+fr.Rect.H)
		}
	}
	topFade, bottomFade := min(top, max(0, drawnAbove-horizon)), min(top, max(0, horizon+drawnBelow-h))
	// The window's edges shade what passes under them; as what stands
	// there comes and goes, the shade comes and goes with it.
	if animate {
		topFade = components.Follow(view, "top-fade", topFade, components.MotionQuick)
		bottomFade = components.Follow(view, "bottom-fade", bottomFade, components.MotionQuick)
	}
	// Staying at the window's top or foot, the pill and its conditions
	// stand out from what passes under them as macOS bars do: the field
	// blurs under them, more toward the window's edge, under its ground
	// fading away from that edge, coming in as the horizon goes past.
	stuckTop, stuckFoot := top+rl.heldAbove-free, free-(h-foot-pill.H-rl.heldBelow)
	s.field.stuck = max(0, min(1, stuckTop/foot), min(1, stuckFoot/foot))
	if animate {
		s.field.stuck = components.Follow(view, "stuck", s.field.stuck, components.MotionQuick)
	}
	edge := py + pill.H + rl.heldBelow + 10*k
	if stuckFoot > 0 {
		edge = h - (py - rl.heldAbove) + 10*k
	}
	view.Children(func() {
		// The top band drags the window where nothing of the field stands over it.
		ui.Box(c).Key("title-band").Absolute().Top(0).Left(0).Right(0).Height(titleLine(c)).DragWindow()
		at := stagePoint{X: w/2 + s.field.poise.X, Y: horizon}
		// Swapping views, only the pill travels: what goes out fades where
		// it stood, and what comes in rises where it will stand.
		swapping := s.field.from != nil && s.field.now.depth == stageSwap
		if swapping {
			at = stagePoint{X: w/2 + poise.X, Y: h/2 - poise.Y - (s.field.scroll - s.field.over.Offset())}
		}
		s.bandNames(c, semantic, f, marks, at, m, animate)
		for _, fr := range drawn {
			cd, ok := s.field.shown[fr.ID]
			if !ok {
				continue
			}
			from := at
			if swapping && !live[fr.ID] {
				from = s.field.swapFrom
			}
			from.X += s.partOf(view, fr.ID, animate)
			s.cardView(c, semantic, fr, cd, s.field.shownShape[fr.ID], from, live[fr.ID], m, animate)
		}
		if s.field.drag != nil {
			s.arrangeTargets(c, semantic, drawn, at, m, animate)
		} else {
			// Let go of, what stood aside goes back.
			s.field.parts = nil
		}
		if topFade > 0 || bottomFade > 0 {
			ui.Box(c).Key("field-edges").Absolute().Top(0).Left(0).Size(w, h).IgnoreInput(true).
				Draw(func(p *ui.Painter, r ui.Rect) {
					s.field.edges.Paint(p, r, semantic.Surface.Background, topFade, bottomFade)
				})
		}
		if s.field.stuck > 0 {
			s.pillBackdrop(c, semantic, w, edge, stuckFoot > 0)
		}
		s.horizonView(c, semantic, stageRect{X: w/2 - pill.W/2, Y: py, W: pill.W, H: pill.H}, rl, w, k, animate)
	})
}

// pillBackdrop is what the stuck pill stands on, reaching from the
// window's top, or its foot, as far as edge.
func (s *State) pillBackdrop(c *ui.Context, semantic theme.Theme, w, edge float32, foot bool) {
	mask := ui.LinearGradient{From: ui.RGB(0, 0, 0).Alpha(s.field.stuck), To: ui.Transparent, Angle: 180}
	if foot {
		mask.Angle = 0
	}
	ground := semantic.Surface.Background.Alpha(max(s.field.stuck, 1.0/255))
	for i, m := range []ui.Material{
		glass.Blur{Radius: 8 * s.field.stuck, Mask: &mask},
		glass.ScrollEdge{Bottom: foot, Background: ground},
	} {
		b := ui.Box(c).Key(fmt.Sprint("pill-backdrop-", i)).Absolute().Left(0).Size(w, edge).IgnoreInput(true).Material(m)
		if foot {
			b.Bottom(0)
		} else {
			b.Top(0)
		}
	}
}

// fieldInput scrolls the field up and down, as far as it holds, and
// pulls it past its ends as the panels and tables are pulled.
func (s *State) comeHome(el ui.Element, animate bool) {
	if !s.field.home {
		return
	}
	if !animate {
		s.field.scroll, s.field.home = s.field.homeTo, false
		return
	}
	if !s.field.homing {
		el.Jump("scroll", s.field.scroll)
		s.field.homing = true
		if s.field.hold {
			return
		}
	}
	s.field.scroll = el.AnimateTo("scroll", s.field.homeTo, components.Spring(components.MotionScene))
	if s.field.scroll == s.field.homeTo {
		s.field.home, s.field.homing = false, false
	}
}

// shift moves the field to frames, the layout at rest of what f found: at
// once, or from what it drew on one spring, the change taking over from
// wherever one under way has got to.
func (s *State) shift(field ui.Element, frames []stageFrame, f found, shapes []shape, animate bool) []stageFrame {
	if s.field.shown == nil {
		s.field.shown, s.field.shownShape = map[string]card{}, map[string]shape{}
	}
	if !slices.Equal(frames, s.field.rest) {
		// The field's first layout stands at once: there is nothing for it
		// to come from.
		first := s.field.rest == nil
		s.field.from = s.field.drawn
		s.field.now = s.field.next
		s.field.next = change{}
		s.field.rest = frames
		if s.field.now.depth == stageSwap {
			// What was drawn goes as it was. A card the new view shows too,
			// as a group's card that is its divider in the list, goes under
			// an id of its own, so it goes out as it stood and comes in
			// anew; the rest keep theirs, and what was drawn of them.
			shared := make(map[string]bool, len(frames))
			for _, fr := range frames {
				shared[fr.ID] = true
			}
			from := make([]stageFrame, len(s.field.from))
			for i, fr := range s.field.from {
				if id := "was:" + fr.ID; shared[fr.ID] {
					s.field.shown[id], s.field.shownShape[id] = s.field.shown[fr.ID], s.field.shownShape[fr.ID]
					fr.ID = id
				}
				from[i] = fr
			}
			s.field.from = from
		}
		live := make(map[string]bool, len(frames)+len(s.field.from))
		for _, fr := range frames {
			live[fr.ID] = true
		}
		for _, fr := range s.field.from {
			live[fr.ID] = true
		}
		for id := range s.field.shown {
			if !live[id] {
				delete(s.field.shown, id)
				delete(s.field.shownShape, id)
			}
		}
		field.Jump("shift", 0)
		if first || !animate {
			field.Jump("shift", 1)
		} else {
			s.field.hold = true
		}
	} else {
		// An action that brought nothing new to lay out is spent: what is
		// written next is writing, not it.
		s.field.next = change{}
	}
	for i, cd := range f.cards {
		s.field.shown[cd.id] = cd
		s.field.shownShape[cd.id] = shapes[i]
	}
	t := float32(1)
	switch {
	case animate && s.field.hold:
		// Its first frame draws where everything was; the spring starts on
		// the next, so the time this one takes is not a leap.
		t = 0
	case animate && s.field.now.depth == stageSwap:
		// Each card eases on its own over the swap's way (stageSwapFrames).
		t = field.AnimateTo("shift", 1, ui.Animation{Duration: swapFor, Ease: func(t float32) float32 { return t }})
	case animate:
		t = field.AnimateTo("shift", 1, components.Spring(components.MotionScene))
	}
	s.field.swapT = -1
	if t >= 1 {
		s.field.from = nil
		s.field.drawn = frames
		return frames
	}
	if s.field.now.depth == stageSwap {
		s.field.swapT = t
	}
	var exit stageRect
	for _, fr := range frames {
		if fr.ID == s.field.now.exit {
			exit = fr.Rect
		}
	}
	s.field.drawn = stageShift(s.field.from, frames, s.field.now.enter, exit, s.field.now.depth, t)
	return s.field.drawn
}

// escapeField closes the actions, then lets the selection go, then
// closes the open key, then puts the suggestions away, then takes the
// words away, then the last condition, and with none left goes back to
// the desk.
// swapFor is how long turning between the views takes, all of it.
const swapFor = 560 * time.Millisecond

func sameCards(a, b []stageFrame) bool {
	return slices.EqualFunc(a, b, func(x, y stageFrame) bool { return x.ID == y.ID })
}

// sameSet reports whether a and b hold the same cards, in any order.
func sameSet(a, b []stageFrame) bool {
	if len(a) != len(b) {
		return false
	}
	ids := make(map[string]bool, len(a))
	for _, fr := range a {
		ids[fr.ID] = true
	}
	for _, fr := range b {
		if !ids[fr.ID] {
			return false
		}
	}
	return true
}

// scopeGroup is the group the field is in, where a sheet may be filed:
// none at the top, in the trash, or in a smart folder.
