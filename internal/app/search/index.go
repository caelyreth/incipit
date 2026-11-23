package search

import (
	"github.com/caelyreth/neverland/internal/text/query"

	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis"
	_ "github.com/blevesearch/bleve/v2/analysis/analyzer/custom"
	"github.com/blevesearch/bleve/v2/index/scorch"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/registry"
	"github.com/caelyreth/neverland/internal/editor/profile"
)

// layoutVersion names how pages are read into the index: the analyzer and
// the fields. An index kept by another is rebuilt, never migrated.
const layoutVersion = "2"

const versionKey = "neverland.layout"

// Page is what the index keeps of one page.
type Page struct {
	Key int64
	// Rev is the page's revision the index holds: what the notebook
	// compares to know which pages to read again (Revisions).
	Rev uint64
	// Title is the page's title; Headings, Body, Code and Quote its texts
	// of each kind, one block a line. Body holds every text, the others'
	// included.
	Title                       string
	Headings, Body, Code, Quote string
	// Tags are the tags the page writes, as written.
	Tags []string
	// Groups are the group the page is filed in and every group above it.
	Groups []int64
	// Kinds are the kinds of thing it holds (Kinds); Domains the sites it
	// links to.
	Kinds   []string
	Domains []string
	// Tasks are its task items, each a part of the page of its own in the
	// index, so that what a task filter asks holds of one task.
	Tasks   []Task
	Created time.Time
	Updated time.Time
	Words   int
	Trashed bool
}

// Task is a task of a page: the item it is, its text, and whether it is
// done.
type Task struct {
	ID   string
	Text string
	Done bool
}

// Index is a notebook's search index. It is a cache: the notebook is the
// source of truth, and an index that cannot be read is made again empty.
// It is safe for use from several goroutines.
type Index struct {
	b bleve.Index
}

// ErrClosed reports an index used after Close.
var ErrClosed = errors.New("search: the index is closed")

// Open opens the index kept in dir, or makes it there. Fresh reports an
// index that holds nothing yet: new, kept by another layout, or unreadable
// and made again; every page is to be put in it.
func Open(dir string) (x *Index, fresh bool, err error) {
	if _, statErr := os.Stat(dir); statErr == nil {
		b, err := bleve.Open(dir)
		if err == nil {
			if v, _ := b.GetInternal([]byte(versionKey)); string(v) == layoutVersion {
				return &Index{b: b}, false, nil
			}
			b.Close()
		}
		if err := os.RemoveAll(dir); err != nil {
			return nil, false, fmt.Errorf("search: clearing the index: %w", err)
		}
	}
	x, err = create(dir)
	return x, true, err
}

// Memory makes an index kept in memory only, on the same engine (scorch)
// as one kept on disk.
func Memory() (*Index, error) { return create("") }

func create(dir string) (*Index, error) {
	m, err := pageMapping()
	if err != nil {
		return nil, err
	}
	b, err := bleve.NewUsing(dir, m, scorch.Name, scorch.Name, nil)
	if err != nil {
		return nil, fmt.Errorf("search: making the index: %w", err)
	}
	if err := b.SetInternal([]byte(versionKey), []byte(layoutVersion)); err != nil {
		b.Close()
		return nil, err
	}
	return &Index{b: b}, nil
}

// Close lets the index go.
func (x *Index) Close() error { return x.b.Close() }

// Put keeps pages in the index, each in place of what it kept of them.
func (x *Index) Put(pages ...Page) error {
	batch := x.b.NewBatch()
	for _, p := range pages {
		if err := batch.Index(docID(p.Key), document(p)); err != nil {
			return err
		}
	}
	return x.b.Batch(batch)
}

// Delete lets pages go from the index.
func (x *Index) Delete(keys ...int64) error {
	batch := x.b.NewBatch()
	for _, k := range keys {
		batch.Delete(docID(k))
	}
	return x.b.Batch(batch)
}

// Revisions are the revisions of the pages the index holds.
func (x *Index) Revisions() (map[int64]uint64, error) {
	n, err := x.b.DocCount()
	if err != nil {
		return nil, err
	}
	req := bleve.NewSearchRequestOptions(bleve.NewMatchAllQuery(), int(n), 0, false)
	req.Fields = []string{"rev"}
	res, err := x.b.Search(req)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]uint64, len(res.Hits))
	for _, h := range res.Hits {
		k, ok := pageKey(h.ID)
		if !ok {
			return nil, fmt.Errorf("search: the index holds %q, not a page", h.ID)
		}
		rev, _ := h.Fields["rev"].(float64)
		out[k] = uint64(rev)
	}
	return out, nil
}

func docID(k int64) string { return strconv.FormatInt(k, 10) }

// pageKey reads a hit's ID: a page's, never one of its parts.
func pageKey(id string) (int64, bool) {
	k, err := strconv.ParseInt(id, 10, 64)
	return k, err == nil
}

func groupTerm(id int64) string { return "g" + strconv.FormatInt(id, 10) }

// document is a page as the index's mapping reads it.
func document(p Page) map[string]any {
	tags := make([]string, len(p.Tags))
	for i, t := range p.Tags {
		tags[i] = profile.TagKey(t)
	}
	groups := make([]string, len(p.Groups))
	for i, g := range p.Groups {
		groups[i] = groupTerm(g)
	}
	var domains []string
	for _, d := range p.Domains {
		domains = append(domains, domainSuffixes(d)...)
	}
	tasks := make([]any, len(p.Tasks))
	todo, done := false, false
	for i, t := range p.Tasks {
		tasks[i] = map[string]any{"id": t.ID, "text": t.Text, "done": t.Done}
		todo, done = todo || !t.Done, done || t.Done
	}
	return map[string]any{
		"rev":        float64(p.Rev),
		"title":      p.Title,
		"title_sort": query.Fold(p.Title),
		"headings":   p.Headings,
		"body":       p.Body,
		"code":       p.Code,
		"quote":      p.Quote,
		"tags":       tags,
		"tagged":     len(tags) > 0,
		"groups":     groups,
		"kinds":      p.Kinds,
		"domains":    domains,
		"tasks":      tasks,
		"todo":       todo,
		"done":       done,
		"created":    p.Created,
		"updated":    p.Updated,
		"words":      float64(p.Words),
		"trashed":    p.Trashed,
	}
}

// domainSuffixes are a site and the sites it is under: docs.github.com is
// also github.com.
func domainSuffixes(d string) []string {
	d = strings.TrimPrefix(query.Fold(strings.TrimSpace(d)), "www.")
	parts := strings.Split(d, ".")
	var out []string
	for i := 0; i+1 < len(parts); i++ {
		out = append(out, strings.Join(parts[i:], "."))
	}
	if len(out) == 0 && d != "" {
		out = append(out, d)
	}
	return out
}

// --- mapping --------------------------------------------------------------

const (
	analyzerIndexed = "neverland"
	analyzerQueried = "neverland-query"
)

type tokenizer struct{ mode query.Mode }

func (t tokenizer) Tokenize(in []byte) analysis.TokenStream {
	tokens := query.Tokens(string(in), t.mode)
	out := make(analysis.TokenStream, len(tokens))
	for i, tk := range tokens {
		kind := analysis.AlphaNumeric
		if r := []rune(tk.Term); len(r) > 0 && query.Unspaced(r[0]) {
			kind = analysis.Ideographic
		}
		out[i] = &analysis.Token{Term: []byte(tk.Term), Start: tk.Start, End: tk.End, Position: tk.Pos, Type: kind}
	}
	return out
}

func init() {
	for name, mode := range map[string]query.Mode{analyzerIndexed: query.Indexed, analyzerQueried: query.Queried} {
		registry.RegisterTokenizer(name, func(map[string]any, *registry.Cache) (analysis.Tokenizer, error) {
			return tokenizer{mode: mode}, nil
		})
	}
}

func pageMapping() (*mapping.IndexMappingImpl, error) {
	m := bleve.NewIndexMapping()
	for _, name := range []string{analyzerIndexed, analyzerQueried} {
		if err := m.AddCustomAnalyzer(name, map[string]any{"type": "custom", "tokenizer": name}); err != nil {
			return nil, err
		}
	}
	m.DefaultAnalyzer = analyzerIndexed
	m.ScoringModel = "bm25"
	m.IndexDynamic, m.StoreDynamic, m.DocValuesDynamic = false, false, false
	text := func() *mapping.FieldMapping {
		f := bleve.NewTextFieldMapping()
		f.Analyzer = analyzerIndexed
		f.Store, f.DocValues, f.IncludeInAll = false, false, false
		f.IncludeTermVectors = true
		return f
	}
	keyword := func(docValues bool) *mapping.FieldMapping {
		f := bleve.NewKeywordFieldMapping()
		f.Store, f.DocValues, f.IncludeInAll, f.IncludeTermVectors = false, docValues, false, false
		return f
	}
	flag := func() *mapping.FieldMapping {
		f := bleve.NewBooleanFieldMapping()
		f.Store, f.DocValues, f.IncludeInAll = false, false, false
		return f
	}
	date := func(store bool) *mapping.FieldMapping {
		f := bleve.NewDateTimeFieldMapping()
		f.Store, f.DocValues, f.IncludeInAll = store, true, false
		return f
	}
	number := func(index, store, docValues bool) *mapping.FieldMapping {
		f := bleve.NewNumericFieldMapping()
		f.Index, f.Store, f.DocValues, f.IncludeInAll = index, store, docValues, false
		return f
	}
	page := bleve.NewDocumentStaticMapping()
	page.AddFieldMappingsAt("rev", number(false, true, false))
	for _, name := range []string{"title", "headings", "body", "code", "quote"} {
		page.AddFieldMappingsAt(name, text())
	}
	page.AddFieldMappingsAt("title_sort", keyword(true))
	for _, name := range []string{"tags", "groups", "kinds"} {
		page.AddFieldMappingsAt(name, keyword(true))
	}
	page.AddFieldMappingsAt("domains", keyword(false))
	page.AddFieldMappingsAt("tagged", flag())
	// Whether it holds an open task, a done one: what $todo and $done ask
	// bare, so that turned they ask of the page, not of each task.
	page.AddFieldMappingsAt("todo", flag())
	page.AddFieldMappingsAt("done", flag())
	tasks := bleve.NewNestedDocumentMapping()
	tasks.Dynamic = false
	id := keyword(false)
	id.Store = true
	tasks.AddFieldMappingsAt("id", id)
	tasks.AddFieldMappingsAt("text", text())
	tasks.AddFieldMappingsAt("done", flag())
	page.AddSubDocumentMapping("tasks", tasks)
	page.AddFieldMappingsAt("trashed", flag())
	page.AddFieldMappingsAt("created", date(false))
	page.AddFieldMappingsAt("updated", date(true))
	page.AddFieldMappingsAt("words", number(true, false, true))
	m.DefaultMapping = page
	return m, m.Validate()
}
