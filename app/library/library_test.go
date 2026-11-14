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

func TestAddingExistingSheetLeavesLibraryUnchanged(t *testing.T) {
	l := New()
	other, err := l.AddGroup(0, "Other")
	must(t, err)
	must(t, l.AddSheet(7, Inbox, now))
	before := l.Records()
	if err := l.AddSheetAt(7, other, now.Add(time.Hour), 0); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate error = %v", err)
	}
	after := l.Records()
	if !slices.Equal(before.Groups, after.Groups) || !slices.Equal(before.Sheets, after.Sheets) ||
		!slices.Equal(before.Chapters, after.Chapters) {
		t.Fatalf("duplicate changed records: before %+v, after %+v", before, after)
	}
	if got := sheetIDs(Restore(after).Sheets(Inbox)); !slices.Equal(got, []SheetID{7}) {
		t.Fatalf("restored placements = %v", got)
	}
}

func TestAddAtPlacesEntriesOnceAndValidatesBeforeMutation(t *testing.T) {
	l := New()
	first, err := l.AddGroup(0, "First")
	must(t, err)
	second, err := l.AddGroupAt(0, "Second", 1)
	must(t, err)
	if got := groupIDs(l.Groups(0)); !slices.Equal(got, []GroupID{Inbox, second, first}) {
		t.Fatalf("group order = %v", got)
	}
	must(t, l.AddSheet(1, Inbox, now))
	must(t, l.AddSheet(2, Inbox, now))
	chapter, err := l.AddChapterAt(Inbox, "Middle", 1)
	must(t, err)
	must(t, l.AddSheetAt(3, Inbox, now, 2))
	if got := l.Entries(Inbox); !slices.Equal(got, []Entry{
		{Sheet: 1}, {Chapter: chapter}, {Sheet: 3}, {Sheet: 2},
	}) {
		t.Fatalf("entry order = %v", got)
	}
	before := l.Records()
	for _, id := range []SheetID{0, -1} {
		if err := l.AddSheetAt(id, Inbox, now, 0); !errors.Is(err, ErrInvalidID) {
			t.Errorf("invalid identity %d: %v", id, err)
		}
	}
	if err := l.AddSheetAt(4, GroupID(999), now, 0); !errors.Is(err, ErrUnknown) {
		t.Fatalf("invalid group error = %v", err)
	}
	if after := l.Records(); !slices.Equal(before.Sheets, after.Sheets) ||
		!slices.Equal(before.Groups, after.Groups) || !slices.Equal(before.Chapters, after.Chapters) {
		t.Fatalf("failed insertion changed records: before %+v, after %+v", before, after)
	}
}

func TestChaptersDivideTheirFollowingSheets(t *testing.T) {
	l := New()
	must(t, l.AddSheet(1, Inbox, now))
	must(t, l.AddSheet(2, Inbox, now))
	opening, err := l.AddChapter(Inbox, "Opening")
	must(t, err)
	must(t, l.AddSheet(3, Inbox, now))
	closing, err := l.AddChapter(Inbox, "Closing")
	must(t, err)
	must(t, l.AddSheet(4, Inbox, now))

	if chapter, ok := l.SheetChapter(1); ok {
		t.Fatalf("sheet before every chapter belongs to %d", chapter)
	}
	if chapter, ok := l.SheetChapter(3); !ok || chapter != opening {
		t.Fatalf("sheet 3 belongs to %d, want %d", chapter, opening)
	}
	if chapter, ok := l.SheetChapter(4); !ok || chapter != closing {
		t.Fatalf("sheet 4 belongs to %d, want %d", chapter, closing)
	}
	entries := l.Entries(Inbox)
	if len(entries) != 6 || !entries[2].IsChapter() || entries[2].Chapter != opening ||
		!entries[4].IsChapter() || entries[4].Chapter != closing {
		t.Fatalf("entries %+v", entries)
	}

	must(t, l.MoveSheet(2, Inbox, 3))
	if chapter, ok := l.SheetChapter(2); !ok || chapter != opening {
		t.Fatalf("moved sheet belongs to %d, want %d", chapter, opening)
	}
	must(t, l.MoveSheet(2, Inbox, 0))
	if chapter, ok := l.SheetChapter(2); ok {
		t.Fatalf("sheet before every chapter belongs to %d", chapter)
	}
	must(t, l.MoveChapter(closing, Inbox, 1))
	if chapter, ok := l.SheetChapter(3); !ok || chapter != opening {
		t.Fatalf("chapter after the moved divider belongs to %d, want %d", chapter, closing)
	}
	if chapter, ok := l.SheetChapter(4); !ok || chapter != opening {
		t.Fatalf("sheet before the moved divider belongs to %d, want %d", chapter, closing)
	}
	must(t, l.RenameChapter(opening, "  "))
	if chapter, _ := l.Chapter(opening); chapter.Name != "Untitled Chapter" {
		t.Fatalf("blank chapter name %q", chapter.Name)
	}
	must(t, l.DeleteChapter(opening))
	if _, ok := l.Chapter(opening); ok {
		t.Fatal("deleted chapter remains")
	}
	if chapter, ok := l.SheetChapter(3); !ok || chapter != closing {
		t.Fatalf("deleting a divider did not merge its sheets into %d", chapter)
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

func TestTrashOrdersEqualTimesByID(t *testing.T) {
	l := New()
	for _, name := range []string{"One", "Two", "Three"} {
		_, err := l.AddGroup(0, name)
		must(t, err)
	}
	for id := SheetID(2); id <= 5; id++ {
		must(t, l.AddSheet(id, Inbox, now))
		must(t, l.TrashSheet(id, now))
	}
	for _, id := range []GroupID{2, 3, 4} {
		must(t, l.TrashGroup(id, now))
	}

	for range 100 {
		groups, sheets := l.Trash()
		if got := groupIDs(groups); !equal(got, []GroupID{4, 3, 2}) {
			t.Fatalf("equal-time groups = %v, want newest ID first", got)
		}
		if got := sheetIDs(sheets); !equal(got, []SheetID{5, 4, 3, 2}) {
			t.Fatalf("equal-time sheets = %v, want newest ID first", got)
		}
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
	chapter, _ := l.AddChapter(part, "Interlude")
	must(t, l.AddSheet(3, part, now))
	must(t, l.MoveSheet(2, part, 0))
	must(t, l.TrashGroup(notes, now))
	restored := Restore(l.Records())
	if got := groupIDs(restored.Groups(0)); !equal(got, []GroupID{Inbox, novel}) {
		t.Fatalf("top %v", got)
	}
	if got := sheetIDs(restored.Sheets(part)); !equal(got, []SheetID{2, 1, 3}) {
		t.Fatalf("sheets %v", got)
	}
	if got, ok := restored.SheetChapter(3); !ok || got != chapter {
		t.Fatalf("restored chapter membership %d %v", got, ok)
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

func TestHeldIsEverySheetTrashedOrNot(t *testing.T) {
	l := New()
	at := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	g, _ := l.AddGroup(0, "Work")
	for id, group := range map[SheetID]GroupID{3: Inbox, 1: g, 2: g} {
		if err := l.AddSheet(id, group, at); err != nil {
			t.Fatal(err)
		}
	}
	_ = l.TrashSheet(3, at)
	_ = l.TrashGroup(g, at)
	held := l.Held()
	if len(held) != 3 || held[0].ID != 1 || held[2].ID != 3 || len(l.Live()) != 0 {
		t.Fatalf("held %+v, live %+v", held, l.Live())
	}
}

func TestATouchMovesTheRevisionButNotTheFiling(t *testing.T) {
	l := New()
	at := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	_ = l.AddSheet(1, Inbox, at)
	rev, filing := l.Revision(), l.Filing()
	l.Touch(1, at.Add(time.Second))
	if l.Revision() == rev || l.Filing() != filing {
		t.Fatalf("touch: revision %d→%d, filing %d→%d", rev, l.Revision(), filing, l.Filing())
	}
	_ = l.TrashSheet(1, at)
	if l.Filing() == filing {
		t.Fatal("trashing moves the filing")
	}
}

func TestASmartFolderHoldsNothingFiled(t *testing.T) {
	l := New()
	novel, _ := l.AddGroup(0, "Novel")
	smart, err := l.AddSmartGroupAt(novel, "This week", "  $updated:this-week ", 0)
	if err != nil {
		t.Fatal(err)
	}
	if g, _ := l.Group(smart); !g.Smart() || g.Query != "$updated:this-week" || g.Parent != novel {
		t.Fatalf("made %+v", g)
	}
	if err := l.AddSheet(5, smart, now); !errors.Is(err, ErrSmart) {
		t.Fatalf("filing a new sheet: %v", err)
	}
	_ = l.AddSheet(6, novel, now)
	if err := l.MoveSheet(6, smart, 0); !errors.Is(err, ErrSmart) {
		t.Fatalf("moving a sheet in: %v", err)
	}
	if _, err := l.AddChapter(smart, "One"); !errors.Is(err, ErrSmart) {
		t.Fatalf("a chapter in: %v", err)
	}
	if _, err := l.AddGroup(smart, "Inside"); !errors.Is(err, ErrSmart) {
		t.Fatalf("a group in: %v", err)
	}
	if err := l.MoveGroup(novel, smart, 0); !errors.Is(err, ErrSmart) && !errors.Is(err, ErrCycle) {
		t.Fatalf("a group moved in: %v", err)
	}
	if err := l.MoveGroup(smart, 0, 99); err != nil {
		t.Fatalf("a smart folder moves as a group does: %v", err)
	}
	if err := l.SetGroupQuery(smart, "#work"); err != nil {
		t.Fatal(err)
	}
	if err := l.SetGroupQuery(smart, " "); !errors.Is(err, ErrQuery) {
		t.Fatalf("a blank query: %v", err)
	}
	if err := l.SetGroupQuery(novel, "#work"); !errors.Is(err, ErrQuery) {
		t.Fatalf("a group made smart: %v", err)
	}
	if _, err := l.AddSmartGroupAt(0, "Blank", "", 0); !errors.Is(err, ErrQuery) {
		t.Fatalf("a blank smart folder: %v", err)
	}
	if g, _ := l.Group(smart); g.Query != "#work" || l.Count(smart) != 0 {
		t.Fatalf("after: %+v, count %d", g, l.Count(smart))
	}
}

func TestRecordsKeepASmartFolderEmpty(t *testing.T) {
	l := New()
	smart, _ := l.AddSmartGroupAt(0, "Work", "#work", 99)
	records := l.Records()
	if r := Restore(records); r.groups[smart].Query != "#work" {
		t.Fatalf("restored %+v", r.groups[smart])
	}
	// Records that say otherwise are read as they still can be: what is
	// filed in a smart folder goes to the Inbox, a group to the top.
	records.Sheets = append(records.Sheets, SheetRecord{Sheet: Sheet{ID: 9, Group: smart}})
	records.Chapters = append(records.Chapters, ChapterRecord{Chapter: Chapter{ID: 4, Group: smart, Name: "C"}})
	records.Groups = append(records.Groups, GroupRecord{Group: Group{ID: 20, Parent: smart, Name: "In"}},
		GroupRecord{Group: Group{ID: Inbox, Name: "x", Query: "#inbox"}})
	r := Restore(records)
	if s, _ := r.Sheet(9); s.Group != Inbox {
		t.Fatalf("a sheet in a smart folder restored to %d", s.Group)
	}
	if c, _ := r.Chapter(4); c.Group != Inbox {
		t.Fatalf("a chapter restored to %d", c.Group)
	}
	if g, _ := r.Group(20); g.Parent != 0 {
		t.Fatalf("a group restored into %d", g.Parent)
	}
	if g, _ := r.Group(Inbox); g.Smart() {
		t.Fatal("the Inbox restored smart")
	}
}
