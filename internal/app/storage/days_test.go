package storage

import (
	"maps"
	"testing"
	"time"
)

// The words written each day are kept as last handed over, a day at a
// time, and come back with the notebook.
func TestTheWordsWrittenEachDayAreKept(t *testing.T) {
	root := t.TempDir()
	store := openStore(t, root)
	saver := store.Autosave()
	saver.Written("2026-11-19", 120)
	saver.Written("2026-11-20", 40)
	if err := saver.Flush(); err != nil {
		t.Fatal(err)
	}
	saver.Written("2026-11-20", 75)
	if err := saver.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store = openStore(t, root)
	defer store.Close()
	notebook, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]int{"2026-11-19": 120, "2026-11-20": 75}; !maps.Equal(notebook.Written, want) {
		t.Fatalf("The days should come back as last kept: %v.", notebook.Written)
	}
}

// A day is named by its date where it falls.
func TestADayIsItsLocalDate(t *testing.T) {
	east := time.FixedZone("east", 9*60*60)
	at := time.Date(2026, 11, 20, 23, 30, 0, 0, time.UTC)
	if got := DayOf(at); got != "2026-11-20" {
		t.Fatalf("got %q", got)
	}
	if got := DayOf(at.In(east)); got != "2026-11-21" {
		t.Fatalf("The day should be where it falls: %q.", got)
	}
}
