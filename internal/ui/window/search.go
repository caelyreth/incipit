package window

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"log"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/caelyreth/neverland/internal/app/library"
	"github.com/caelyreth/neverland/internal/app/search"
	"github.com/caelyreth/neverland/internal/app/storage"
	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
	"github.com/caelyreth/neverland/internal/text/query"
	"github.com/caelyreth/neverland/internal/ui/shell"
)

// indexFolder is where the notebook's search index is kept, beside the
// notebook in its folder. It is a cache (internal/app/search): losing it costs a
// rebuild, never a page.
const indexFolder = "Search Index"

// finder keeps the notebook's search index in step with its pages and
// answers the sidebar's searches.
type finder struct {
	index   *search.Index
	indexer *search.Indexer
	// wake asks for a frame as the index takes changes, from its
	// goroutine.
	wake atomic.Pointer[func()]
	// sent is each sheet's revision last handed to the indexer, and synced
	// whether the whole notebook was.
	sent   map[library.SheetID]uint64
	synced bool
	// handedFiling and handedSessions are the library's filing and the
	// sessions the whole notebook was last looked over at.
	handedFiling, handedSessions uint64
	// texts are the snapshots last read of the pages that have one, each
	// with a count that moves as the snapshot does: the index's revision of
	// a page whose text may be newer than the store's.
	texts map[library.SheetID]textState
	// found is the last search, kept while its query, what it reads
	// (queryStamp) and the lines read stand; counted the last count of a
	// key's values, and smart each smart folder's sheets, the same.
	found   *found
	counted *counted
	sites   *sites
	smart   map[string]*smartFound
	lines   *lines
}

// queryStamp is what a query's answer stands on beside its text: the
// index, the library, the sessions, the pages open ($open) and the minute
// (dates such as today and 7d). An answer is kept while its stamp is the
// one now.
type queryStamp struct {
	generation, rev, sessions uint64
	open                      string
	minute                    time.Time
}

// Revision is part of shell.Library: what queries stand on now, as one
// number.
func (a *AppState) Revision() uint64 {
	st := a.queryStamp()
	h := fnv.New64a()
	fmt.Fprintf(h, "%d/%d/%d/%s/%d", st.generation, st.rev, st.sessions, st.open, st.minute.Unix())
	return h.Sum64()
}

// queryStamp is what queries stand on now.
func (a *AppState) queryStamp() queryStamp {
	var open strings.Builder
	for _, id := range a.Workspace.Opened() {
		if session, ok := a.sessions[id]; ok {
			fmt.Fprintf(&open, "%d,", session.key)
		}
	}
	return queryStamp{
		generation: a.finder.indexer.Generation(), rev: a.library.Revision(), sessions: a.sessionsRev,
		open: open.String(), minute: time.Now().Truncate(time.Minute),
	}
}

type textState struct {
	snap   *doc.Snapshot
	serial uint64
}

// sites are the sites the notebook links to, kept while what they were
// read from stands.
type sites struct {
	stamp queryStamp
	names []string
}

type counted struct {
	query, key, values string
	stamp              queryStamp
	counts             map[string]int
}

type found struct {
	query  string
	stamp  queryStamp
	read   uint64
	result shell.Found
	close  bool
}

// smartFound is a smart folder's sheets, in the order its query finds
// them.
type smartFound struct {
	stamp queryStamp
	items []shell.SheetItem
}

// openFinder opens the notebook's index beside the store, or one in memory
// for a notebook held in memory.
func (a *AppState) openFinder(store *storage.Store) error {
	var x *search.Index
	var err error
	if store == nil {
		x, err = search.Memory()
	} else {
		x, _, err = search.Open(filepath.Join(store.Root(), indexFolder))
	}
	if err != nil {
		return err
	}
	f := &finder{index: x, sent: map[library.SheetID]uint64{}, texts: map[library.SheetID]textState{}, smart: map[string]*smartFound{}}
	changed := func() {
		if wake := f.wake.Load(); wake != nil {
			(*wake)()
		}
	}
	f.lines = newLines(changed)
	failed := func(err error) { log.Printf("search index: %v", err) }
	f.indexer, err = search.NewIndexer(x, changed, failed)
	if err != nil {
		x.Close()
		return err
	}
	a.finder = f
	a.reindex()
	return nil
}

// SetWake is how the index asks for a frame once it took changes; it is
// safe from any goroutine.
func (f *finder) SetWake(wake func()) { f.wake.Store(&wake) }

// closeFinder stops the indexer and lets the index go, before the store
// it reads from closes.
func (a *AppState) closeFinder() {
	if a.finder == nil {
		return
	}
	a.finder.indexer.Close()
	a.finder.lines.close()
	if err := a.finder.index.Close(); err != nil {
		log.Printf("closing the search index: %v", err)
	}
	a.finder = nil
}

// reindex hands the indexer the sheets that changed since it last looked.
// The whole notebook is looked over at first and when its filing or its
// sessions change; as pages are written, only the pages whose text moved.
func (a *AppState) reindex() {
	f := a.finder
	if f == nil {
		return
	}
	if !f.synced || f.handedFiling != a.library.Filing() || f.handedSessions != a.sessionsRev {
		a.reindexAll()
		return
	}
	paths := map[library.GroupID][]int64{}
	var changed []search.Entry
	for _, session := range a.sessions {
		id := library.SheetID(session.key)
		snap := session.text()
		if snap == nil || f.texts[id].snap == snap {
			continue
		}
		sheet, ok := a.library.Sheet(id)
		if !ok {
			continue
		}
		f.texts[id] = textState{snap: snap, serial: f.texts[id].serial + 1}
		e := a.indexEntry(sheet, session, f.texts[id], paths)
		f.sent[id] = e.Rev
		changed = append(changed, e)
	}
	if len(changed) > 0 {
		f.indexer.Put(changed...)
	}
}

// reindexAll looks the whole notebook over: what changed goes to the
// indexer, or everything at first, and the sheets that went are dropped.
func (a *AppState) reindexAll() {
	f := a.finder
	f.handedFiling, f.handedSessions = a.library.Filing(), a.sessionsRev
	sessions := a.sessionsBySheet()
	paths := map[library.GroupID][]int64{}
	var all, changed []search.Entry
	seen := make(map[library.SheetID]bool, len(sessions))
	for _, sheet := range a.library.Held() {
		session, ok := sessions[sheet.ID]
		if !ok {
			continue
		}
		seen[sheet.ID] = true
		text := f.texts[sheet.ID]
		if snap := session.text(); snap != nil && snap != text.snap {
			text = textState{snap: snap, serial: text.serial + 1}
			f.texts[sheet.ID] = text
		}
		e := a.indexEntry(sheet, session, text, paths)
		if f.synced && f.sent[sheet.ID] == e.Rev {
			continue
		}
		f.sent[sheet.ID] = e.Rev
		if f.synced {
			changed = append(changed, e)
		} else {
			all = append(all, e)
		}
	}
	if !f.synced {
		f.indexer.Sync(all)
		f.synced = true
		return
	}
	var gone []int64
	for id := range f.sent {
		if !seen[id] {
			delete(f.sent, id)
			delete(f.texts, id)
			gone = append(gone, int64(id))
		}
	}
	if len(changed) > 0 {
		f.indexer.Put(changed...)
	}
	if len(gone) > 0 {
		f.indexer.Drop(gone...)
	}
}

// indexEntry is a sheet as the index takes it. Its revision moves with
// whatever the index keeps of it: its text (the count of snapshots read
// this run, or its change time), its summary, its filing and whether it
// is in the trash. A page with a snapshot is read from it, since the store
// may not hold it yet; any other from the store.
func (a *AppState) indexEntry(sheet library.Sheet, session *sheetSession, text textState, paths map[library.GroupID][]int64) search.Entry {
	groups, ok := paths[sheet.Group]
	if !ok {
		for _, g := range a.library.Path(sheet.Group) {
			groups = append(groups, int64(g.ID))
		}
		paths[sheet.Group] = groups
	}
	meta := search.Meta{Groups: groups, Created: sheet.Created, Updated: sheet.Updated, Trashed: a.library.SheetInTrash(sheet.ID)}
	summary := session.summary
	h := fnv.New64a()
	var buf [8]byte
	number := func(n int64) {
		binary.LittleEndian.PutUint64(buf[:], uint64(n))
		h.Write(buf[:])
	}
	for _, n := range []int64{sheet.Created.UnixMilli(), sheet.Updated.UnixMilli(), int64(summary.Words), boolInt(meta.Trashed), int64(text.serial)} {
		number(n)
	}
	for _, g := range groups {
		number(g)
	}
	h.Write([]byte(summary.Title))
	for _, t := range summary.Tags {
		h.Write([]byte{0})
		h.Write([]byte(t.Name))
	}
	out := search.Text{Title: summary.Title, Tags: tagNames(summary.Tags), Words: summary.Words}
	snap, key := text.snap, session.key
	var store *storage.Store
	if a.notebook != nil {
		store = a.notebook.store
	}
	read := func() (search.Text, error) {
		switch {
		case snap != nil:
			out.Content = search.Read(snap.Root())
		case store != nil:
			loaded, err := store.LoadPage(key, profile.Schema(), doc.NewInterner())
			if err != nil {
				return out, err
			}
			if loaded != nil {
				out.Content = search.Read(loaded.Root())
			}
		}
		return out, nil
	}
	return search.Entry{Key: int64(sheet.ID), Rev: h.Sum64(), Meta: meta, Text: read}
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// searchSheets runs a query over the index: the sheets it finds, best
// first, with where their titles match and the lines of their texts that
// do, as they are read.
func (a *AppState) searchSheets(text string) shell.Found {
	f := a.finder
	if f == nil {
		return shell.Found{}
	}
	stamp, read := a.queryStamp(), f.lines.generation()
	if r := f.found; r != nil && r.query == text && r.stamp == stamp && r.read == read {
		return r.result
	}
	q := query.Parse(text)
	res, err := f.index.Search(q, search.Request{Env: a.searchEnv(), Limit: searchLimit})
	if err != nil {
		log.Printf("search: %v", err)
	}
	m := query.NewMatcher(q, res.Close)
	f.lines.query(text, m)
	sessions := a.sessionsBySheet()
	paths := map[library.GroupID][]int64{}
	out := shell.Found{Total: int(res.Total), Close: res.Close}
	var wanted []lineJob
	for _, hit := range res.Hits {
		id := library.SheetID(hit.Key)
		sheet, ok := a.library.Sheet(id)
		if !ok {
			continue
		}
		items := sheetItemsWithSessions([]library.Sheet{sheet}, sessions)
		if len(items) == 0 {
			continue
		}
		found := shell.FoundSheet{SheetItem: items[0], Score: hit.Score}
		if !m.Empty() {
			found.TitleMatches = m.Find(found.Title)
			if len(out.Sheets) < linesRead {
				key := lineKey{id: id, rev: f.sent[id]}
				if l, ok := f.lines.line(key); ok {
					found.Line, found.LineMatches = l.text, l.spans
				} else {
					session := sessions[id]
					wanted = append(wanted, lineJob{key: key, read: a.indexEntry(sheet, session, f.texts[id], paths).Text})
				}
			}
		}
		out.Sheets = append(out.Sheets, found)
	}
	f.lines.want(text, wanted)
	f.found = &found{query: text, stamp: stamp, read: f.lines.generation(), result: out, close: res.Close}
	return out
}

const (
	// searchLimit is how many sheets a search lists, and linesRead how
	// many of the best the lines of are read.
	searchLimit = 200
	linesRead   = 40
	// lineWidth is about how many characters a found line shows.
	lineWidth = 120
)

// SearchNames is part of shell.Library: the tags and the groups the
// notebook holds, for completion.
func (a *AppState) SearchNames(key string) []string {
	switch key {
	case "tag":
		return a.notebookTags(nil, "")
	case "in":
		var names []string
		seen := map[string]bool{}
		a.filingGroups(func(g library.Group) {
			if folded := query.Fold(g.Name); !seen[folded] {
				seen[folded] = true
				names = append(names, g.Name)
			}
		})
		return names
	case "link":
		return a.linkedSites()
	case "title":
		return a.titleWords()
	case "key":
		return a.Settings.KeyNames()
	}
	return nil
}

// namesMost is how many names a key offers at most.
const namesMost = 60

// linkedSites are the sites the notebook's sheets link to, most linked
// first.
func (a *AppState) linkedSites() []string {
	f := a.finder
	if f == nil {
		return nil
	}
	stamp := a.queryStamp()
	if r := f.sites; r != nil && r.stamp == stamp {
		return r.names
	}
	names, err := f.index.Sites(namesMost, a.searchEnv())
	if err != nil {
		log.Printf("search sites: %v", err)
	}
	f.sites = &sites{stamp: stamp, names: names}
	return names
}

// titleWords are the words the titles of the sheets out of the trash are
// written in, those in most titles first, as first written: what $title
// offers. Words of fewer than three letters, and the commonest words of
// English, say too little of a title to be offered.
func (a *AppState) titleWords() []string {
	// Walked in the pages' order, so words found as often stand the same
	// way each time.
	var titles []string
	for _, id := range slices.Sorted(maps.Keys(a.sessions)) {
		session := a.sessions[id]
		if sheet, ok := a.library.Sheet(library.SheetID(session.key)); ok && sheet.Trashed.IsZero() {
			titles = append(titles, session.summary.Title)
		}
	}
	return search.TitleWords(titles, namesMost)
}

// CountValues is part of shell.Library: how many sheets text finds with
// each of values for key as one condition more, kept while the query and
// what it reads stand. A value counted is in the map; one that cannot be,
// as a setting's, is not.
func (a *AppState) CountValues(text, key string, values []string) map[string]int {
	f := a.finder
	if f == nil || len(values) == 0 {
		return nil
	}
	stamp, joined := a.queryStamp(), strings.Join(values, "\x00")
	if r := f.counted; r != nil && r.query == text && r.key == key && r.values == joined && r.stamp == stamp {
		return r.counts
	}
	counts, err := f.index.Counts(query.Parse(text), key, values, a.searchEnv())
	if err != nil {
		log.Printf("search counts: %v", err)
	}
	f.counted = &counted{query: text, key: key, values: joined, stamp: stamp, counts: counts}
	return counts
}

// filingGroups visits the groups out of the trash that sheets are filed
// in, in the library's order: those $in names. A smart folder files none.
func (a *AppState) filingGroups(visit func(library.Group)) {
	var walk func(parent library.GroupID)
	walk = func(parent library.GroupID) {
		for _, g := range a.library.Groups(parent) {
			if !g.Smart() {
				visit(g)
				walk(g.ID)
			}
		}
	}
	walk(0)
}

// smartItems are the sheets a smart folder's query finds, in the order it
// finds them: every one, as the folder lists them all.
func (a *AppState) smartItems(text string) []shell.SheetItem {
	f := a.finder
	if f == nil {
		return nil
	}
	stamp := a.queryStamp()
	if r, ok := f.smart[text]; ok && r.stamp == stamp {
		return r.items
	}
	held := len(a.library.Held())
	res, err := f.index.Search(query.Parse(text), search.Request{Env: a.searchEnv(), Limit: max(held, 1)})
	if err != nil {
		log.Printf("smart folder: %v", err)
	}
	sheets := make([]library.Sheet, 0, len(res.Hits))
	for _, hit := range res.Hits {
		if sheet, ok := a.library.Sheet(library.SheetID(hit.Key)); ok {
			sheets = append(sheets, sheet)
		}
	}
	items := sheetItemsWithSessions(sheets, a.sessionsBySheet())
	// Only the folders shown are kept: a query no folder holds now goes.
	for q, r := range f.smart {
		if r.stamp != stamp {
			delete(f.smart, q)
		}
	}
	f.smart[text] = &smartFound{stamp: stamp, items: items}
	return items
}

// OpenFound is part of shell.Library: the sheet opens with the first of
// the query's matches in it selected, and in view.
func (a *AppState) OpenFound(name, text string, alongside bool) {
	a.OpenSheet(name, alongside)
	session, ok := a.sessionOf(name)
	if !ok {
		return
	}
	if page, ok := a.Workspace.Active(); !ok || page != session.page {
		return
	}
	el := a.pageElement()
	if el == nil {
		return
	}
	close := false
	if r := a.finder; r != nil && r.found != nil && r.found.query == text {
		close = r.found.close
	}
	m := query.NewMatcher(query.Parse(text), close)
	if m.Empty() {
		return
	}
	spot, ok := search.FirstMatch(el.Editor.Engine.Snapshot().Root(), m)
	if !ok {
		return
	}
	now := time.Now()
	el.Editor.Do(vocab.Place{Stop: doc.CaretAt(doc.At(spot.Slot, spot.Span.From))}, now)
	el.Editor.Do(vocab.Place{Stop: doc.CaretAt(doc.At(spot.Slot, spot.Span.To)), Extend: true}, now)
}

// searchEnv is what a query reads from the notebook now.
func (a *AppState) searchEnv() search.Env {
	env := search.Env{Now: time.Now(), Inbox: int64(library.Inbox)}
	env.Groups = func(name string) []int64 {
		var ids []int64
		a.filingGroups(func(g library.Group) {
			if query.Fold(g.Name) == name {
				ids = append(ids, int64(g.ID))
			}
		})
		return ids
	}
	for _, id := range a.Workspace.Opened() {
		if session, ok := a.sessions[id]; ok {
			env.Open = append(env.Open, int64(session.key))
		}
	}
	return env
}
