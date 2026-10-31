package library

import (
	"errors"
	"slices"
	"testing"
	"time"
)

var now = time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)

func ids[T any, K comparable](items []T, id func(T) K) []K {
	var out []K
	for _, item := range items {
		out = append(out, id(item))
	}
	return out
}

func groupIDs(groups []Group) []GroupID { return ids(groups, func(g Group) GroupID { return g.ID }) }
func sheetIDs(sheets []Sheet) []SheetID { return ids(sheets, func(s Sheet) SheetID { return s.ID }) }

func equal[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestGroupsNestAndKeepTheirOrder(t *testing.T) {
	l := New()
	novel, _ := l.AddGroup(0, "Novel")
	part, _ := l.AddGroup(novel, "Part one")
	notes, _ := l.AddGroup(0, " ")
	if g, _ := l.Group(notes); g.Name != "Untitled Group" {
		t.Fatalf("blank name %q", g.Name)
	}
	if got := groupIDs(l.Groups(0)); !equal(got, []GroupID{Inbox, novel, notes}) {
		t.Fatalf("top level %v", got)
	}
	must(t, l.MoveGroup(notes, 0, 1))
	if got := groupIDs(l.Groups(0)); !equal(got, []GroupID{Inbox, notes, novel}) {
		t.Fatalf("reordered %v", got)
	}
	if err := l.MoveGroup(novel, part, 0); !errors.Is(err, ErrCycle) {
		t.Fatalf("moved into its own group: %v", err)
	}
	for _, err := range []error{l.RenameGroup(Inbox, "x"), l.MoveGroup(Inbox, novel, 0), l.TrashGroup(Inbox, now)} {
		if !errors.Is(err, ErrInbox) {
			t.Fatalf("changed the Inbox: %v", err)
		}
	}
	if path := groupIDs(l.Path(part)); !equal(path, []GroupID{novel, part}) {
		t.Fatalf("path %v", path)
	}
}

func TestSheetsAreFiledMovedAndCounted(t *testing.T) {
	l := New()
	novel, _ := l.AddGroup(0, "Novel")
	part, _ := l.AddGroup(novel, "Part one")
	must(t, l.AddSheet(1, Inbox, now))
	must(t, l.AddSheet(2, novel, now))
	must(t, l.AddSheet(3, part, now))
	must(t, l.MoveSheet(1, novel, 0))
	if got := sheetIDs(l.Sheets(novel)); !equal(got, []SheetID{1, 2}) {
		t.Fatalf("novel sheets %v", got)
	}
	if l.Count(novel) != 3 || l.Count(Inbox) != 0 {
		t.Fatalf("counts %d %d", l.Count(novel), l.Count(Inbox))
	}
	l.Touch(3, now.Add(time.Hour))
	if got := sheetIDs(l.Live()); got[0] != 3 {
		t.Fatalf("latest first %v", got)
	}
	if got := sheetIDs(l.Since(now.Add(time.Minute))); !equal(got, []SheetID{3}) {
		t.Fatalf("since %v", got)
	}
}

func TestAllUsesCreatedTimeWhileRecentUsesUpdatedTime(t *testing.T) {
	l := New()
	must(t, l.AddSheet(1, Inbox, now))
	must(t, l.AddSheet(2, Inbox, now.Add(time.Hour)))
	must(t, l.AddSheet(3, Inbox, now.Add(2*time.Hour)))
	l.Touch(1, now.Add(3*time.Hour))
	if got := sheetIDs(l.All()); !equal(got, []SheetID{3, 2, 1}) {
		t.Fatalf("All order = %v, want newest created first", got)
	}
	if got := sheetIDs(l.Since(now)); !equal(got, []SheetID{1, 3, 2}) {
		t.Fatalf("Recent order = %v, want latest updated first", got)
	}
}

func TestTheTrashKeepsWhatIsPutAwayUntilEmptied(t *testing.T) {
	l := New()
	novel, _ := l.AddGroup(0, "Novel")
	part, _ := l.AddGroup(novel, "Part one")
	must(t, l.AddSheet(1, novel, now))
	must(t, l.AddSheet(2, part, now))
	must(t, l.AddSheet(3, Inbox, now))
	must(t, l.TrashSheet(3, now))
	must(t, l.TrashGroup(novel, now.Add(time.Second)))
	if !l.SheetInTrash(2) || len(l.Groups(0)) != 1 || len(l.Live()) != 0 {
		t.Fatal("a trashed group's sheets are still out")
	}
	groups, sheets := l.Trash()
	if !equal(groupIDs(groups), []GroupID{novel}) || !equal(sheetIDs(sheets), []SheetID{3}) {
		t.Fatalf("trash %v %v", groupIDs(groups), sheetIDs(sheets))
	}
	if err := l.DeleteSheet(1); err != nil {
		t.Fatalf("a sheet in a trashed group is in the trash: %v", err)
	}
	if err := l.AddSheet(9, part, now); !errors.Is(err, ErrTrashed) {
		t.Fatalf("filed into the trash: %v", err)
	}
	must(t, l.RestoreSheet(3))
	if l.SheetInTrash(3) {
		t.Fatal("restored sheet is in the trash")
	}
	deleted := l.EmptyTrash()
	if !equal(deleted, []SheetID{2}) || len(l.Groups(0)) != 1 {
		t.Fatalf("emptied %v", deleted)
	}
	if _, ok := l.Group(part); ok {
		t.Fatal("a deleted group's group remains")
	}
	if err := l.DeleteSheet(3); !errors.Is(err, ErrNotTrashed) {
		t.Fatalf("deleted a live sheet: %v", err)
	}
}

func TestRestoringOutOfATrashedGroupComesBackToTheTop(t *testing.T) {
	l := New()
	novel, _ := l.AddGroup(0, "Novel")
	part, _ := l.AddGroup(novel, "Part one")
	must(t, l.AddSheet(1, novel, now))
	must(t, l.TrashSheet(1, now))
	must(t, l.TrashGroup(part, now))
	must(t, l.TrashGroup(novel, now))
	must(t, l.RestoreSheet(1))
	if sheet, _ := l.Sheet(1); sheet.Group != Inbox || l.SheetInTrash(1) {
		t.Fatalf("restored sheet in %d", sheet.Group)
	}
	must(t, l.RestoreGroup(part))
	if group, _ := l.Group(part); group.Parent != 0 || l.GroupInTrash(part) {
		t.Fatalf("restored group in %d", group.Parent)
	}
}

func TestRecordsRestoreTheLibrary(t *testing.T) {
	l := New()
	novel, _ := l.AddGroup(0, "Novel")
	part, _ := l.AddGroup(novel, "Part one")
	notes, _ := l.AddGroup(0, "Notes")
	must(t, l.MoveGroup(notes, 0, 1))
	must(t, l.AddSheet(1, part, now))
	must(t, l.AddSheet(2, part, now))
	must(t, l.MoveSheet(2, part, 0))
	must(t, l.TrashGroup(notes, now))
	restored := Restore(l.Records())
	if got := groupIDs(restored.Groups(0)); !equal(got, []GroupID{Inbox, novel}) {
		t.Fatalf("top %v", got)
	}
	if got := sheetIDs(restored.Sheets(part)); !equal(got, []SheetID{2, 1}) {
		t.Fatalf("sheets %v", got)
	}
	if !restored.GroupInTrash(notes) {
		t.Fatal("trash lost")
	}
	next, _ := restored.AddGroup(0, "next")
	if next <= notes {
		t.Fatalf("group id %d reused", next)
	}
}

func TestRestoreReadsBrokenRecordsForWhatTheyStillSay(t *testing.T) {
	records := Records{
		Groups: []GroupRecord{
			{Group: Group{ID: 5, Parent: 6, Name: "loop a"}},
			{Group: Group{ID: 6, Parent: 5, Name: "loop b"}},
			{Group: Group{ID: 7, Parent: 99, Name: "orphan"}},
		},
		Sheets: []SheetRecord{{Sheet: Sheet{ID: 1, Group: 42}}},
	}
	l := Restore(records)
	top := groupIDs(l.Groups(0))
	if top[0] != Inbox || len(top) < 3 {
		t.Fatalf("top %v", top)
	}
	if group, _ := l.Group(7); group.Parent != 0 {
		t.Fatal("an orphaned group stays orphaned")
	}
	if sheet, _ := l.Sheet(1); sheet.Group != Inbox {
		t.Fatal("a sheet of a missing group is not in the Inbox")
	}
}

func TestAGroupGoesWithAllItHolds(t *testing.T) {
	l := New()
	novel, _ := l.AddGroup(0, "Novel")
	part, _ := l.AddGroup(novel, "Part one")
	must(t, l.AddSheet(1, novel, now))
	must(t, l.AddSheet(2, part, now))
	must(t, l.AddSheet(3, part, now))
	must(t, l.TrashSheet(3, now))
	if groups, sheets := l.Holds(novel); groups != 1 || sheets != 3 {
		t.Fatalf("holds %d groups, %d sheets", groups, sheets)
	}
	must(t, l.TrashGroup(novel, now.Add(time.Second)))
	must(t, l.RestoreGroup(novel))
	if l.SheetInTrash(2) || !l.SheetInTrash(3) {
		t.Fatal("a group came back without what it held, or with what was trashed on its own")
	}
	must(t, l.TrashGroup(novel, now.Add(2*time.Second)))
	deleted, err := l.DeleteGroup(novel)
	must(t, err)
	slices.Sort(deleted)
	if !equal(deleted, []SheetID{1, 2, 3}) {
		t.Fatalf("deleted %v", deleted)
	}
	if _, _, ok := sheetOrGroupLeft(l, part, 3); ok {
		t.Fatal("something the group held is left")
	}
}

func sheetOrGroupLeft(l *Library, group GroupID, sheet SheetID) (Group, Sheet, bool) {
	g, gok := l.Group(group)
	s, sok := l.Sheet(sheet)
	return g, s, gok || sok
}
