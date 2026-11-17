package shell

import (
	"slices"
	"strings"
	"time"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/scrollfx"
	"github.com/caelyreth/neverland/native/stage"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The field shows what its sources find for its query (cards.go), the
// library's in blocks (results.go, blocks.go), around the pill the query
// is written in (pill.go).
// Everything done on the field changes the query, and the field moves to
// what the changed query finds on one value (stage.Shift), from where the
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
	// rerank has found ranked again, as the window comes back to the
	// field, and rankDue is when it is, the writing in the pill paused.
	rerank  bool
	rankDue time.Time
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
	rest, drawn, from []stage.Frame
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
	poise, poiseTo stage.Point
	home, homing   bool
	// away is how far what the pill offers around it is put away, as its
	// horizon goes past where it all fits.
	away float32
	// measured is how tall each card of the settings laid itself out, by
	// its section, and measures counts the changes.
	measured map[string]float32
	measures uint64
	// over is how far past its ends the field is pulled, as the panels and
	// tables pull, and viewH the field's height, for its reach.
	over  scrollfx.Elastic
	viewH float32
	edges components.EdgeFade
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
	// picked is the sheets picked together, anchor where a Shift-click's
	// run starts (picks.go).
	picked []string
	anchor string
	// order is the rows the field shows, in order, and chosen the one the
	// keys are on.
	order  []string
	chosen string
	// renaming is the group whose name is being written on its head.
	renaming, renameName string
	renameFocus          bool
}

// change is how the field moves to a new layout: which way, the place its
// new results come from, and the result its old ones go back into.
type change struct {
	depth stage.Depth
	enter stage.Rect
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
func (s *State) narrow(cond string, from stage.Rect) {
	c, ok := condition(cond)
	if !ok {
		return
	}
	key := written(c).Key.Name
	s.field.conditions = slices.DeleteFunc(s.field.conditions, func(text string) bool { return conditionKey(text) == key })
	s.field.conditions, _ = put(s.field.conditions, c)
	s.field.next = change{depth: stage.Down, enter: from}
	s.field.chosen = ""
}

// broaden takes the query's condition at i away, coming back up into the
// block of the group it was in, if it was one.
func (s *State) broaden(i int) {
	if i < 0 || i >= len(s.field.conditions) {
		return
	}
	next := change{depth: stage.Up}
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
func (s *State) into(g GroupItem, from stage.Rect) {
	if g.Smart() {
		s.SetFieldQuery(g.Query)
		s.field.next = change{depth: stage.Down, enter: from}
		return
	}
	s.narrow((&query.Condition{Key: mustKey("in"), Value: g.Name}).String(), from)
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
// side of its horizon (stage.Horizon), the pill in the horizon, and the
// whole scrolling up and down as one, the pill keeping near the window's
// top or foot once its horizon has gone past them; pressed there, the
// pill brings the field back to its horizon. Hidden, as the desk fills the
// window, it is still built, so it keeps where everything stands and
// comes back as it was.
func (s *State) fieldView(c *ui.Context, semantic theme.Theme, look stage.Look, x, w, h float32, hidden, animate bool) {
	m := s.blockMetrics(w, h)
	k := m.k
	view := ui.Box(c).Key("field").Absolute().Top(0).Left(x).Size(w, h).Clip().Hidden(hidden).
		Role(ui.RoleGroup).Label("Field")
	if look.Offset.X != 0 {
		view.Translate(look.Offset.X, 0)
	}
	view.ContextMenu(s.fieldMenu)
	s.scopeDrops(semantic, view, k, animate)
	view.HandleInput(s.fieldInput)
	s.field.hidden = hidden
	// The field takes the keyboard from the pill let go of, and as it
	// comes back from the desk, for that frame alone: after it, nothing
	// holds the keyboard, and Escape goes on with the field.
	if s.field.quiet && !hidden {
		s.field.quiet = false
		view.Focusable().FocusRing(false).Focus()
	}
	if !s.OnDesk() && !c.FocusTakesText() && c.Shortcut(0, ui.KeyEscape) {
		s.escapeField()
	}
	f := s.results(c.Now())
	if !s.field.rankDue.IsZero() {
		c.After(s.field.rankDue.Sub(c.Now()))
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
	pill := stage.Size{W: pillW, H: 40 * k}
	rl := s.ribbons(c, k, w/2, targetPillW, pill.H)
	z := m.horizon
	z.Above += rl.above
	z.Below += rl.below
	rects := z.Place(heights)
	frames := make([]stage.Frame, len(rects))
	for i, r := range rects {
		frames[i] = stage.Frame{ID: f.cards[i].id, Size: shapes[i].size, Rect: r, Shown: 1}
	}
	below, above := stage.Reach(rects)
	below, above = max(below, z.Below), max(above, z.Above)
	top, foot := 48*k, 24*k
	// At rest, the field stands even in the window: its horizon rises or
	// falls into the room it leaves empty over or under it, never past
	// where something of it would leave the window, and its cards, where a
	// ring of columns is taken on one side alone, go across to stand even
	// about the pill. Never while the pill is written in, the pill and
	// what it finds keeping still under the caret: an action on the query,
	// or the pill let go of, sets it again.
	first := s.field.rest == nil
	if !s.field.open || s.field.next != (change{}) || first {
		up, down := max(0, h/2-top-above), max(0, h/2-foot-below)
		s.field.poiseTo = stage.Point{X: -stage.Centre(rects), Y: max(-down, min(up, (foot-top+below-above)/2))}
	}
	poise := s.field.poiseTo
	lift := poise.Y
	s.field.lo, s.field.hi = -max(0, above+3*m.pad-h/2+lift), max(0, below+3*m.pad-h/2-lift)
	s.field.order = s.field.order[:0]
	for _, cd := range f.cards {
		s.field.order = append(s.field.order, cd.kind.rows()...)
	}
	s.keepPicked()
	// What the query shows changed, the field comes back to its horizon on
	// the change's own spring; a card only growing or shrinking, as a
	// setting's fold opening, leaves it where it is.
	if s.field.rest != nil && !sameCards(frames, s.field.rest) && s.field.scroll != 0 {
		s.field.home = true
	}
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
	case !s.field.hold:
		spring := components.Spring(components.MotionScene)
		s.field.poise = stage.Point{X: view.AnimateTo("poise-x", poise.X, spring), Y: view.AnimateTo("poise-y", poise.Y, spring)}
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
	free := horizon - pill.H/2
	away := free < top+rl.above || free > h-foot-pill.H-rl.below
	py := max(top+rl.heldAbove, min(free, h-foot-pill.H-rl.heldBelow))
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
	view.Children(func() {
		// The top band drags the window where nothing of the field stands over it.
		ui.Box(c).Key("title-band").Absolute().Top(0).Left(0).Right(0).Height(titleLine(c)).DragWindow()
		at := stage.Point{X: w/2 + s.field.poise.X, Y: horizon}
		for _, fr := range drawn {
			cd, ok := s.field.shown[fr.ID]
			if !ok {
				continue
			}
			s.cardView(c, semantic, fr, cd, s.field.shownShape[fr.ID], at, live[fr.ID], m, animate)
		}
		if topFade > 0 || bottomFade > 0 {
			ui.Box(c).Key("field-edges").Absolute().Top(0).Left(0).Size(w, h).IgnoreInput(true).
				Draw(func(p *ui.Painter, r ui.Rect) {
					s.field.edges.Paint(p, r, semantic.Surface.Background, topFade, bottomFade)
				})
		}
		s.horizonView(c, semantic, stage.Rect{X: w/2 - pill.W/2, Y: py, W: pill.W, H: pill.H}, rl, w, k, animate)
	})
}

// fieldInput scrolls the field up and down, as far as it holds, and
// pulls it past its ends as the panels and tables are pulled.
func (s *State) fieldInput(ev ui.InputEvent) bool {
	if ev.Kind != ui.InputScroll {
		return false
	}
	s.field.home, s.field.homing = false, false
	s.field.scroll = components.OverscrollPanel.Scroll(&s.field.over, s.field.scroll, ev.DY, s.field.lo, s.field.hi,
		s.field.viewH, s.Settings.UIFontSize, ev)
	return true
}

// comeHome brings the field back to its horizon at the window's middle on
// one spring, from wherever it was scrolled to.
func (s *State) comeHome(el ui.Element, animate bool) {
	if !s.field.home {
		return
	}
	if !animate {
		s.field.scroll, s.field.home = 0, false
		return
	}
	if !s.field.homing {
		el.Jump("scroll", s.field.scroll)
		s.field.homing = true
		if s.field.hold {
			return
		}
	}
	s.field.scroll = el.AnimateTo("scroll", 0, components.Spring(components.MotionScene))
	if s.field.scroll == 0 {
		s.field.home, s.field.homing = false, false
	}
}

// shift moves the field to frames, the layout at rest of what f found: at
// once, or from what it drew on one spring, the change taking over from
// wherever one under way has got to.
func (s *State) shift(field ui.Element, frames []stage.Frame, f found, shapes []shape, animate bool) []stage.Frame {
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
		if first {
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
	case animate:
		t = field.AnimateTo("shift", 1, components.Spring(components.MotionScene))
	}
	if t >= 1 {
		s.field.from = nil
		s.field.drawn = frames
		return frames
	}
	var exit stage.Rect
	for _, fr := range frames {
		if fr.ID == s.field.now.exit {
			exit = fr.Rect
		}
	}
	s.field.drawn = stage.Shift(s.field.from, frames, s.field.now.enter, exit, s.field.now.depth, t)
	return s.field.drawn
}

// escapeField lets the picked go, then closes the open key, then puts the
// suggestions away, then
// takes the words away, then the last condition, and with none left goes
// back to the desk.
func (s *State) escapeField() {
	switch {
	case len(s.field.picked) > 0:
		s.field.picked = nil
	case s.field.opened != nil:
		s.closeKey()
	case s.offered():
		s.field.dismissed = s.field.words
	case s.field.words != "":
		s.setWords("")
	case len(s.field.conditions) > 0:
		s.broaden(len(s.field.conditions) - 1)
	default:
		s.ShowDesk()
	}
}

// goHome clears any active query filter and selection, returning the field
// to its non-filter home state and recentering the pill on its horizon.
func (s *State) goHome() {
	s.field.picked = nil
	s.field.chosen = ""
	s.field.dismissed = ""
	if s.field.opened != nil {
		s.closeKey()
	}
	if s.FieldQuery() != "" {
		next := change{depth: stage.Up}
		if s.Library != nil && !s.InSettings() {
			for _, cond := range s.field.conditions {
				if c, ok := condition(cond); ok && c.Key.Name == "in" {
					if g, ok := s.groupNamed(c.Value); ok {
						next.exit = "group:" + g.ID
					}
				}
			}
		}
		s.SetFieldQuery("")
		s.field.next = next
	}
	if s.field.scroll != 0 || s.field.over.Offset() != 0 {
		s.field.home = true
	}
}

// choose moves the keys by by along the rows the field shows.
func (s *State) choose(by int) {
	ids := s.field.order
	if len(ids) == 0 {
		return
	}
	at := slices.Index(ids, s.field.chosen)
	if at < 0 {
		s.field.chosen = ids[0]
		return
	}
	s.field.chosen = ids[max(0, min(at+by, len(ids)-1))]
}

// activate does what the row id does chosen: a group narrows the query
// to it, and a sheet opens on the desk, in place of the one in front or
// alongside it.
func (s *State) activate(id string, alongside bool) {
	if s.Library == nil {
		return
	}
	kind, key, _ := strings.Cut(id, ":")
	switch kind {
	case "suggest":
		s.takeSuggestion(key)
	case "key":
		s.takeKey(key)
	case "command":
		// The command runs as the field was before it was asked for.
		s.setWords("")
		if s.RunCommand != nil {
			s.RunCommand(key)
		}
	case "group":
		if g, ok := s.Library.Group(key); ok {
			s.into(g, stage.Rect{})
		}
	case "settings-cat", "settings-sec":
		s.intoSettings(key, stage.Rect{})
	case "sheet":
		for _, cd := range s.field.found.cards {
			b, ok := cd.kind.(block)
			if !ok || b.trashed {
				continue
			}
			for _, e := range b.entries {
				if e.sheet.ID != key {
					continue
				}
				if s.field.words != "" {
					s.Library.OpenFound(key, s.FieldQuery(), alongside)
				} else {
					s.Library.OpenSheet(key, alongside)
				}
				s.ShowDesk()
				return
			}
		}
	}
}

// sameCards reports whether a and b are the same cards, in order, wherever
// and however large they stand.
func sameCards(a, b []stage.Frame) bool {
	return slices.EqualFunc(a, b, func(x, y stage.Frame) bool { return x.ID == y.ID })
}

// scopeGroup is the group the field is in, where a sheet may be filed:
// none at the top, in the trash, or in a smart folder.
func (s *State) scopeGroup() (GroupItem, bool) {
	if s.Library == nil {
		return GroupItem{}, false
	}
	scope, trash, _ := s.browsing(query.Parse(s.wholeQuery()))
	if scope == "" || trash {
		return GroupItem{}, false
	}
	g, ok := s.Library.Group(scope)
	return g, ok && !g.Smart()
}

// scopeDrops makes the field's ground, in a group, take what that group
// takes, filing it at the group's top, as dropped on its block: in a group
// whose sheets each stand alone, there is no block of its own to drop on.
// At the top the ground takes nothing. It is outlined while something it
// takes is over it.
func (s *State) scopeDrops(semantic theme.Theme, view ui.Element, k float32, animate bool) {
	g, ok := s.scopeGroup()
	if !ok {
		return
	}
	over, dropped, ok := dragOn(view, filing(g))
	if ok {
		s.fileInto(g, dropped)
	}
	lit := map[bool]float32{true: 1}[over]
	if animate {
		lit = components.Follow(view, "drop", lit, components.MotionQuick)
	}
	if lit > 0.001 {
		view.DrawOver(func(p *ui.Painter, r ui.Rect) {
			inset := 8 * k
			r = ui.Rect{X: r.X + inset, Y: r.Y + inset, W: r.W - 2*inset, H: r.H - 2*inset}
			components.DashedOutline(p, r, 16*k, 1.5, semantic.Accent.Foreground.Alpha(lit))
		})
	}
}

// fieldMenu is what the field's ground offers.
func (s *State) fieldMenu(m *ui.Menu) {
	if s.Library == nil {
		return
	}
	if m.Item("New Sheet").Chosen() {
		s.Library.NewSheet(s.CurrentGroup(), Last)
		s.ShowDesk()
	}
	if m.Item("New Group").Chosen() {
		s.newGroup(s.scope())
	}
	m.Separator()
	if s.inTrash() {
		if m.Item("Leave Trash").Chosen() {
			s.toggleTrash()
		}
		if m.Item("Empty Trash…").Chosen() {
			s.arm(emptyTrashKey)
		}
		return
	}
	if m.Item("Show Trash").Chosen() {
		s.toggleTrash()
	}
}

// inTrash reports whether the field shows the trash.
func (s *State) inTrash() bool {
	return slices.ContainsFunc(s.field.conditions, func(c string) bool { return conditionKey(c) == "trash" })
}

// toggleTrash shows the trash in the field, in place of what it showed, or
// leaves it.
func (s *State) toggleTrash() {
	if i := slices.IndexFunc(s.field.conditions, func(c string) bool { return conditionKey(c) == "trash" }); i >= 0 {
		s.broaden(i)
		return
	}
	s.field.conditions = nil
	s.setWords("")
	s.narrow((&query.Condition{Key: mustKey("trash")}).String(), stage.Rect{})
}

// scope is the group the query is in, "" for none.
func (s *State) scope() string {
	if s.Library == nil {
		return ""
	}
	scope, _, _ := s.browsing(query.Parse(s.wholeQuery()))
	return scope
}

// CurrentGroup is the group a new sheet goes in: the one the field is in,
// or the Inbox.
func (s *State) CurrentGroup() string {
	if g := s.scope(); g != "" {
		return g
	}
	if s.Library == nil {
		return ""
	}
	for _, g := range s.Library.Groups("") {
		if g.Inbox {
			return g.ID
		}
	}
	return ""
}

func (s *State) newGroup(parent string) {
	id := s.Library.NewGroup(parent, Last)
	s.startRename(id)
}

func (s *State) startRename(id string) {
	g, ok := s.Library.Group(id)
	if !ok || g.Inbox {
		return
	}
	s.field.renaming, s.field.renameName, s.field.renameFocus = id, g.Name, true
}

// ReconcileLibrary drops what the library no longer has: a group being
// renamed, and a condition naming a group gone.
func (s *State) ReconcileLibrary() {
	if s.Library == nil {
		return
	}
	if s.field.renaming != "" {
		if _, ok := s.Library.Group(s.field.renaming); !ok {
			s.field.renaming = ""
		}
	}
	s.field.conditions = slices.DeleteFunc(s.field.conditions, func(text string) bool {
		c, ok := condition(text)
		if !ok || c.Key.Name != "in" {
			return false
		}
		_, found := s.groupNamed(c.Value)
		return !found
	})
}
