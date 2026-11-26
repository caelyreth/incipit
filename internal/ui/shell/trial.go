package shell

import (
	"time"

	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo/ui"
)

// Trial is a piece of the field to try its menus on, as the onboarding
// does: one group's card drawn as the field draws it, holding a group and
// two sheets of sample words, its head and rows taking the field's own
// menus. Its library is the card's alone: a pin, a color or a name chosen
// shows on the card, and nothing reaches a notebook.
type Trial struct {
	s   *State
	lib *trialLibrary
	// block is what the card holds, and shaped its shape, for the measure
	// it was shaped at; both are made again as either changes.
	block    block
	shaped   shape
	shapedAt [2]float32
}

// shape makes the card's block from its library, and its shape at m: the
// group in it, then its sheets, the pinned first, as the field stands
// them, with no area to make one, which would leave the trial.
func (t *Trial) shape(c *ui.Context, m blockMetrics) {
	g := t.lib.group
	b := block{group: &g, groups: []GroupItem{t.lib.sub}}
	for _, sh := range t.lib.Sheets(Place{Kind: PlaceGroup, Group: g.ID}) {
		b.entries = append(b.entries, entry{sheet: sh})
	}
	rows := make([]row, 0, len(b.groups)+len(b.entries))
	pinnedFirst(b.groups, b.entries, func(i int) {
		rows = append(rows, row{group: &b.groups[i], h: m.groupRow})
	}, func(i int) {
		rows = append(rows, row{entry: &b.entries[i], h: entryHeight(c, b.entries[i], m, 2)})
	})
	t.block, t.shaped = b, stack(rows, m)
}

// NewTrial makes the card, its sheets written at made.
func NewTrial(made time.Time) *Trial {
	lib := newTrialLibrary(made)
	return &Trial{s: &State{Library: lib}, lib: lib}
}

// View draws the card width across at settings' type size, and returns
// it.
func (t *Trial) View(c *ui.Context, semantic theme.Theme, settings theme.Settings, width float32, animate bool) ui.Element {
	s := t.s
	s.Settings = settings
	// The field's measures, its column the card's width and its rows as
	// tall as they come.
	k := settings.UIFontSize / theme.DefaultUIFontSize
	m := s.columnMetrics(width+48*k, 4000, width/k, width/k)
	m.col = width - 2*m.pad
	if at := [2]float32{width, settings.UIFontSize}; t.shapedAt != at || t.lib.changed {
		t.shape(c, m)
		t.shapedAt, t.lib.changed = at, false
	}
	b, sh, g := t.block, t.shaped, t.block.group
	rim := components.Rim
	frame := ui.Box(c.Key("trial")).Size(sh.size.W, sh.size.H).Shrink(0).Radius(12*m.k).Clip().
		Role(ui.RoleGroup).Label(g.Title()).
		Background(semantic.Surface.Page).Border(rim, semantic.Border.Variant)
	fr := stageFrame{ID: "trial", Size: sh.size, Rect: stageRect{W: sh.size.W, H: sh.size.H}, Shown: 1}
	frame.Children(func() {
		box := ui.Column(c.Key("content")).Size(sh.size.W-2*rim, sh.size.H-2*rim).Shrink(0).Padding(m.pad - rim)
		b.view(s, c, semantic, box, fr, sh, true, m, animate)
	})
	return frame
}

// trialLibrary is the trial card's library: one group, a group in it,
// and two sheets, which keep what is chosen on them while the card is
// shown; what would make, move or remove anything does nothing.
type trialLibrary struct {
	group, sub GroupItem
	sheets     []SheetItem
	rev        uint64
	// changed is set as what the card holds changes, to shape it again.
	changed bool
}

func newTrialLibrary(made time.Time) *trialLibrary {
	return &trialLibrary{
		group: GroupItem{ID: "trial", Name: locales.OnboardingTrialGroup(), Count: 14, HasGroups: true, Color: TintHalo},
		sub:   GroupItem{ID: "trial-sub", Name: locales.OnboardingTrialSubgroup(), Count: 6, Color: TintHalo + 3},
		sheets: []SheetItem{
			{ID: "trial-first", Title: locales.OnboardingTrialFirst(), Excerpt: locales.OnboardingTrialFirstExcerpt(),
				Created: made.Add(-26 * time.Hour), Updated: made.Add(-25 * time.Minute), Words: 412, Tasks: Tasks{Open: 1, Done: 2}},
			{ID: "trial-second", Title: locales.OnboardingTrialSecond(), Excerpt: locales.OnboardingTrialSecondExcerpt(),
				Created: made.Add(-96 * time.Hour), Updated: made.Add(-50 * time.Hour), Words: 186},
		},
	}
}

func (l *trialLibrary) touch() { l.rev++; l.changed = true }

func (l *trialLibrary) groupPtr(id string) *GroupItem {
	switch id {
	case l.group.ID:
		return &l.group
	case l.sub.ID:
		return &l.sub
	}
	return nil
}

func (l *trialLibrary) Groups(parent string) []GroupItem {
	switch parent {
	case "":
		return []GroupItem{l.group}
	case l.group.ID:
		return []GroupItem{l.sub}
	}
	return nil
}

func (l *trialLibrary) Group(id string) (GroupItem, bool) {
	if g := l.groupPtr(id); g != nil {
		return *g, true
	}
	return GroupItem{}, false
}

func (l *trialLibrary) Sheets(place Place) []SheetItem {
	if place.Kind == PlaceGroup && place.Group != l.group.ID {
		return nil
	}
	return append([]SheetItem(nil), l.sheets...)
}

func (l *trialLibrary) Revision() uint64                                    { return l.rev }
func (l *trialLibrary) Search(string) Found                                 { return Found{} }
func (l *trialLibrary) SearchNames(string) []string                         { return nil }
func (l *trialLibrary) CountValues(string, string, []string) map[string]int { return nil }
func (l *trialLibrary) OpenFound(string, string, bool)                      {}
func (l *trialLibrary) TrashedGroups() []GroupItem                          { return nil }

func (l *trialLibrary) Targets() []MoveTarget {
	return []MoveTarget{{ID: l.group.ID, Group: l.group}, {ID: l.sub.ID, Group: l.sub, Depth: 1}}
}

func (l *trialLibrary) SheetGroup(string) string { return l.group.ID }
func (l *trialLibrary) NewSheet(string, int)     {}
func (l *trialLibrary) OpenSheet(string, bool)   {}
func (l *trialLibrary) DuplicateSheet(string) string {
	return ""
}

func (l *trialLibrary) Pin(id string, pinned bool) bool {
	if g := l.groupPtr(trimKind(id, "group:")); g != nil {
		g.Pinned = pinned
	}
	for i := range l.sheets {
		if "sheet:"+l.sheets[i].ID == id {
			l.sheets[i].Pinned = pinned
		}
	}
	l.touch()
	return true
}

func (l *trialLibrary) MoveSheet(string, string, int) bool { return false }
func (l *trialLibrary) TrashSheet(string) bool             { return false }
func (l *trialLibrary) RestoreSheet(string) bool           { return false }
func (l *trialLibrary) DeleteSheet(string)                 {}
func (l *trialLibrary) NewGroup(string, int) string        { return "" }

func (l *trialLibrary) RenameGroup(id, name string) bool {
	if g := l.groupPtr(id); g != nil {
		g.Name = name
		l.touch()
		return true
	}
	return false
}

func (l *trialLibrary) NewSmartGroup(string, string) string { return "" }

func (l *trialLibrary) StyleGroup(id, icon string, color int) bool {
	if g := l.groupPtr(id); g != nil {
		g.Icon, g.Color = icon, color
		l.touch()
		return true
	}
	return false
}

func (l *trialLibrary) KeepAsList(id string, listed bool) bool {
	if g := l.groupPtr(id); g != nil {
		g.Listed = listed
		l.touch()
		return true
	}
	return false
}

func (l *trialLibrary) MoveGroup(string, string, int) bool { return false }
func (l *trialLibrary) TrashGroup(string) bool             { return false }
func (l *trialLibrary) RestoreGroup(string) bool           { return false }
func (l *trialLibrary) DeleteGroup(string)            {}
func (l *trialLibrary) EmptyTrash()                   {}
func (l *trialLibrary) Export(string)                 {}

// trimKind is id without its kind's prefix, or id where it has another.
func trimKind(id, kind string) string {
	if len(id) > len(kind) && id[:len(kind)] == kind {
		return id[len(kind):]
	}
	return id
}
