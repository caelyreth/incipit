package shell

import (
	"cmp"
	"slices"
	"time"

	"github.com/caelyreth/incipit/internal/text/query"
	"github.com/caelyreth/incipit/internal/ui/components"
	"github.com/caelyreth/incipit/internal/ui/theme"
	"github.com/caelyreth/incipit/locales"
	"github.com/egoist/mygo/ui"
)

// The field never claims an exact order (field-and-desk.md §4.7.5): it
// stands by nearness, in a few steps a person can read, and bands are
// those steps. A band is the cards that stand at one distance from the
// pill, as a lens says what near is: activity, what was written lately
// by the pill and what lies dormant further out; relevance, cut where the
// match falls away; when sheets were changed or made; their tags. The
// field stands each band as a ring of its own, outward from the last
// (stageHorizon.Place), and the list as a section, each under its name.
// What is pinned stands nearest of all in the field, a band of its own,
// the hand's one say in the order.
//
// Bands are chosen, never the default: a default lens of activity moved
// cards on every sheet opened or written, and its rings left rows half
// empty about the pill (tried and dropped). Unchosen, the field stands as
// filed while browsing and as ranked while searching, densely.

// lens is the lens $bands chooses, if any.
func (s *State) lens() string {
	for _, text := range s.field.conditions {
		if c, ok := condition(text); ok && written(c).Key.Name == "bands" {
			return query.Fold(c.Value)
		}
	}
	return ""
}

// facts are what a card's band is read from: for a group's card, what its
// sheets say, at any depth.
type facts struct {
	updated, created time.Time
	score            float64
	tag              string
	pinned           bool
}

// band stands f's results in bands as the lens says, nearest first, each
// in the order the query gave it: f.bands names them, and each card
// carries its own. What has no band of its own, as the area that makes a
// sheet, goes with what stands before it.
func (s *State) band(f *found, now time.Time) {
	lens := s.lens()
	if lens == "" || s.modeFor(f.query) != modeLibrary {
		return
	}
	_, _, browse := s.browsing(query.Parse(f.query))
	var at []int
	var of []facts
	for i, cd := range f.cards {
		if b, ok := cd.kind.(block); ok && cd.tier == tierResult && !b.create {
			at = append(at, i)
			of = append(of, s.factsOf(b))
		}
	}
	if len(at) == 0 {
		return
	}
	bands, names := lensBands(lens, of, !browse, now)
	// Pins stand nearest in the field, before any band; in a list, where
	// they are.
	if browse && !f.listed && slices.ContainsFunc(of, func(x facts) bool { return x.pinned }) {
		for i := range bands {
			bands[i]++
			if of[i].pinned {
				bands[i] = 0
			}
		}
		names = append([]string{locales.BandPinned()}, names...)
	}
	for i := range f.cards {
		f.cards[i].band = -1
	}
	for j, i := range at {
		f.cards[i].band = bands[j]
	}
	last := 0
	for i := range f.cards {
		if f.cards[i].band < 0 {
			f.cards[i].band = last
		}
		last = f.cards[i].band
	}
	slices.SortStableFunc(f.cards, func(a, b card) int {
		return cmp.Or(cmp.Compare(a.tier, b.tier), cmp.Compare(a.band, b.band))
	})
	f.bands = names
}

// factsOf is what b says for its band.
func (s *State) factsOf(b block) facts {
	var x facts
	tags := map[string]int{}
	add := func(sh SheetItem, score float64) {
		if sh.Updated.After(x.updated) {
			x.updated = sh.Updated
		}
		if sh.Created.After(x.created) {
			x.created = sh.Created
		}
		x.score = max(x.score, score)
		if len(sh.Tags) > 0 {
			tags[sh.Tags[0]]++
		}
	}
	for _, e := range b.entries {
		add(e.sheet, e.score)
	}
	if b.single {
		x.pinned = b.entries[0].sheet.Pinned
	}
	// A group browsed holds more than its rows show.
	if b.group != nil && !b.single {
		x.pinned = b.group.Pinned
		var walk func(id string)
		walk = func(id string) {
			for _, sh := range s.Library.Sheets(Place{Kind: PlaceGroup, Group: id}) {
				add(sh, 0)
			}
			for _, g := range s.groups(id) {
				walk(g.ID)
			}
		}
		for _, g := range b.groups {
			walk(g.ID)
		}
	}
	best := 0
	for tag, n := range tags {
		if n > best || n == best && tag < x.tag {
			x.tag, best = tag, n
		}
	}
	return x
}

// lensBands is each of of's band under lens, and the bands' names, nearest
// first; only bands something stands in are named.
func lensBands(lens string, of []facts, searching bool, now time.Time) ([]int, []string) {
	out := make([]int, len(of))
	var names []string
	switch lens {
	case "activity":
		// Written, not opened: a sheet looked at moves nothing.
		names = []string{locales.BandLately(), locales.BandThisMonth(), locales.BandDormant()}
		for i, x := range of {
			age := now.Sub(x.updated)
			switch {
			case age < 7*24*time.Hour:
				out[i] = 0
			case age < 30*24*time.Hour:
				out[i] = 1
			default:
				out[i] = 2
			}
		}
	case "updated", "created":
		names = dateSteps()
		for i, x := range of {
			t := x.updated
			if lens == "created" {
				t = x.created
			}
			out[i] = slices.Index(names, dateStep(t, now))
		}
	case "tag":
		count := map[string]int{}
		for _, x := range of {
			count[x.tag]++
		}
		var tags []string
		for tag := range count {
			if tag != "" {
				tags = append(tags, tag)
			}
		}
		slices.SortFunc(tags, func(a, b string) int { return cmp.Or(count[b]-count[a], cmp.Compare(a, b)) })
		// A few tags stand each in a band; the rest share one.
		const most = 6
		rest := min(len(tags), most)
		names = append(slices.Clone(tags[:rest]), locales.BandOtherTags(), locales.BandNoTag())
		for i, x := range of {
			switch j := slices.Index(tags, x.tag); {
			case x.tag == "":
				out[i] = rest + 1
			case j < most:
				out[i] = j
			default:
				out[i] = rest
			}
		}
	case "relevance":
		if !searching {
			return out, []string{locales.BandFound()}
		}
		names = []string{locales.BandStrong(), locales.BandAlso(), locales.BandMentions()}
		for i, cut := range relevanceCuts(of) {
			out[i] = cut
		}
	default:
		return out, []string{""}
	}
	return compact(out, names)
}

// dateSteps are the steps time is told in, nearest first.
func dateSteps() []string {
	return []string{locales.DateToday(), locales.DateThisWeek(), locales.DateThisMonth(), locales.DateThisYear(), locales.DateEarlier()}
}

// dateStep is the step of dateSteps t falls in, at now.
func dateStep(t, now time.Time) string {
	for i, v := range []string{"today", "this-week", "this-month", "this-year"} {
		if d, _ := query.ParseDates(v, now); d.Contains(t) {
			return dateSteps()[i]
		}
	}
	return locales.DateEarlier()
}

// relevanceCuts is each of of's band by how well it matched, in the order
// found, best first: a band ends where the match falls away most, less
// than fallAway of the one before, at most twice.
func relevanceCuts(of []facts) []int {
	const fallAway = 0.7
	type drop struct {
		at    int
		ratio float64
	}
	var drops []drop
	for i := 1; i < len(of); i++ {
		if prev := of[i-1].score; prev > 0 && of[i].score/prev < fallAway {
			drops = append(drops, drop{i, of[i].score / prev})
		}
	}
	slices.SortStableFunc(drops, func(a, b drop) int { return cmp.Compare(a.ratio, b.ratio) })
	drops = drops[:min(2, len(drops))]
	out := make([]int, len(of))
	for i := range of {
		for _, d := range drops {
			if i >= d.at {
				out[i]++
			}
		}
	}
	return out
}

// compact numbers bands from nearest, leaving out those nothing stands
// in, and names them.
func compact(bands []int, names []string) ([]int, []string) {
	used := make([]bool, len(names))
	for _, b := range bands {
		used[b] = true
	}
	to := make([]int, len(names))
	var kept []string
	for i, u := range used {
		if u {
			to[i] = len(kept)
			kept = append(kept, names[i])
		}
	}
	out := make([]int, len(bands))
	for i, b := range bands {
		out[i] = to[b]
	}
	return out, kept
}

// cardBands is each card's band as they stand, never nearer than one
// before it (what is held in place while writing goes with what stands
// before it until the field settles); nil where the field has none.
func cardBands(f found) []int {
	if f.bands == nil {
		return nil
	}
	out := make([]int, len(f.cards))
	last := 0
	for i, cd := range f.cards {
		last = max(last, cd.band)
		out[i] = last
	}
	return out
}

// bandNames stand each band's name, small and quiet, at the left of the
// ring's room it starts past, the field's origin at at; each comes and
// goes, and moves, as the cards do.
func (s *State) bandNames(c *ui.Context, semantic theme.Theme, f found, marks []stageMark, at stagePoint, m blockMetrics, animate bool) {
	lineH := theme.TextXSmall.LineHeight(m.scale)
	named := map[string]bool{}
	defer func() { s.field.named = named }()
	for _, mk := range marks {
		if mk.Band >= len(f.bands) || f.bands[mk.Band] == "" {
			continue
		}
		name := f.bands[mk.Band]
		x, y := mk.Rect.X+m.pad, mk.Rect.Y+(mk.Rect.H-lineH)/2
		// A band taking both halves is named in each.
		key := name + map[bool]string{true: "-below", false: "-above"}[mk.Below]
		label := ui.Box(c.Key("band-" + key)).Absolute().Height(lineH).IgnoreInput(true).Role(ui.RoleHeading).Label(name)
		shown := float32(1)
		named[key] = true
		if animate {
			// A name new to the field fades in where it stands.
			if !s.field.named[key] {
				label.Jump("x", x)
				label.Jump("y", y)
				label.Jump("shown", 0)
			}
			x = components.Follow(label, "x", x, components.MotionScene)
			y = components.Follow(label, "y", y, components.MotionScene)
			shown = components.Follow(label, "shown", 1, components.MotionQuick)
		} else {
			label.Jump("x", x)
			label.Jump("y", y)
		}
		label.Left(at.X + x).Top(at.Y + y).Opacity(shown).Children(func() {
			components.CenteredLabel(c, name, theme.TextXSmall, semantic.Text.Muted)
		})
	}
}
