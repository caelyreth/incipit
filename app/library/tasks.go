package library

// A sheet's tasks are counted from its text, open and done, and handed to
// the library (SetTasks), which keeps for each group what its whole tree
// holds: its own sheets' and, through each of its groups, theirs. The
// sums are kept as the library changes, never by walking it again: what
// a change adds or takes away is spread up the chain of groups above it
// (spread), as far as the first group in the trash, which keeps its own
// sum for when it comes back. A sheet in the trash on its own counts for
// no group. A smart folder holds nothing, so it counts none.

// Tasks counts tasks, open and done.
type Tasks struct{ Open, Done int }

// All is how many tasks there are.
func (t Tasks) All() int { return t.Open + t.Done }

func (t Tasks) plus(o Tasks) Tasks  { return Tasks{t.Open + o.Open, t.Done + o.Done} }
func (t Tasks) minus(o Tasks) Tasks { return Tasks{t.Open - o.Open, t.Done - o.Done} }

// SetTasks keeps the tasks a sheet's text holds, which its groups count;
// a sheet not filed yet counts as it is filed.
func (l *Library) SetTasks(id SheetID, tasks Tasks) {
	was := l.tasks[id]
	if tasks == was {
		return
	}
	l.rev++
	l.touches++
	if tasks == (Tasks{}) {
		delete(l.tasks, id)
	} else {
		l.tasks[id] = tasks
	}
	if sheet, ok := l.sheets[id]; ok && sheet.Trashed.IsZero() {
		l.spread(sheet.Group, tasks.minus(was))
	}
}

// Tasks is the tasks a sheet's text holds.
func (l *Library) Tasks(id SheetID) Tasks { return l.tasks[id] }

// GroupTasks is the tasks a group's whole tree holds, out of the trash.
func (l *Library) GroupTasks(id GroupID) Tasks { return l.groupTasks[id] }

// spread adds by to group and the groups above it, up to the first in the
// trash.
func (l *Library) spread(group GroupID, by Tasks) {
	if by == (Tasks{}) {
		return
	}
	for id := group; id != 0; {
		g, ok := l.groups[id]
		if !ok {
			return
		}
		if sum := l.groupTasks[id].plus(by); sum == (Tasks{}) {
			delete(l.groupTasks, id)
		} else {
			l.groupTasks[id] = sum
		}
		if !g.Trashed.IsZero() {
			return
		}
		id = g.Parent
	}
}

// sheetCounts is what a sheet adds to its group: its tasks, unless it is
// in the trash on its own.
func (l *Library) sheetCounts(sheet *Sheet) Tasks {
	if !sheet.Trashed.IsZero() {
		return Tasks{}
	}
	return l.tasks[sheet.ID]
}

// groupCounts is what a group adds to its parent: its tree's tasks, unless
// it is in the trash itself.
func (l *Library) groupCounts(group *Group) Tasks {
	if !group.Trashed.IsZero() {
		return Tasks{}
	}
	return l.groupTasks[group.ID]
}
