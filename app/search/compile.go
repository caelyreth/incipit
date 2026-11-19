package search

import (
	"github.com/caelyreth/neverland/internal/query"

	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/blevesearch/bleve/v2"
	bq "github.com/blevesearch/bleve/v2/search/query"
	"github.com/caelyreth/neverland/editor/profile"
)

// Env is what a query reads from the notebook as it runs.
type Env struct {
	Now time.Time
	// Groups are the groups a name names, folded; nil names none.
	Groups func(name string) []int64
	// Inbox is the Inbox's group.
	Inbox int64
	// Open are the pages open in tabs.
	Open []int64
}

// Request is a search of the index.
type Request struct {
	Env
	// Limit is how many hits to return from Offset; zero returns 50.
	Limit, Offset int
	// Facets are the fields to count the hits by: "tags", "kinds" or
	// "groups".
	Facets []string
}

// Hit is a page a query found.
type Hit struct {
	Key   int64
	Score float64
}

// Count is how many hits hold a facet's value.
type Count struct {
	Value string
	Count int
}

// Result is what a search found.
type Result struct {
	Hits  []Hit
	Total int
	// Close reports hits found by words a typo away, the query's own
	// finding too few.
	Close  bool
	Facets map[string][]Count
}

// fewHits is how few hits the words as written find before words a typo
// away are taken too.
const fewHits = 3

// rerankDepth is how many of the best hits are ranked again by what Bleve
// cannot see.
const rerankDepth = 200

// Search runs a notebook bq. A query run in a sheet ($sheet) is no
// question for the index and finds nothing here.
func (x *Index) Search(q query.Query, r Request) (Result, error) {
	if q.Scope != query.InNotebook {
		return Result{}, nil
	}
	if r.Limit <= 0 {
		r.Limit = 50
	}
	if r.Now.IsZero() {
		r.Now = time.Now()
	}
	c := compiler{env: r.Env, issued: q.Issued()}
	res, err := x.run(q, r, c.compile(q))
	if err != nil || res.Total >= fewHits || !c.loose {
		return res, err
	}
	c.close = true
	closer, err := x.run(q, r, c.compile(q))
	if err != nil || closer.Total <= res.Total {
		return res, err
	}
	closer.Close = true
	return closer, nil
}

func (x *Index) run(q query.Query, r Request, bq bq.Query) (Result, error) {
	order, byField := sortOf(q)
	size, from := r.Limit, r.Offset
	if !byField {
		size, from = max(rerankDepth, r.Offset+r.Limit), 0
	}
	req := bleve.NewSearchRequestOptions(bq, size, from, false)
	req.Fields = []string{"updated"}
	if byField {
		req.SortBy(order)
	}
	for _, f := range r.Facets {
		req.AddFacet(f, bleve.NewFacetRequest(f, 20))
	}
	res, err := x.b.Search(req)
	if err != nil {
		return Result{}, err
	}
	out := Result{Total: int(res.Total)}
	for _, h := range res.Hits {
		k, ok := pageKey(h.ID)
		if !ok {
			continue
		}
		hit := Hit{Key: k, Score: h.Score}
		if !byField {
			hit.Score *= recency(h.Fields["updated"], r.Now)
		}
		out.Hits = append(out.Hits, hit)
	}
	if !byField {
		slices.SortStableFunc(out.Hits, func(a, b Hit) int {
			switch {
			case a.Score > b.Score:
				return -1
			case a.Score < b.Score:
				return 1
			}
			return 0
		})
		out.Hits = out.Hits[min(r.Offset, len(out.Hits)):min(r.Offset+r.Limit, len(out.Hits))]
	}
	if len(res.Facets) > 0 {
		out.Facets = map[string][]Count{}
		for name, f := range res.Facets {
			for _, t := range f.Terms.Terms() {
				v := t.Term
				if name == "groups" {
					v = strings.TrimPrefix(v, "g")
				}
				out.Facets[name] = append(out.Facets[name], Count{Value: v, Count: t.Count})
			}
		}
	}
	return out, nil
}

// recency weighs a hit by how lately its page changed: a page changed now
// counts a fifth more than one changed long ago, half that after a month.
func recency(updated any, now time.Time) float64 {
	s, _ := updated.(string)
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return 1
	}
	age := now.Sub(t).Hours() / 24
	return 1 + 0.2*math.Exp2(-max(age, 0)/30)
}

// sortOf is the order a query's $sort asks for, and whether it is a
// field's rather than relevance.
func sortOf(q query.Query) ([]string, bool) {
	var order []string
	query.Walk(q.Root, func(n query.Node) {
		f, ok := n.(*query.Filter)
		if !ok || f.Key == nil || f.Key.Name != "sort" || !f.Key.Accepts(f.Value) {
			return
		}
		v := query.Fold(f.Value)
		turned := strings.HasPrefix(v, "-")
		v = strings.TrimPrefix(v, "-")
		// Dates run oldest first (so newest sits by the list's pill), lengths
		// longest first, and titles A to Z.
		desc := v == "words"
		if turned {
			desc = !desc
		}
		field := map[string]string{"updated": "updated", "created": "created", "title": "title_sort", "words": "words"}[v]
		switch {
		case v == "relevance":
			order = nil
		case desc:
			order = []string{"-" + field, "-_score"}
		default:
			order = []string{field, "-_score"}
		}
	})
	return order, order != nil
}

// compiler turns a parsed query into the index's. It builds field
// queries, boolean queries, conjunctions and disjunctions only.
type compiler struct {
	env    Env
	issued map[query.Span]bool
	// close takes words a typo away for every word it may; loose is set
	// when a query has such a word.
	close, loose bool
}

func (c *compiler) compile(q query.Query) bq.Query {
	trashed := false
	query.Walk(q.Root, func(n query.Node) {
		if f, ok := n.(*query.Filter); ok && f.Key != nil && f.Key.Name == "trash" && !c.issued[f.Span] {
			trashed = true
		}
	})
	place := bleve.NewBoolFieldQuery(trashed)
	place.SetField("trashed")
	root := c.node(q.Root)
	if root == nil {
		return place
	}
	return bleve.NewConjunctionQuery(root, place)
}

// node is a part's query; nil is a part that counts for nothing.
func (c *compiler) node(n query.Node) bq.Query {
	switch n := n.(type) {
	case nil:
		return nil
	case *query.Term:
		if n.Text == "" {
			return nil
		}
		return c.text(n, textFields)
	case *query.Filter:
		if c.issued[n.Span] || n.Key == nil {
			return nil
		}
		return c.filter(n)
	case *query.Group:
		return c.node(n.Part)
	case *query.Not:
		part := c.node(n.Part)
		if part == nil {
			return nil
		}
		b := bleve.NewBooleanQuery()
		b.AddMust(bleve.NewMatchAllQuery())
		b.AddMustNot(part)
		return b
	case *query.All:
		return c.all(n.Parts)
	case *query.Any:
		var parts []bq.Query
		for _, p := range n.Parts {
			if q := c.node(p); q != nil {
				parts = append(parts, q)
			}
		}
		switch len(parts) {
		case 0:
			return nil
		case 1:
			return parts[0]
		}
		return bleve.NewDisjunctionQuery(parts...)
	}
	return nil
}

// all is parts that must all match: its Not parts are what must not.
func (c *compiler) all(nodes []query.Node) bq.Query {
	var must, not []bq.Query
	for _, p := range nodes {
		if neg, ok := p.(*query.Not); ok {
			if q := c.node(neg.Part); q != nil {
				not = append(not, q)
			}
			continue
		}
		if q := c.node(p); q != nil {
			must = append(must, q)
		}
	}
	if len(not) == 0 {
		switch len(must) {
		case 0:
			return nil
		case 1:
			return must[0]
		}
		return bleve.NewConjunctionQuery(must...)
	}
	b := bleve.NewBooleanQuery()
	if len(must) == 0 {
		b.AddMust(bleve.NewMatchAllQuery())
	} else {
		b.AddMust(must...)
	}
	b.AddMustNot(not...)
	return b
}

// field is a text field a term is looked for in, and how much a match
// there counts.
type field struct {
	name  string
	boost float64
}

var textFields = []field{{"title", 3}, {"headings", 2}, {"body", 1}}

// text is a term looked for in fields.
func (c *compiler) text(t *query.Term, fields []field) bq.Query {
	tokens := query.Tokens(t.Text, query.Queried)
	if len(tokens) == 0 {
		return bleve.NewMatchNoneQuery()
	}
	var parts []bq.Query
	for _, f := range fields {
		if len(tokens) > 1 || t.Phrase {
			parts = append(parts, phrase(tokens, f))
			continue
		}
		term := tokens[0].Term
		exact := bleve.NewTermQuery(term)
		exact.SetField(f.name)
		exact.SetBoost(f.boost)
		parts = append(parts, exact)
		runes := utf8.RuneCountInString(term)
		if t.Open && t.Match != query.Exact && (runes >= 2 || query.Unspaced([]rune(term)[0])) {
			pre := bleve.NewPrefixQuery(term)
			pre.SetField(f.name)
			pre.SetBoost(f.boost * 0.8)
			parts = append(parts, pre)
		}
		if t.Match != query.Exact && runes >= 4 && !query.Unspaced([]rune(term)[0]) {
			if t.Match == query.Auto {
				c.loose = true
			}
			if t.Match == query.Fuzzy || c.close {
				fz := bleve.NewFuzzyQuery(term)
				fz.Fuzziness = 1
				if runes >= 8 {
					fz.Fuzziness = 2
				}
				fz.SetField(f.name)
				fz.SetBoost(f.boost * 0.5)
				parts = append(parts, fz)
			}
		}
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return bleve.NewDisjunctionQuery(parts...)
}

// phrase is tokens at their positions, a position the query leaves open
// taking any term.
func phrase(tokens []query.Token, f field) bq.Query {
	first := tokens[0].Pos
	slots := make([][]string, tokens[len(tokens)-1].Pos-first+1)
	for _, tk := range tokens {
		slots[tk.Pos-first] = append(slots[tk.Pos-first], tk.Term)
	}
	for i := range slots {
		if slots[i] == nil {
			slots[i] = []string{""}
		}
	}
	q := bq.NewMultiPhraseQuery(slots, f.name)
	q.SetBoost(f.boost)
	return q
}

// words are a filter's text as the terms it holds, all of which must
// match in fields.
func (c *compiler) words(v string, fields []field) bq.Query {
	var parts []bq.Query
	for _, w := range strings.Fields(v) {
		parts = append(parts, c.text(&query.Term{Text: w}, fields))
	}
	switch len(parts) {
	case 0:
		return nil
	case 1:
		return parts[0]
	}
	return bleve.NewConjunctionQuery(parts...)
}

func term(field, value string) bq.Query {
	q := bleve.NewTermQuery(value)
	q.SetField(field)
	return q
}

func (c *compiler) filter(f *query.Filter) bq.Query {
	v := f.Value
	switch f.Key.Name {
	case "tag":
		if query.Fold(v) == "none" {
			q := bleve.NewBoolFieldQuery(false)
			q.SetField("tagged")
			return q
		}
		return term("tags", profile.TagKey(v))
	case "in":
		return c.groups(groupsNamed(c.env, v))
	case "inbox":
		return c.groups([]int64{c.env.Inbox})
	case "open":
		ids := make([]string, len(c.env.Open))
		for i, k := range c.env.Open {
			ids[i] = docID(k)
		}
		if len(ids) == 0 {
			return bleve.NewMatchNoneQuery()
		}
		return bleve.NewDocIDQuery(ids)
	case "updated", "date", "created":
		d, _ := query.ParseDates(v, c.env.Now)
		yes, no := true, false
		q := bleve.NewDateRangeInclusiveQuery(d.From, d.To, &yes, &no)
		q.SetField(dateField(f.Key.Name))
		return q
	case "words":
		n, _ := query.ParseCounts(v)
		var lo, hi *float64
		if n.HasMin {
			x := float64(n.Min)
			lo = &x
		}
		if n.HasMax {
			x := float64(n.Max)
			hi = &x
		}
		yes := true
		q := bleve.NewNumericRangeInclusiveQuery(lo, hi, &yes, &yes)
		q.SetField("words")
		return q
	case "title":
		return c.words(v, []field{{"title", 1}})
	case "heading":
		return c.words(v, []field{{"headings", 1}})
	case "code":
		return c.words(v, []field{{"code", 1}})
	case "quote":
		return c.words(v, []field{{"quote", 1}})
	case "has":
		return term("kinds", query.Fold(v))
	case "link":
		return term("domains", strings.TrimPrefix(query.Fold(v), "www."))
	}
	// $trash, $sort, $bands, $sheet and the find's own keys are no condition here.
	return nil
}

func (c *compiler) groups(ids []int64) bq.Query {
	var parts []bq.Query
	for _, id := range ids {
		parts = append(parts, term("groups", groupTerm(id)))
	}
	switch len(parts) {
	case 0:
		return bleve.NewMatchNoneQuery()
	case 1:
		return parts[0]
	}
	return bleve.NewDisjunctionQuery(parts...)
}

// String names a page for tests and logs.
func (h Hit) String() string { return strconv.FormatInt(h.Key, 10) }
