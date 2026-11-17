package search

import (
	"regexp"
	"strings"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/internal/query"
)

// countFacet names the facet a count runs.
const countFacet = "count"

// Counts are how many pages a query finds with each of values for a key
// as one condition more: what a filter builder shows beside the values it
// offers. They are counted from the query's words as written, as its
// first pass reads them. Values of one key are counted together, by the
// field the key reads, in one search: a tag, a kind, a group, a span of
// dates or of lengths. A value no field counts alone, as a name that
// names several groups, is counted by a search of its own. A value the
// key does not take, and a setting's, have no count.
func (x *Index) Counts(q query.Query, key string, values []string, env Env) (map[string]int, error) {
	k, ok := query.KeyNamed(key)
	if !ok || k.Setting || q.Scope != query.InNotebook || len(values) == 0 {
		return nil, nil
	}
	if env.Now.IsZero() {
		env.Now = time.Now()
	}
	var (
		field string
		terms = map[string][]string{}
		facet = &bleve.FacetRequest{}
		// ranged are the values the facet counts as spans, each once.
		ranged = map[string]bool{}
		alone  []string
	)
	for _, v := range values {
		if !k.Accepts(v) {
			continue
		}
		switch k.Name {
		case "tag":
			if query.Fold(v) == "none" {
				alone = append(alone, v)
				continue
			}
			field = "tags"
			terms[profile.TagKey(v)] = append(terms[profile.TagKey(v)], v)
		case "has":
			field = "kinds"
			terms[query.Fold(v)] = append(terms[query.Fold(v)], v)
		case "in":
			ids := groupsNamed(env, v)
			if len(ids) != 1 {
				alone = append(alone, v)
				continue
			}
			field = "groups"
			terms[groupTerm(ids[0])] = append(terms[groupTerm(ids[0])], v)
		case "updated", "date", "created":
			d, _ := query.ParseDates(v, env.Now)
			field = dateField(k.Name)
			if !ranged[v] {
				ranged[v] = true
				facet.AddDateTimeRange(v, d.From, d.To)
			}
		case "words":
			n, _ := query.ParseCounts(v)
			field = "words"
			if !ranged[v] {
				ranged[v] = true
				var lo, hi *float64
				if n.HasMin {
					x := float64(n.Min)
					lo = &x
				}
				if n.HasMax {
					// A facet's span leaves its end out; a count's holds it.
					x := float64(n.Max + 1)
					hi = &x
				}
				facet.AddNumericRange(v, lo, hi)
			}
		default:
			alone = append(alone, v)
		}
	}
	out := map[string]int{}
	c := compiler{env: env, issued: q.Issued()}
	if field != "" {
		facet.Field, facet.Size = field, max(len(terms), len(ranged))
		if len(terms) > 0 {
			var pattern []string
			for t := range terms {
				pattern = append(pattern, regexp.QuoteMeta(t))
			}
			facet.SetRegexFilter("^(?:" + strings.Join(pattern, "|") + ")$")
		}
		// A facet leaves out what no page holds: those values find none.
		for _, vs := range terms {
			for _, v := range vs {
				out[v] = 0
			}
		}
		for v := range ranged {
			out[v] = 0
		}
		req := bleve.NewSearchRequestOptions(c.compile(q), 0, 0, false)
		req.AddFacet(countFacet, facet)
		res, err := x.b.Search(req)
		if err != nil {
			return nil, err
		}
		if f := res.Facets[countFacet]; f != nil {
			for _, t := range f.Terms.Terms() {
				for _, v := range terms[t.Term] {
					out[v] = t.Count
				}
			}
			for _, r := range f.DateRanges {
				out[r.Name] = r.Count
			}
			for _, r := range f.NumericRanges {
				out[r.Name] = r.Count
			}
		}
	}
	for _, v := range alone {
		with := query.Parse(query.And(q.Text, query.Condition{Key: k, Value: v}.String()))
		ac := compiler{env: env, issued: with.Issued()}
		res, err := x.b.Search(bleve.NewSearchRequestOptions(ac.compile(with), 0, 0, false))
		if err != nil {
			return nil, err
		}
		out[v] = int(res.Total)
	}
	return out, nil
}

// groupsNamed are the groups a name names.
func groupsNamed(env Env, name string) []int64 {
	if env.Groups == nil {
		return nil
	}
	return env.Groups(query.Fold(name))
}

// dateField is the field a date key reads.
func dateField(key string) string {
	if key == "created" {
		return "created"
	}
	return "updated"
}

// Sites are the sites the notebook's pages link to, most linked first, at
// most limit of them: what $link offers. A site is offered with the sites
// it is under, as docs.github.com with github.com.
func (x *Index) Sites(limit int, env Env) ([]string, error) {
	if env.Now.IsZero() {
		env.Now = time.Now()
	}
	q := query.Parse("")
	c := compiler{env: env, issued: q.Issued()}
	req := bleve.NewSearchRequestOptions(c.compile(q), 0, 0, false)
	req.AddFacet(countFacet, bleve.NewFacetRequest("domains", limit))
	res, err := x.b.Search(req)
	if err != nil {
		return nil, err
	}
	var out []string
	if f := res.Facets[countFacet]; f != nil {
		for _, t := range f.Terms.Terms() {
			out = append(out, t.Term)
		}
	}
	return out, nil
}
