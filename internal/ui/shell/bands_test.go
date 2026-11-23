package shell

import (
	"slices"
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/text/query"
)

// A search's bands cut where the match falls away most, not at counts.
func TestRelevanceCutsWhereTheMatchFallsAway(t *testing.T) {
	var of []facts
	for _, score := range []float64{10, 9.5, 9, 4, 3.8, 1, 0.9} {
		of = append(of, facts{score: score})
	}
	bands, names := lensBands("relevance", of, true, time.Now())
	if !slices.Equal(bands, []int{0, 0, 0, 1, 1, 2, 2}) || !slices.Equal(names, []string{"Strong", "Also", "Mentions"}) {
		t.Fatalf("bands %v %v", bands, names)
	}
	// An even fall is one band.
	of = []facts{{score: 5}, {score: 4.5}, {score: 4}}
	if bands, names := lensBands("relevance", of, true, time.Now()); !slices.Equal(bands, []int{0, 0, 0}) || len(names) != 1 {
		t.Fatalf("bands %v %v", bands, names)
	}
}

// Activity is what was written lately, the bands nothing stands in left
// out; tags stand the most found nearest, the untagged furthest.
func TestBandsAreNamedForWhatStandsInThem(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	of := []facts{{updated: now.Add(-time.Hour)}, {updated: now.AddDate(-1, 0, 0)}}
	bands, names := lensBands("activity", of, false, now)
	if !slices.Equal(bands, []int{0, 1}) || !slices.Equal(names, []string{"Lately", "Dormant"}) {
		t.Fatalf("activity %v %v", bands, names)
	}
	of = []facts{{tag: "b"}, {tag: "a"}, {}, {tag: "a"}}
	bands, names = lensBands("tag", of, false, now)
	if !slices.Equal(bands, []int{1, 0, 2, 0}) || !slices.Equal(names, []string{"a", "b", "No tag"}) {
		t.Fatalf("tag %v %v", bands, names)
	}
}

// Unchosen, the field has no bands and stands as it did; $bands chosen
// stands it in named rings, offered in the field and in the list alike.
func TestBandsAreChosen(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	settle(tester)
	if f := state.results(time.Now()); f.bands != nil {
		t.Fatalf("The field should have no bands unchosen: %v.", f.bands)
	}
	state.SetFieldQuery("$bands:updated")
	settle(tester)
	f := state.results(time.Now())
	if len(f.bands) == 0 {
		t.Fatal("A lens chosen should stand the field in bands.")
	}
	if _, ok := tester.Find(f.bands[0]); !ok {
		t.Fatalf("The nearest band's name %q should show: %v", f.bands[0], tester.Texts())
	}
	last := 0
	for _, cd := range f.cards {
		if cd.band < last {
			t.Fatalf("Bands should stand nearest first: %+v.", f.cards)
		}
		last = cd.band
	}
}

// A sort and bands each choose a view: one taken puts the other away.
func TestASortAndBandsPutEachOtherAway(t *testing.T) {
	conds, _ := put([]string{"$sort:title"}, query.Condition{Key: mustKey("bands"), Value: "tag"})
	if !slices.Equal(conds, []string{"$bands:tag"}) {
		t.Fatalf("Bands taken should put the sort away: %v.", conds)
	}
	conds, _ = put(conds, query.Condition{Key: mustKey("sort"), Value: "updated"})
	if !slices.Equal(conds, []string{"$sort:updated"}) {
		t.Fatalf("A sort taken should put the bands away: %v.", conds)
	}
}
