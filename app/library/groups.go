package library

import (
	"slices"
	"strings"
	"time"
)

// AddGroup makes a group at the end of parent.
func (l *Library) AddGroup(parent GroupID, name string) (GroupID, error) {
	return l.AddGroupAt(parent, name, len(l.children[parent]))
}

// AddGroupAt makes a group in parent at index, clamped to its groups. It
// inserts the group and its placement together, so a failed call changes
// neither.
func (l *Library) AddGroupAt(parent GroupID, name string, index int) (GroupID, error) {
	l.rev++
	if parent != 0 {
		if err := l.fileable(parent); err != nil {
			return 0, err
		}
	}
	id := l.nextGroup
	l.nextGroup++
	l.groups[id] = &Group{ID: id, Parent: parent, Name: groupName(name)}
	l.children[parent] = insertAt(l.children[parent], index, id)
	return id, nil
}

// AddSmartGroupAt makes a smart folder of a query in parent at index, as
// AddGroupAt makes a group.
func (l *Library) AddSmartGroupAt(parent GroupID, name, query string, index int) (GroupID, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		l.rev++
		return 0, ErrQuery
	}
	id, err := l.AddGroupAt(parent, name, index)
	if err == nil {
		l.groups[id].Query = query
	}
	return id, err
}

// SetGroupQuery sets a smart folder's query.
func (l *Library) SetGroupQuery(id GroupID, query string) error {
	l.rev++
	group, ok := l.groups[id]
	query = strings.TrimSpace(query)
	switch {
	case !ok:
		return ErrUnknown
	case !group.Smart() || query == "":
		return ErrQuery
	}
	group.Query = query
	return nil
}

// fileable reports why a group cannot take what is filed in it: sheets and
// groups. Nil is a group that can.
func (l *Library) fileable(id GroupID) error {
	group, ok := l.groups[id]
	switch {
	case !ok:
		return ErrUnknown
	case l.GroupInTrash(id):
		return ErrTrashed
	case group.Smart():
		return ErrSmart
	}
	return nil
}

func groupName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Untitled Group"
	}
	return name
}

// RenameGroup names a group; a blank name is "Untitled Group".
func (l *Library) RenameGroup(id GroupID, name string) error {
	l.rev++
	group, ok := l.groups[id]
	switch {
	case !ok:
		return ErrUnknown
	case id == Inbox:
		return ErrInbox
	}
	group.Name = groupName(name)
	return nil
}

// StyleGroup sets a group's icon and tint.
func (l *Library) StyleGroup(id GroupID, icon string, color uint8) error {
	l.rev++
	group, ok := l.groups[id]
	switch {
	case !ok:
		return ErrUnknown
	case id == Inbox:
		return ErrInbox
	}
	group.Icon, group.Color = icon, color
	return nil
}

// MoveGroup puts a group in parent at index, clamped to its groups; moving
// within the same parent reorders it.
func (l *Library) MoveGroup(id, parent GroupID, index int) error {
	l.rev++
	group, ok := l.groups[id]
	switch {
	case !ok:
		return ErrUnknown
	case id == Inbox:
		return ErrInbox
	}
	if parent != 0 {
		if err := l.fileable(parent); err != nil {
			return err
		}
	}
	for at := parent; at != 0; at = l.groups[at].Parent {
		if at == id {
			return ErrCycle
		}
	}
	l.children[group.Parent] = slices.DeleteFunc(l.children[group.Parent], func(other GroupID) bool { return other == id })
	group.Parent = parent
	l.children[parent] = insertAt(l.children[parent], index, id)
	return nil
}

// TrashGroup puts a group, and all it holds, in the trash.
func (l *Library) TrashGroup(id GroupID, at time.Time) error {
	l.rev++
	group, ok := l.groups[id]
	switch {
	case !ok:
		return ErrUnknown
	case id == Inbox:
		return ErrInbox
	}
	if group.Trashed.IsZero() {
		group.Trashed = at
	}
	return nil
}

// RestoreGroup takes a group out of the trash. One whose parent is still
// in the trash, or gone, comes back at the top level.
func (l *Library) RestoreGroup(id GroupID) error {
	l.rev++
	group, ok := l.groups[id]
	if !ok {
		return ErrUnknown
	}
	group.Trashed = time.Time{}
	if l.GroupInTrash(group.Parent) {
		return l.MoveGroup(id, 0, len(l.children[0]))
	}
	return nil
}

// PinGroup pins a group, or lets its pin go.
func (l *Library) PinGroup(id GroupID, pinned bool) error {
	l.rev++
	group, ok := l.groups[id]
	if !ok {
		return ErrUnknown
	}
	group.Pinned = pinned
	return nil
}

// DeleteGroup removes a group in the trash for good, with everything it
// holds, and returns the sheets that went with it.
func (l *Library) DeleteGroup(id GroupID) ([]SheetID, error) {
	l.rev++
	group, ok := l.groups[id]
	if !ok {
		return nil, ErrUnknown
	}
	if !l.GroupInTrash(id) {
		return nil, ErrNotTrashed
	}
	var deleted []SheetID
	var remove func(GroupID)
	remove = func(id GroupID) {
		for _, child := range slices.Clone(l.children[id]) {
			remove(child)
		}
		for _, sheet := range l.order[id] {
			delete(l.sheets, sheet)
			deleted = append(deleted, sheet)
		}
		delete(l.order, id)
		delete(l.children, id)
		delete(l.groups, id)
	}
	l.children[group.Parent] = slices.DeleteFunc(l.children[group.Parent], func(other GroupID) bool { return other == id })
	remove(id)
	return deleted, nil
}
