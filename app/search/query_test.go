package search

import (
	"testing"
	"time"
)

func kinds(q Query) map[string]IssueKind {
	out := map[string]IssueKind{}
	for _, is := range q.Issues {
		out[q.Text[is.From:is.To]] = is.Kind
	}
	return out
}

func TestAQueryPrintsBackAsWritten(t *testing.T) {
	for _, text := range []string{
		"budget review",
		`"weekly notes" -draft`,
		"a | b c",
		"(a | b) -(c d)",
		`$tag:work #ideas $in:"Side projects" $updated:7d`,
		"~recieve =go $inbox $has:image $sort:-updated",
		`"say \"hi\""`,
	} {
		if got := Parse(text).String(); got != text {
			t.Errorf("%q printed %q", text, got)
		}
	}
}

func TestPartsJoinByAllBeforeAny(t *testing.T) {
	q := Parse("a b | c")
	or, ok := q.Root.(*Any)
	if !ok || len(or.Parts) != 2 {
		t.Fatalf("root %#v", q.Root)
	}
	if and, ok := or.Parts[0].(*All); !ok || len(and.Parts) != 2 {
		t.Fatalf("first part %#v", or.Parts[0])
	}
}

func TestTheWordBeingTypedIsOpen(t *testing.T) {
	q := Parse("budget rev")
	parts := q.Root.(*All).Parts
	if parts[0].(*Term).Open || !parts[1].(*Term).Open {
		t.Fatal("only the last word, still being typed, is open")
	}
	if Parse("budget rev ").Root.(*All).Parts[1].(*Term).Open {
		t.Fatal("a word followed by a space is finished")
	}
}

func TestUnfinishedAndWrongPartsAreIssuesNotErrors(t *testing.T) {
	q := Parse(`$tag: $nope:x $updated:someday $inbox:x "open phrase`)
	got := kinds(q)
	want := map[string]IssueKind{"$tag:": Pending, "$nope:x": Unknown, "someday": Invalid, "x": Invalid, `"open phrase`: Pending}
	for text, kind := range want {
		if got[text] != kind {
			t.Errorf("%q: issue %d, want %d (all %v)", text, got[text], kind, got)
		}
	}
	if q := Parse("budget $up"); kinds(q)["$up"] != Pending {
		t.Fatalf("a key being written is pending: %v", kinds(q))
	}
	if q := Parse("$up budget"); kinds(q)["$up"] != Unknown {
		t.Fatalf("a key left unfinished is unknown: %v", kinds(q))
	}
	if q := Parse("(a b"); q.Root.(*Group).Closed || kinds(q)["(a b"] != Pending {
		t.Fatalf("an unclosed group is pending: %v", kinds(q))
	}
	if q := Parse("a ) b"); kinds(q)[")"] != Invalid || len(q.Root.(*All).Parts) != 2 {
		t.Fatalf("a stray parenthesis counts for nothing: %v", kinds(q))
	}
}

func TestSheetRunsAQueryInTheSheetAndDatesGoInert(t *testing.T) {
	q := Parse("$sheet budget $updated:7d $heading:plan")
	if q.Scope != InSheet {
		t.Fatal("$sheet runs the query in the sheet")
	}
	got := kinds(q)
	if got["$updated:7d"] != Inert || got["$heading:plan"] != 0 {
		t.Fatalf("issues %v", got)
	}
	if kinds(Parse("$case budget"))["$case"] != Inert {
		t.Fatal("$case means nothing across the notebook")
	}
}

func TestHashWritesATagAsTextWritesIt(t *testing.T) {
	f, ok := Parse("#work").Root.(*Filter)
	if !ok || f.Key.Name != "tag" || f.Value != "work" || !f.Hash {
		t.Fatalf("filter %#v", Parse("#work").Root)
	}
	if _, ok := Parse("#1").Root.(*Term); !ok {
		t.Fatal("#1 holds no tag")
	}
}

func TestDatesReadAtNow(t *testing.T) {
	now := time.Date(2026, 11, 14, 15, 0, 0, 0, time.UTC) // a Saturday
	day := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }
	for v, want := range map[string]Dates{
		"today":            {day(2026, 11, 14), day(2026, 11, 15)},
		"this-week":        {day(2026, 11, 9), day(2026, 11, 16)},
		"last-month":       {day(2026, 10, 1), day(2026, 11, 1)},
		"2026-11":          {day(2026, 11, 1), day(2026, 12, 1)},
		">2026-11-01":      {From: day(2026, 11, 2)},
		"<=2026":           {To: day(2027, 1, 1)},
		"2026-10..2026-11": {day(2026, 10, 1), day(2026, 12, 1)},
		"3d":               {From: now.AddDate(0, 0, -3)},
		"<3d":              {From: now.AddDate(0, 0, -3)},
		">3d":              {To: now.AddDate(0, 0, -3)},
		"30d..7d":          {now.AddDate(0, 0, -30), now.AddDate(0, 0, -7)},
	} {
		got, ok := ParseDates(v, now)
		if !ok || !got.From.Equal(want.From) || !got.To.Equal(want.To) {
			t.Errorf("%s: %v %v, want %v", v, got, ok, want)
		}
	}
	for _, v := range []string{"", "someday", "3x", "2026-13", ".."} {
		if _, ok := ParseDates(v, now); ok {
			t.Errorf("%q read as a date", v)
		}
	}
}

func TestCountsRead(t *testing.T) {
	for v, want := range map[string]Counts{
		"500":     {Min: 500, HasMin: true},
		">500":    {Min: 501, HasMin: true},
		"<100":    {Max: 99, HasMax: true},
		"100..2k": {Min: 100, Max: 2000, HasMin: true, HasMax: true},
		"..1.5k":  {Max: 1500, HasMax: true},
	} {
		if got, ok := ParseCounts(v); !ok || got != want {
			t.Errorf("%s: %+v %v, want %+v", v, got, ok, want)
		}
	}
}
