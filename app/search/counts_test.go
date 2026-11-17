package search

import (
	"slices"
	"testing"

	"github.com/caelyreth/neverland/internal/query"
)

func TestCountsAreWhatEachValueWouldFind(t *testing.T) {
	x := notebook(t)
	groups := env().Groups
	e := env()
	e.Groups = func(name string) []int64 {
		if name == "twice" {
			return []int64{work, side}
		}
		return groups(name)
	}
	for _, tc := range []struct {
		text, key string
		values    []string
	}{
		{"", "tag", []string{"work", "Work", "db", "travel", "none", "absent"}},
		{"budget", "tag", []string{"work", "travel"}},
		{"", "has", []string{"link", "image", "table"}},
		{"", "in", []string{"Work", "side projects", "twice", "nowhere"}},
		{"", "updated", []string{"today", "7d", "this-month", "2026-08..2026-09", ">2026-01-01"}},
		{"-#travel", "created", []string{"this-year", "last-year"}},
		{"", "words", []string{"100", "<100", "100..900", ">=900"}},
	} {
		counts, err := x.Counts(query.Parse(tc.text), tc.key, tc.values, e)
		if err != nil {
			t.Fatalf("%q %s: %v", tc.text, tc.key, err)
		}
		k, _ := query.KeyNamed(tc.key)
		for _, v := range tc.values {
			with := query.And(tc.text, query.Condition{Key: k, Value: v}.String())
			q := query.Parse(with)
			c := compiler{env: e, issued: q.Issued()}
			res, err := x.run(q, Request{Env: e, Limit: 50}, c.compile(q))
			if err != nil {
				t.Fatal(err)
			}
			if got, ok := counts[v]; !ok || got != res.Total {
				t.Errorf("%q %s:%s counts %d (%v), finds %d", tc.text, tc.key, v, got, ok, res.Total)
			}
		}
	}
}

func TestSettingsAndSheetsHaveNoCounts(t *testing.T) {
	x := notebook(t)
	if c, _ := x.Counts(query.Parse(""), "sort", []string{"title"}, env()); c != nil {
		t.Fatalf("a setting counted %v", c)
	}
	if c, _ := x.Counts(query.Parse("$sheet"), "tag", []string{"work"}, env()); c != nil {
		t.Fatalf("a sheet's find counted %v", c)
	}
}

// The sites offered are those the pages link to, each with the sites it
// is under, most linked first.
func TestSitesAreThoseThePagesLinkTo(t *testing.T) {
	x := notebook(t)
	sites, err := x.Sites(10, env())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(sites, "docs.github.com") || !slices.Contains(sites, "github.com") {
		t.Fatalf("sites = %v", sites)
	}
}
