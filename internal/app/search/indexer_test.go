package search

import (
	"github.com/caelyreth/neverland/internal/text/query"

	"errors"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/profile"
)

func entry(key int64, rev uint64, body string, reads *atomic.Int32) Entry {
	return Entry{Key: key, Rev: rev, Text: func() (Text, error) {
		reads.Add(1)
		return Text{Title: body, Content: Content{Body: body}}, nil
	}}
}

func TestTheIndexerReadsOnlyWhatTheIndexLacks(t *testing.T) {
	x, err := Memory()
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	var reads atomic.Int32
	var changes atomic.Int32
	ix, err := NewIndexer(x, func() { changes.Add(1) }, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ix.Close()
	ix.Sync([]Entry{entry(1, 1, "alpha", &reads), entry(2, 1, "beta", &reads)})
	ix.Flush()
	if reads.Load() != 2 || ix.Generation() != 1 || changes.Load() != 1 {
		t.Fatalf("reads %d generation %d changes %d", reads.Load(), ix.Generation(), changes.Load())
	}
	// The same revisions again read nothing; a new one reads its page.
	ix.Sync([]Entry{entry(1, 1, "alpha", &reads), entry(2, 2, "gamma", &reads)})
	ix.Flush()
	if reads.Load() != 3 {
		t.Fatalf("reads %d", reads.Load())
	}
	res, _ := x.Search(query.Parse("gamma"), Request{})
	if !slices.Equal(keys(res), []int64{2}) {
		t.Fatalf("gamma %v", keys(res))
	}
	// A page the notebook no longer holds goes.
	ix.Sync([]Entry{entry(2, 2, "gamma", &reads)})
	ix.Flush()
	if revs, _ := x.Revisions(); len(revs) != 1 || revs[2] != 2 {
		t.Fatalf("revisions %v", revs)
	}
	ix.Drop(2)
	ix.Put(entry(3, 1, "delta", &reads))
	ix.Flush()
	if revs, _ := x.Revisions(); len(revs) != 1 || revs[3] != 1 {
		t.Fatalf("revisions %v", revs)
	}
}

func TestAPageThatCannotBeReadIsTriedAgain(t *testing.T) {
	x, _ := Memory()
	defer x.Close()
	var failures atomic.Int32
	ix, _ := NewIndexer(x, nil, func(error) { failures.Add(1) })
	defer ix.Close()
	broken := Entry{Key: 1, Rev: 1, Text: func() (Text, error) { return Text{}, errors.New("unreadable") }}
	ix.Sync([]Entry{broken})
	ix.Flush()
	var reads atomic.Int32
	ix.Sync([]Entry{entry(1, 1, "alpha", &reads)})
	ix.Flush()
	if failures.Load() != 1 || reads.Load() != 1 {
		t.Fatalf("failures %d reads %d", failures.Load(), reads.Load())
	}
}

func TestChangesSettleBeforeTheyAreRead(t *testing.T) {
	x, _ := Memory()
	defer x.Close()
	ix, _ := NewIndexer(x, nil, nil)
	defer ix.Close()
	var reads atomic.Int32
	for rev := range uint64(20) {
		ix.Put(entry(1, rev+1, "alpha", &reads))
	}
	ix.Flush()
	if reads.Load() != 1 {
		t.Fatalf("a page changed many times is read once: %d", reads.Load())
	}
}

func TestReadTakesAPagesTextsKindsAndSites(t *testing.T) {
	text := func(s string, marks *doc.MarkSet) doc.Item { return doc.TextItem(s, marks) }
	in := doc.NewInterner()
	code := in.Intern([]doc.Mark{{Kind: profile.CodeMark}})
	heading := doc.NewNode(profile.Heading, profile.Schema().Canonical(profile.Heading, doc.Attrs{}), doc.InlineSlot(text("Plans", nil)))
	para := profile.NewParagraph(text("See https://docs.github.com/x and run ", nil), text("go test", code))
	quote := doc.NewNode(profile.Quote, doc.Attrs{}, doc.BlockSlot(profile.NewParagraph(text("a quoted line", nil))))
	root := doc.NewNode(profile.Page, doc.Attrs{}, doc.BlockSlot(heading, para, quote))
	c := Read(root)
	if c.Headings != "Plans" || c.Quote != "a quoted line" || c.Code != "go test" {
		t.Fatalf("content %+v", c)
	}
	if c.Body != "Plans\nSee https://docs.github.com/x and run go test\na quoted line" {
		t.Fatalf("body %q", c.Body)
	}
	if !slices.Equal(c.Kinds, []string{"code", "link", "quote"}) || !slices.Equal(c.Domains, []string{"docs.github.com"}) {
		t.Fatalf("kinds %v domains %v", c.Kinds, c.Domains)
	}
}

func TestDomainsAreTheSitesAddressesName(t *testing.T) {
	got := Domains("at https://Example.com:8080/a, www.notes.org, ftp://files.net and mailto:x — not.this or a://b")
	if !slices.Equal(got, []string{"example.com", "notes.org", "files.net"}) {
		t.Fatalf("domains %v", got)
	}
}
