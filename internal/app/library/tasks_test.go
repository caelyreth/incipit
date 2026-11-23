package library

import (
	"math/rand/v2"
	"testing"
)

// recount is a group's tree's tasks counted afresh: its sheets out of the
// trash on their own, and its groups out of the trash themselves.
func (l *Library) recount(id GroupID) Tasks {
	var sum Tasks
	for _, sheet := range l.order[id] {
		if s := l.sheets[sheet]; s.Trashed.IsZero() {
			sum = sum.plus(l.tasks[sheet])
		}
	}
	for _, child := range l.children[id] {
		if l.groups[child].Trashed.IsZero() {
			sum = sum.plus(l.recount(child))
		}
	}
	return sum
}

// A group's tasks are its tree's, kept as sheets are counted, filed,
// moved, trashed, restored and deleted, and groups moved, trashed,
// restored and deleted, whatever the order: always what counting them
// afresh finds.
func TestAGroupsTasksFollowEveryChange(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	l := New()
	groups := []GroupID{Inbox}
	var sheets []SheetID
	pick := func() GroupID { return groups[r.IntN(len(groups))] }
	for step := range 4000 {
		switch r.IntN(11) {
		case 0:
			if id, err := l.AddGroup(pick(), "g"); err == nil {
				groups = append(groups, id)
			}
		case 1:
			id := SheetID(len(sheets) + 1)
			if l.AddSheet(id, pick(), now) == nil {
				sheets = append(sheets, id)
			}
		case 2, 3:
			if len(sheets) > 0 {
				l.SetTasks(sheets[r.IntN(len(sheets))], Tasks{Open: r.IntN(5), Done: r.IntN(5)})
			}
		case 4:
			if len(sheets) > 0 {
				_ = l.MoveSheet(sheets[r.IntN(len(sheets))], pick(), r.IntN(4))
			}
		case 5:
			if len(sheets) > 0 {
				_ = l.TrashSheet(sheets[r.IntN(len(sheets))], now)
			}
		case 6:
			if len(sheets) > 0 {
				_ = l.RestoreSheet(sheets[r.IntN(len(sheets))])
			}
		case 7:
			_ = l.MoveGroup(pick(), pick(), r.IntN(4))
		case 8:
			_ = l.TrashGroup(pick(), now)
		case 9:
			_ = l.RestoreGroup(pick())
		case 10:
			if r.IntN(4) == 0 {
				l.EmptyTrash()
			}
		}
		for id := range l.groups {
			if got, want := l.GroupTasks(id), l.recount(id); got != want {
				t.Fatalf("step %d: group %d holds %+v, counted afresh %+v", step, id, got, want)
			}
		}
	}
}

// A sheet counted before it is filed counts in its group once filed.
func TestASheetCountedBeforeItIsFiledCountsOnceFiled(t *testing.T) {
	l := New()
	l.SetTasks(7, Tasks{Open: 2, Done: 1})
	if got := l.GroupTasks(Inbox); got != (Tasks{}) {
		t.Fatalf("nothing is filed yet: %+v", got)
	}
	if err := l.AddSheet(7, Inbox, now); err != nil {
		t.Fatal(err)
	}
	if got := l.GroupTasks(Inbox); got != (Tasks{Open: 2, Done: 1}) {
		t.Fatalf("filed, it counts: %+v", got)
	}
}
