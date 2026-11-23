package shell

import (
	"slices"
	"strings"

	"github.com/caelyreth/neverland/internal/text/query"
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo/ui"
)

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

// escapeField is Escape on the field: it puts away what stands open for
// a moment, one thing at a time, the actions, the selection, a key, the
// completion, and then lets go of the pill. It never takes a condition
// or the words away, and never goes anywhere: pressed again and again,
// it is safe. Clearing is the pill's ×, or Home; the desk is ⌘0.
func (s *State) escapeField() {
	switch {
	case s.field.acting:
		s.toggleActions()
	case len(s.field.picked) > 0:
		s.field.picked = nil
	case s.field.opened != nil:
		s.closeKey()
	case s.offered():
		s.field.dismissed = s.field.words
	case s.field.chosen != "":
		s.field.chosen = ""
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
		next := change{depth: stageUp}
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
	s.homeward()
}

// choose moves the keys by by along the rows the field shows.
func (s *State) choose(by int) {
	ids := s.field.order
	if len(ids) == 0 {
		return
	}
	at := slices.Index(ids, s.field.chosen)
	if at < 0 {
		// The keys come onto the field at what ranks first.
		s.field.chosen = s.field.ranked[0]
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
		if id, ok := strings.CutPrefix(key, actionCommand); ok {
			s.runActionCommand(id)
		} else if s.RunCommand != nil {
			s.RunCommand(key)
		}
	case "group":
		if g, ok := s.Library.Group(key); ok {
			s.into(g, stageRect{})
		}
	case "settings-cat":
		s.intoSettings(key, "", stageRect{})
	case "settings-sec":
		category, _, _ := strings.Cut(key, "/")
		s.intoSettings(category, key, stageRect{})
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

// fieldMenu is what the field's ground offers.
func (s *State) fieldMenu(m *ui.Menu) {
	if s.Library == nil {
		return
	}
	// The place's actions, as the pill's button opens them; what asks
	// again opens them, ready.
	acts := s.fieldActions()
	s.actionsMenu(m, acts)
	if len(acts) > 0 {
		m.Separator()
	}
	s.verbItem(m, VerbShowTrash, map[bool]string{true: locales.LeaveTrash(), false: locales.ShowTrash()}[s.inTrash()], true, s.toggleTrash)
	s.verbItem(m, VerbViewList, s.listLabel(), true, s.toggleList)
	if g, ok := s.scopeGroup(); ok && !s.inTrash() {
		s.keepItem(m, g)
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
		if !s.leave("trash") {
			s.broaden(i)
		}
		return
	}
	s.field.conditions = nil
	s.setWords("")
	s.narrow((&query.Condition{Key: mustKey("trash")}).String(), stageRect{})
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
