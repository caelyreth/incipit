package shell

import (
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/caelyreth/neverland/internal/text/query"
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo/ui"
)

// What the field shows comes from its sources (field-and-desk.md §4.4):
// the library, the app's commands, the settings, and more to come. Each source answers
// the query with cards, or with none; which answer is the query's to say,
// never a mode's. The field ranks every card the same way, in tiers, then
// in the order its source gave, and places and moves them whatever they
// came from. A card is drawn by its kind: its own measure and its own
// view, in a frame every card shares.

// card is one thing a source puts in the field.
type card struct {
	// id is the card's identity as it moves between layouts.
	id   string
	tier tier
	// band is how far out it stands among the field's bands (bands.go).
	band int
	kind cardKind
	// rows are its rows' ids, as the keys go (cardKind.rows), worked out
	// once as it is found: the field reads them every frame, for every
	// card, built or not.
	rows []string
	// look is what its shape is laid out from, beside the library and the
	// measures (State.shapes): the query it was found for, or what a kind
	// that says (shapeKeyer) shows.
	look string
}

// shapeKeyer is a card kind whose shape follows from what it shows, not
// from the query that found it: as a sort or a filter changes, a card
// showing the same is not laid out again.
type shapeKeyer interface {
	shapeKey() string
}

// tier is how near the pill a card stands among others, before the order
// its source gives: what the query finds, then what is around it.
type tier uint8

const (
	tierResult tier = iota
	tierAmbient
)

// cardKind is what a card holds, and how it is laid out and drawn.
type cardKind interface {
	// shape lays the card out at the field's measures.
	shape(s *State, c *ui.Context, m blockMetrics) shape
	// rows are the rows the keys go along in it, in order.
	rows() []string
	// ground reports whether the card stands on a card's ground, framed.
	ground() bool
	// drawsLive reports whether the card is drawn anew each frame, as while
	// it holds the keyboard, instead of kept as a layer.
	drawsLive(s *State) bool
	// view fills box, the card at its own size, as sh lays it out; current
	// while the field still shows it, not on its way out.
	view(s *State, c *ui.Context, semantic theme.Theme, box ui.Element, fr stageFrame, sh shape, current bool, m blockMetrics, animate bool)
}

// source answers the field's query text with cards, added to f.
type source interface {
	answer(s *State, text string, f *found)
}

// sources are what the field asks, in order.
var sources = []source{librarySource{}, commandSource{}, settingsSource{}, ambientSource{}, nothingFound{}}

type statMode uint8

const (
	statLibrary statMode = iota
	statCommands
	statSettings
)

type fieldStats struct {
	mode  statMode
	count int
	words int
}

// found is what the field's query found, for the library as it stood.
type found struct {
	query string
	rev   uint64
	open  string
	held  uint64
	// cards are what the field shows, in order, and ranked in the order
	// the query ranks them, which cards takes once it is ranked again.
	cards, ranked []card
	stats         fieldStats
	// listed is whether it was found for a list; now is when.
	listed bool
	now    time.Time
	// bands name the bands the cards stand in, nearest first, as the
	// query chose them; nil for none.
	bands []string
}

func (f *found) add(id string, t tier, kind cardKind) {
	look := f.query
	if k, ok := kind.(shapeKeyer); ok {
		look = k.shapeKey()
	}
	f.cards = append(f.cards, card{id: id, tier: t, kind: kind, rows: kind.rows(), look: look})
}

// results is what the field's query finds, worked out again only when the
// query, the library, the sheets open or the tray change, as it stands
// at now.
//
// A search's rank holds while its query stands (field-and-desk.md §5.3):
// what it finds again, as a sheet is written, stands where it stood, and
// what is written on in the pill keeps what it still finds where it was,
// what it finds besides after them, until the writing pauses (settleAt);
// then, or as the window comes back to the field, it is ranked again. A
// query changed by an action, or one that browses, is ranked as it comes:
// what is browsed stands as it is filed.
func (s *State) results(now time.Time) found {
	if s.Library == nil {
		return found{}
	}
	text := s.wholeQuery()
	open := openKey(s.Open)
	rev := s.Library.Revision()
	held := s.lane.heldRev
	prev := s.field.found
	listed := s.listed()
	if prev.query == text && prev.rev == rev && prev.open == open && prev.held == held && prev.listed == listed && prev.cards != nil {
		if paused := !s.field.settleAt.IsZero() && !now.Before(s.field.settleAt); s.field.rerank || paused {
			prev.cards = prev.ranked
			s.field.found = prev
			s.field.settled = paused
			s.field.rerank, s.field.settleAt = false, time.Time{}
		}
		return prev
	}
	f := found{query: text, rev: rev, open: open, held: held, listed: listed, now: now, cards: []card{}}
	for _, src := range sources {
		src.answer(s, text, &f)
	}
	sort.SliceStable(f.cards, func(i, j int) bool { return f.cards[i].tier < f.cards[j].tier })
	s.band(&f, now)
	f.ranked = f.cards
	_, _, browse := s.browsing(query.Parse(text))
	switch {
	case prev.cards == nil || s.field.rerank || s.field.next != (change{}):
		s.field.settleAt = time.Time{}
		s.field.settled = true
	case prev.query != text:
		// Written on: the field settles once the writing pauses.
		s.field.settleAt = now.Add(writingPause)
		if !browse {
			f.cards = steady(prev.cards, f.ranked)
		}
	case !browse:
		f.cards = steady(prev.cards, f.ranked)
	}
	s.field.rerank = false
	f.stats = s.computeFieldStats(text, f.cards)
	s.field.found = f
	return f
}

// writingPause is how long the writing in the pill pauses before the
// field settles: what it finds ranked again, and the field standing even
// in the window.
const writingPause = 600 * time.Millisecond

// steady is next's cards, those prev shows in the order it shows them,
// then the rest in next's own; each tier apart.
func steady(prev, next []card) []card {
	at := make(map[string]int, len(prev))
	for i, cd := range prev {
		at[cd.id] = i
	}
	out := slices.Clone(next)
	slices.SortStableFunc(out, func(a, b card) int {
		if a.tier != b.tier {
			return int(a.tier) - int(b.tier)
		}
		i, wasA := at[a.id]
		j, wasB := at[b.id]
		switch {
		case wasA && wasB:
			return i - j
		case wasA:
			return -1
		case wasB:
			return 1
		}
		return 0
	})
	return out
}

func (s *State) computeFieldStats(text string, cards []card) fieldStats {
	switch s.modeFor(text) {
	case modeCommands:
		n := 0
		for _, cd := range cards {
			if cl, ok := cd.kind.(commandList); ok {
				n += len(cl.commands)
			}
		}
		return fieldStats{mode: statCommands, count: n}
	case modeSettings:
		n := 0
		for _, cd := range cards {
			switch sc := cd.kind.(type) {
			case settingsCategoryCard:
				n += len(sc.sections)
			case settingsCard:
				n++
			}
		}
		return fieldStats{mode: statSettings, count: n}
	default:
		st := fieldStats{mode: statLibrary}
		seen := map[string]bool{}
		add := func(sh SheetItem) {
			if seen[sh.ID] {
				return
			}
			seen[sh.ID] = true
			st.count++
			st.words += sh.Words
		}
		var walkGroup func(id string)
		walkGroup = func(id string) {
			for _, sh := range s.Library.Sheets(Place{Kind: PlaceGroup, Group: id}) {
				if !s.inTray(sh.ID) {
					add(sh)
				}
			}
			for _, sub := range s.groups(id) {
				walkGroup(sub.ID)
			}
		}
		for _, cd := range cards {
			b, ok := cd.kind.(block)
			if !ok || b.create {
				continue
			}
			for _, e := range b.entries {
				add(e.sheet)
			}
			if b.group != nil && !b.trashed && b.group.HasGroups {
				for _, sub := range b.groups {
					walkGroup(sub.ID)
				}
			}
		}
		return st
	}
}

// cardView draws cd shaped sh as fr says, the field's origin at at.
func (s *State) cardView(c *ui.Context, semantic theme.Theme, fr stageFrame, cd card, sh shape, at stagePoint, current bool, m blockMetrics, animate bool) {
	scale := fr.Rect.W / max(fr.Size.W, 1)
	// Its frame stands as tall as the change has it, its contents laid out
	// at their own height within: a card growing or shrinking shows more
	// or less of itself, and is never stretched.
	frame := ui.Box(c.Key("card-"+fr.ID)).Absolute().Left(0).Top(0).Size(fr.Size.W, fr.Rect.H/max(scale, 0.01)).
		TransformOrigin(0, 0).Scale(scale, scale).Translate(at.X+fr.Rect.X, at.Y+fr.Rect.Y).
		Opacity(fr.Shown).Radius(12 * m.k).Clip().Role(ui.RoleGroup)
	// The rim is inside the card's edge and takes room from what it holds:
	// the contents stand within it, padded the less, so they are m.pad
	// from the edge all round.
	rim := float32(0)
	if cd.kind.ground() {
		rim = components.Rim
		// Arriving, it is marked, unless it is its own row, which is.
		arriving := float32(0)
		if !slices.Contains(cd.rows, fr.ID) {
			arriving = s.arrivalAt(fr.ID, c.Now())
		}
		frame.Background(components.Blend(semantic.Surface.Page, semantic.Accent.Tint, arriving)).
			Border(rim, semantic.Border.Variant)
	}
	s.field.drawing = cardPlace{rect: fr.Rect, scale: scale, box: frame.LayoutBounds()}
	frame.Children(func() {
		box := ui.Column(c.Key("content")).Size(fr.Size.W-2*rim, fr.Size.H-2*rim).Shrink(0).Padding(m.pad - rim)
		if !cd.kind.drawsLive(s) {
			box.Cache()
		}
		if !current {
			box.IgnoreInput(true)
		}
		cd.kind.view(s, c, semantic, box, fr, sh, current, m, animate)
	})
}

// stack lays rows out in one column under a head, as tall as its half of
// the field holds; what is left out is counted, and a row saying so ends
// the column.
func stack(rows []row, m blockMetrics) shape {
	var sh shape
	var col []row
	colH := float32(0)
	for i, r := range rows {
		if len(col) > 0 && colH+m.rowGap+r.h > m.tall {
			for _, rest := range rows[i:] {
				if !rest.create {
					sh.more++
				}
			}
			for len(col) > 0 && colH+m.rowGap+m.groupRow > m.tall {
				colH -= col[len(col)-1].h + m.rowGap
				if last := col[len(col)-1]; !last.create {
					sh.more++
				}
				col = col[:len(col)-1]
			}
			col = append(col, row{h: m.groupRow})
			colH += m.rowGap + m.groupRow
			break
		}
		if len(col) > 0 {
			colH += m.rowGap
		}
		col = append(col, r)
		colH += r.h
	}
	sh.size = stageSize{W: m.col + 2*m.pad, H: m.head + 2*m.pad}
	if len(col) > 0 {
		sh.cols = [][]row{col}
		sh.size.H += m.headGap + colH
	}
	return sh
}

// cardRows draws sh's rows, column by column, each as draw says.
func cardRows(c *ui.Context, sh shape, m blockMetrics, draw func(row)) {
	if len(sh.cols) == 0 {
		return
	}
	ui.Row(c).Key("cols").AlignItems(ui.Start).Children(func() {
		for i, col := range sh.cols {
			ui.Column(c).Key(fmt.Sprint(i)).Width(m.col).Gap(m.rowGap).Shrink(0).Children(func() {
				for _, r := range col {
					draw(r)
				}
			})
		}
	})
}

// cardHead is a card's head that only says what the card is.
func cardHead(c *ui.Context, semantic theme.Theme, icon components.IconName, title string, m blockMetrics) {
	ui.Row(c).Key("head").Height(m.head).WidthPercent(100).PaddingX(8 * m.k).Gap(8 * m.k).
		AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Icon(c, icon.SVG()).FontSize(15 * m.k).TextColor(semantic.Icon.Muted).Shrink(0)
		components.CenteredLabel(c, title, theme.TextDefault, semantic.Text.Heading).Grow(1).MinWidth(0)
	})
}

// moreLabel says how many rows a card leaves out.
func moreLabel(c *ui.Context, semantic theme.Theme, more int, h float32, m blockMetrics) {
	ui.Row(c).Key("more").Height(h).PaddingX(rowPad(m.scale)).AlignItems(ui.Center).Children(func() {
		components.CenteredLabel(c, locales.More(thousands(more)), theme.TextXSmall, semantic.Text.Muted).FontFeatures("tnum")
	})
}
