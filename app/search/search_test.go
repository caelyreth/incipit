package search

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/blevesearch/bleve/v2/search/query"
)

var now = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

const (
	inbox int64 = 1
	work  int64 = 2
	side  int64 = 3 // inside work
)

func notebook(t testing.TB) *Index {
	t.Helper()
	x, err := Memory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { x.Close() })
	err = x.Put(
		Page{Key: 10, Rev: 1, Title: "Database notes", Body: "我们正在设计关系数据库的索引。\nBudget planning for the café", Headings: "Database notes",
			Tags: []string{"Work", "db"}, Groups: []int64{work}, Kinds: []string{"task", "link"}, Domains: []string{"docs.github.com"},
			Tasks:   []Task{{ID: "t1", Text: "Budget planning for the café"}, {ID: "t2", Text: "Q4 review", Done: true}},
			Created: now.AddDate(0, -2, 0), Updated: now.AddDate(0, 0, -1), Words: 900},
		Page{Key: 20, Rev: 3, Title: "Trip", Body: "budget review of the trip\n数据很重要", Tags: []string{"travel"},
			Groups: []int64{side, work}, Kinds: []string{"image"}, Created: now.AddDate(-1, 0, 0), Updated: now.AddDate(0, -3, 0), Words: 120},
		Page{Key: 30, Rev: 1, Title: "Recipes", Body: "ラーメンを食べた\nreceive the parcel", Groups: []int64{inbox},
			Created: now.AddDate(0, 0, -2), Updated: now.AddDate(0, 0, -2), Words: 40},
		Page{Key: 40, Rev: 2, Title: "Old budget", Body: "budget for last year", Groups: []int64{inbox}, Trashed: true,
			Created: now.AddDate(-2, 0, 0), Updated: now.AddDate(-1, 0, 0), Words: 4},
	)
	if err != nil {
		t.Fatal(err)
	}
	return x
}

func env() Env {
	return Env{Now: now, Inbox: inbox, Open: []int64{30}, Groups: func(name string) []int64 {
		return map[string][]int64{"work": {work}, "side projects": {side}}[name]
	}}
}

func find(t *testing.T, x *Index, text string) Result {
	t.Helper()
	res, err := x.Search(Parse(text), Request{Env: env()})
	if err != nil {
		t.Fatalf("%q: %v", text, err)
	}
	return res
}

func keys(res Result) []int64 {
	var out []int64
	for _, h := range res.Hits {
		out = append(out, h.Key)
	}
	slices.Sort(out)
	return out
}

func TestQueriesFindTheirPages(t *testing.T) {
	x := notebook(t)
	for text, want := range map[string][]int64{
		"budget":                      {10, 20},
		"BUDGET Planning":             {10},
		"budget -planning":            {20},
		"budget | ramen":              {10, 20},
		"cafe":                        {10},
		`"budget review"`:             {20},
		"数据库":                         {10},
		"数据":                          {10, 20},
		"系数":                          {10},
		"库":                           {10},
		"ラーメン":                        {30},
		"らーめん":                        {30},
		"#work":                       {10},
		"$tag:WORK $tag:db":           {10},
		"$tag:none":                   {30},
		"$in:work":                    {10, 20},
		`$in:"side projects"`:         {20},
		"$inbox":                      {30},
		"$trash":                      {40},
		"$trash budget":               {40},
		"$open":                       {30},
		"$updated:7d":                 {10, 30},
		"$updated:>7d":                {20},
		"$created:2025":               {20},
		"$words:500":                  {10},
		"$words:<100":                 {30},
		"$title:trip":                 {20},
		"$heading:database":           {10},
		"$has:image":                  {20},
		"$link:github.com":            {10},
		"$link:docs.github.com":       {10},
		"$task":                       {10},
		"$todo:budget":                {10},
		"$done:review":                {10},
		"$todo:review":                nil,
		"budget $in:work $updated:7d": {10},
		"":                            {10, 20, 30},
		"$nope:x budget":              {10, 20},
		"$tag: budget":                {10, 20},
	} {
		if got := keys(find(t, x, text)); !slices.Equal(got, want) {
			t.Errorf("%q found %v, want %v", text, got, want)
		}
	}
}

func TestTheWordBeingTypedFindsTheWordsItStarts(t *testing.T) {
	x := notebook(t)
	if got := keys(find(t, x, "budg")); !slices.Equal(got, []int64{10, 20}) {
		t.Fatalf("budg found %v", got)
	}
	if got := keys(find(t, x, "budg ")); got != nil {
		t.Fatalf("a finished word finds itself only: %v", got)
	}
	if got := keys(find(t, x, "=budg")); got != nil {
		t.Fatalf("=budg found %v", got)
	}
}

func TestWordsATypoAwayComeOnlyWhenTheWordsFindTooFew(t *testing.T) {
	x := notebook(t)
	res := find(t, x, "recieve ")
	if !res.Close || !slices.Equal(keys(res), []int64{30}) {
		t.Fatalf("recieve: %v close=%v", keys(res), res.Close)
	}
	if res := find(t, x, "budget "); res.Close {
		t.Fatal("words that find enough take no typos")
	}
	if res := find(t, x, "~budgte "); !slices.Equal(keys(res), []int64{10, 20}) || res.Close {
		t.Fatalf("~budgte: %v close=%v", keys(res), res.Close)
	}
}

func TestATaskFilterHoldsOfOneTaskAndNamesIt(t *testing.T) {
	x := notebook(t)
	res := find(t, x, "$todo:budget")
	if len(res.Hits) != 1 || !slices.Equal(res.Hits[0].Tasks, []string{"t1"}) {
		t.Fatalf("hits %+v", res.Hits)
	}
	// "review" is in a done task, "planning" in an open one: no one task
	// holds both.
	if got := keys(find(t, x, `$task:"planning review"`)); got != nil {
		t.Fatalf("found %v", got)
	}
}

func TestSortAndFacets(t *testing.T) {
	x := notebook(t)
	order := func(text string) []int64 {
		res := find(t, x, text)
		var out []int64
		for _, h := range res.Hits {
			out = append(out, h.Key)
		}
		return out
	}
	if got := order("$sort:updated"); !slices.Equal(got, []int64{10, 30, 20}) {
		t.Errorf("updated %v", got)
	}
	if got := order("$sort:-updated"); !slices.Equal(got, []int64{20, 30, 10}) {
		t.Errorf("-updated %v", got)
	}
	if got := order("$sort:title"); !slices.Equal(got, []int64{10, 30, 20}) {
		t.Errorf("title %v", got)
	}
	if got := order("$sort:words"); !slices.Equal(got, []int64{10, 20, 30}) {
		t.Errorf("words %v", got)
	}
	res, err := x.Search(Parse("budget"), Request{Env: env(), Facets: []string{"tags", "groups"}})
	if err != nil {
		t.Fatal(err)
	}
	count := func(name, v string) int {
		for _, c := range res.Facets[name] {
			if c.Value == v {
				return c.Count
			}
		}
		return 0
	}
	if count("tags", "work") != 1 || count("tags", "travel") != 1 || count("groups", fmt.Sprint(work)) != 2 {
		t.Fatalf("facets %+v", res.Facets)
	}
}

func TestAPageIsPutInPlaceOfItselfAndLetGo(t *testing.T) {
	x := notebook(t)
	if err := x.Put(Page{Key: 20, Rev: 4, Title: "Trip", Body: "nothing about money"}); err != nil {
		t.Fatal(err)
	}
	if got := keys(find(t, x, "budget")); !slices.Equal(got, []int64{10}) {
		t.Fatalf("budget %v after the trip changed", got)
	}
	if err := x.Delete(10); err != nil {
		t.Fatal(err)
	}
	if got := keys(find(t, x, "budget")); got != nil {
		t.Fatalf("budget %v after the notes went", got)
	}
	if got := keys(find(t, x, "$todo")); got != nil {
		t.Fatalf("a page's tasks go with it: %v", got)
	}
	revs, err := x.Revisions()
	if err != nil {
		t.Fatal(err)
	}
	if len(revs) != 3 || revs[20] != 4 || revs[30] != 1 {
		t.Fatalf("revisions %v", revs)
	}
}

func TestAnIndexKeptOnDiskOpensAgainAndRemakesWhatItCannotRead(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "index")
	x, fresh, err := Open(dir)
	if err != nil || !fresh {
		t.Fatalf("open %v fresh=%v", err, fresh)
	}
	x.Put(Page{Key: 1, Rev: 7, Title: "kept"})
	x.Close()
	x, fresh, err = Open(dir)
	if err != nil || fresh {
		t.Fatalf("reopen %v fresh=%v", err, fresh)
	}
	if revs, _ := x.Revisions(); revs[1] != 7 {
		t.Fatalf("revisions %v", revs)
	}
	x.b.SetInternal([]byte(versionKey), []byte("0"))
	x.Close()
	x, fresh, err = Open(dir)
	if err != nil || !fresh {
		t.Fatalf("another layout: %v fresh=%v", err, fresh)
	}
	x.Close()
	os.WriteFile(filepath.Join(dir, "index_meta.json"), []byte("{broken"), 0o644)
	x, fresh, err = Open(dir)
	if err != nil || !fresh {
		t.Fatalf("a broken index: %v fresh=%v", err, fresh)
	}
	x.Close()
}

func TestASheetQueryIsNoQuestionForTheIndex(t *testing.T) {
	x := notebook(t)
	if res := find(t, x, "$sheet budget"); res.Total != 0 {
		t.Fatalf("found %v", keys(res))
	}
}

// Every key compiles to nodes Bleve's nested collection looks through, so
// no hit is ever part of a page (references/search-engine.md, pitfalls).
func TestNoHitIsEverPartOfAPage(t *testing.T) {
	x := notebook(t)
	values := map[Takes]string{TakesText: "budget", TakesOptionalText: "budget", TakesName: "work", TakesDate: "7d", TakesNumber: "10", TakesNothing: ""}
	for _, k := range Keys {
		v := values[k.Takes]
		if k.Takes == TakesChoice {
			v = k.Choices[0]
		}
		text := "$" + k.Name
		if v != "" {
			text += ":" + v
		}
		for _, q := range []string{text, text + " budget", "-" + text + " budget", "(" + text + " | budget)"} {
			parsed := Parse(q)
			c := compiler{env: env(), issued: issued(parsed)}
			compiled := c.compile(parsed)
			m := mustMapping(t)
			fields, err := query.ExtractFields(compiled, m, nil)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(q, "$task") || strings.Contains(q, "$todo") || strings.Contains(q, "$done") {
				if !m.IntersectsPrefix(fields) {
					t.Errorf("%q: the task fields are hidden from the collector (%v)", q, fields)
				}
			}
			res, err := x.Search(parsed, Request{Env: env()})
			if err != nil {
				t.Fatalf("%q: %v", q, err)
			}
			for _, h := range res.Hits {
				if h.Key == 0 {
					t.Errorf("%q: hit %+v", q, h)
				}
			}
		}
	}
}

func mustMapping(t *testing.T) *mappingImpl {
	m, err := pageMapping()
	if err != nil {
		t.Fatal(err)
	}
	return m
}
