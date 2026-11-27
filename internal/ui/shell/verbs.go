package shell

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/caelyreth/incipit/internal/ui/components"
	"github.com/caelyreth/incipit/locales"
)

// The field's verbs, as the menu bar, their keys and the rows' menus all
// reach them: each is declared once in the app's keymap (internal/app/keymap) and
// answered here, by its command's ID, for what the field shows now. A verb
// acts on its subject: what is selected, or, with nothing selected, the
// row the keys are on.

// Verb IDs, the keymap's command IDs for what the field answers.
const (
	VerbNewGroup   = "library.new-group"
	VerbDuplicate  = "library.duplicate"
	VerbRename     = "library.rename"
	VerbPin        = "library.pin"
	VerbTrash      = "library.trash"
	VerbEmptyTrash = "library.empty-trash"
	VerbViewList   = "library.view"
	VerbBack       = "go.back"
	VerbForward    = "go.forward"
	VerbHome       = "go.home"
	VerbShowTrash  = "go.trash"
	// verbExport is the window's, not the field's: a menu only shows its
	// key.
	verbExport = "page.export"
)

// VerbState is how a verb stands now: what it is called, whether it can
// be done, and whether it is on.
type VerbState struct {
	Label   string
	Enabled bool
	Checked bool
}

// KeyOf is the key a command runs on, for a menu to show: none where
// it has none.
type KeyOf func(id string) (ui.Modifiers, ui.Key, bool)

// subject is what a verb acts on: the selection, or the row the keys are
// on where it is a group or a sheet.
func (s *State) subject() []string {
	if len(s.field.picked) > 0 {
		return s.field.picked
	}
	if c := s.field.chosen; strings.HasPrefix(c, "group:") || strings.HasPrefix(c, "sheet:") {
		return []string{c}
	}
	return nil
}

// ExportSubject is what Export acts on: the sheet in front on the desk,
// else the one group or sheet that is the verbs' subject, out of the
// trash.
func (s *State) ExportSubject() (string, bool) {
	if s == nil || s.Library == nil {
		return "", false
	}
	if s.OnDesk() {
		for _, page := range s.Open {
			if page.Column == s.ActiveColumn {
				return "sheet:" + page.ID, true
			}
		}
		return "", false
	}
	subject := s.subject()
	if !s.libraryShown() || s.inTrash() || len(subject) != 1 {
		return "", false
	}
	return subject[0], true
}

// libraryShown reports whether the field shows the library, which the
// library's verbs act on.
func (s *State) libraryShown() bool {
	return s.Library != nil && s.mode() == modeLibrary
}

// Verb is how the verb id stands for what the field shows.
func (s *State) Verb(id string) VerbState {
	if s == nil {
		return VerbState{}
	}
	subject := s.subject()
	one := len(subject) == 1
	switch id {
	case VerbNewGroup:
		return VerbState{Label: locales.NewGroup(), Enabled: s.placeAction("new-group") != nil}
	case VerbDuplicate:
		sheets := bare(subject, "sheet:")
		return VerbState{Label: locales.Duplicate(), Enabled: s.libraryShown() && !s.inTrash() &&
			len(sheets) > 0 && len(sheets) == len(subject)}
	case VerbRename:
		ok := false
		if g, found := s.subjectGroup(); found && one && !s.inTrash() {
			ok = !g.Inbox && !g.Smart()
		}
		return VerbState{Label: locales.Rename(), Enabled: ok}
	case VerbPin:
		pinned, ok := false, false
		if one && s.libraryShown() && !s.inTrash() {
			pinned, ok = s.pinnedNow(subject[0])
		}
		return VerbState{Label: map[bool]string{true: locales.Unpin(), false: locales.Pin()}[pinned], Enabled: ok, Checked: pinned}
	case VerbTrash:
		label := locales.MoveToTrash()
		if s.inTrash() {
			label = locales.PutBack()
		}
		return VerbState{Label: label, Enabled: s.libraryShown() && len(subject) > 0 && !s.holdsInbox(subject)}
	case VerbEmptyTrash:
		holds := s.Library != nil && len(s.Library.TrashedGroups())+len(s.Library.Sheets(Place{Kind: PlaceTrash})) > 0
		return VerbState{Label: locales.EmptyTrash(), Enabled: holds}
	case VerbViewList:
		listed := s.listed()
		return VerbState{Label: map[bool]string{true: locales.ViewAsField(), false: locales.ViewAsList()}[listed],
			Enabled: s.libraryShown() && !s.inTrash(), Checked: listed}
	case VerbBack:
		return VerbState{Label: locales.Back(), Enabled: s.canGo(-1)}
	case VerbForward:
		return VerbState{Label: locales.Forward(), Enabled: s.canGo(1)}
	case VerbHome:
		return VerbState{Label: locales.Home(), Enabled: s.Library != nil}
	case VerbShowTrash:
		return VerbState{Label: locales.Trash(), Enabled: s.Library != nil, Checked: s.inTrash() && !s.OnDesk()}
	}
	return VerbState{}
}

// DoVerb does the verb id, where it can be done.
func (s *State) DoVerb(id string) {
	if s == nil || !s.Verb(id).Enabled {
		return
	}
	subject := s.subject()
	switch id {
	case VerbNewGroup:
		if a := s.placeAction("new-group"); a != nil {
			a.run()
		}
	case VerbDuplicate:
		s.duplicate(bare(subject, "sheet:"))
	case VerbRename:
		if g, ok := s.subjectGroup(); ok {
			s.startRename(g.ID)
		}
	case VerbPin:
		pinned, _ := s.pinnedNow(subject[0])
		s.pin(subject[0], !pinned)
	case VerbTrash:
		s.trashOrPutBack(subject)
	case VerbEmptyTrash:
		// Empty Trash asks again where it is seen: in the trash, its
		// action turned, waiting for the second press.
		s.ShowField()
		if !s.inTrash() {
			s.toggleTrash()
		}
		s.field.acting, s.field.actionAt = true, 0
		s.closeKey()
		s.arm(emptyTrashKey)
	case VerbViewList:
		s.toggleList()
	case VerbBack:
		s.goPlace(-1)
	case VerbForward:
		s.goPlace(1)
	case VerbHome:
		s.ShowField()
		s.goHome()
	case VerbShowTrash:
		s.ShowField()
		s.toggleTrash()
	}
}

// verbItem adds the verb id to a menu, under its own name or label, beside
// icon, with its key, done on the subject the menu was opened for.
func (s *State) verbItem(m *ui.Menu, id, label string, icon components.IconName, enabled bool, run func()) {
	it := components.MenuItem(m.Item(label), icon).ID(id).Disabled(!enabled)
	// Emptying the trash is the one verb that is not undone.
	if id == VerbEmptyTrash {
		it.Danger()
	}
	if s.KeyOf != nil {
		if mods, key, ok := s.KeyOf(id); ok {
			it.Shortcut(mods, key)
		}
	}
	if it.Chosen() && enabled {
		run()
	}
}

// placeAction is the place's action named id, if the place has it.
func (s *State) placeAction(id string) *fieldAction {
	if !s.libraryShown() {
		return nil
	}
	for _, a := range s.placeActions() {
		if a.id == id {
			return &a
		}
	}
	return nil
}

// subjectGroup is the one group the subject is.
func (s *State) subjectGroup() (GroupItem, bool) {
	groups := bare(s.subject(), "group:")
	if len(groups) != 1 || len(s.subject()) != 1 {
		return GroupItem{}, false
	}
	return s.shownGroup(groups[0])
}

// pinnedNow reports whether the object id is pinned, and whether it is
// one the field shows.
func (s *State) pinnedNow(id string) (pinned, ok bool) {
	if g, found := strings.CutPrefix(id, "group:"); found {
		item, ok := s.shownGroup(g)
		return item.Pinned, ok && !item.Inbox
	}
	if sh, found := strings.CutPrefix(id, "sheet:"); found {
		item, ok := s.shownSheet(sh)
		return item.Pinned, ok
	}
	return false, false
}

// holdsInbox reports whether ids hold the Inbox, which nothing moves.
func (s *State) holdsInbox(ids []string) bool {
	for _, g := range bare(ids, "group:") {
		if item, ok := s.shownGroup(g); ok && item.Inbox {
			return true
		}
	}
	return false
}

// trashOrPutBack puts the rows ids in the trash, or, in the trash, back
// where they were: one notice, one Undo.
func (s *State) trashOrPutBack(ids []string) {
	groups := func() []GroupItem {
		var out []GroupItem
		for _, id := range bare(ids, "group:") {
			if g, ok := s.shownGroup(id); ok {
				out = append(out, g)
			}
		}
		return out
	}()
	if s.inTrash() {
		s.together(locales.PutBackWhat(s.objectsNamed(ids)), func() {
			s.restoreSheets(bare(ids, "sheet:"))
			for _, g := range groups {
				s.restoreGroup(g)
			}
		})
	} else {
		s.together(locales.MovedToTrash(s.objectsNamed(ids)), func() {
			s.trashSheets(bare(ids, "sheet:"))
			for _, g := range groups {
				s.trashGroup(g)
			}
		})
	}
	s.field.picked, s.field.chosen = nil, ""
}

// binding is the command id's key spelled, for a hint: "" for none.
func (s *State) binding(id string) string {
	if s.Binding == nil {
		return ""
	}
	return s.Binding(id)
}
