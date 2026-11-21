package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"maps"
	"slices"
	"time"

	"github.com/caelyreth/neverland/app/library"
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
)

// PageKey is a page's identity on disk, kept for the page's life.
type PageKey int64

// Summary is what a page's record shows of it in the library: its title,
// an excerpt, how many words it holds, the tags it writes and its tasks.
type Summary struct {
	Title, Excerpt string
	Words          int
	// Tags are the tags the page writes, most written first.
	Tags []Tag
	// Tasks are its task items, open and done.
	Tasks library.Tasks
	// Picture is the media of its first figure with a picture, "" for
	// none.
	Picture string
}

// Tag is a tag a page writes, as first written, and how often.
type Tag struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Equal reports whether two summaries say the same thing.
func (s Summary) Equal(o Summary) bool {
	return s.Title == o.Title && s.Excerpt == o.Excerpt && s.Words == o.Words && slices.Equal(s.Tags, o.Tags) && s.Tasks == o.Tasks && s.Picture == o.Picture
}

// encodeTags is a summary's tags as their column holds them.
func encodeTags(tags []Tag) string {
	if len(tags) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(tags)
	return string(b)
}

// Page is one stored page's record.
type Page struct {
	Key PageKey
	Summary
	Created time.Time
	Updated time.Time
}

// ViewCaret is one end of a saved selection.
type ViewCaret struct {
	Node   uint64 `json:"node"`
	Slot   int    `json:"slot"`
	Offset int    `json:"offset"`
	Stop   int    `json:"stop"`
}

// ViewState is a page's selection as the workspace keeps it.
type ViewState struct {
	Kind   uint8     `json:"kind"`
	Anchor ViewCaret `json:"anchor"`
	Head   ViewCaret `json:"head"`
	Node   uint64    `json:"node,omitempty"`
}

// Layout is how the notebook was left: its open pages in order, the page in
// front, where each page was scrolled, and each page's selection.
type Layout struct {
	Opened    []PageKey             `json:"tabs"`
	Active    PageKey               `json:"active"`
	HasActive bool                  `json:"hasActive"`
	Scroll    map[PageKey]float32   `json:"scroll,omitempty"`
	Views     map[PageKey]ViewState `json:"views,omitempty"`
}

// Equal reports whether two layouts say the same thing.
func (l Layout) Equal(other Layout) bool {
	return l.Active == other.Active && l.HasActive == other.HasActive &&
		slices.Equal(l.Opened, other.Opened) && maps.Equal(l.Scroll, other.Scroll) &&
		maps.Equal(l.Views, other.Views)
}

// clone returns a layout that shares no mutable storage with the caller.
func (l Layout) clone() Layout {
	l.Opened = slices.Clone(l.Opened)
	l.Scroll = maps.Clone(l.Scroll)
	l.Views = maps.Clone(l.Views)
	return l
}

// Notebook is everything a launch restores.
type Notebook struct {
	// Pages are the sound pages, oldest first.
	Pages  []Page
	Layout Layout
	// HasLayout reports whether a layout was saved at all.
	HasLayout bool
	// Library is how the pages are filed. A page it does not file is for
	// the caller to file.
	Library library.Records
	// NextKey is the key the next new page takes.
	NextKey PageKey
	// Written is the words written each day, by DayOf: what writing added
	// to the pages, less what it took away.
	Written map[string]int
}

const layoutKey = "workspace"

// Load reads the whole notebook.
func (s *Store) Load() (Notebook, error) {
	ctx := context.Background()
	notebook := Notebook{NextKey: 1}
	rows, err := s.db.QueryContext(ctx, `SELECT id, created, updated, title, excerpt, words, tags, open_tasks, done_tasks, picture FROM pages ORDER BY id`)
	if err != nil {
		return Notebook{}, err
	}
	// unread are the pages whose tags, tasks or picture were never read,
	// saved before pages kept them.
	var unread []int
	for rows.Next() {
		var page Page
		var created, updated int64
		var tags, picture sql.NullString
		var open, done sql.NullInt64
		if err := rows.Scan(&page.Key, &created, &updated, &page.Title, &page.Excerpt, &page.Words, &tags, &open, &done, &picture); err != nil {
			rows.Close()
			return Notebook{}, err
		}
		page.Created, page.Updated = time.UnixMilli(created), time.UnixMilli(updated)
		page.Tasks = library.Tasks{Open: int(open.Int64), Done: int(done.Int64)}
		page.Picture = picture.String
		switch {
		case !tags.Valid || !open.Valid || !done.Valid || !picture.Valid:
			unread = append(unread, len(notebook.Pages))
		case json.Unmarshal([]byte(tags.String), &page.Tags) != nil:
			// Tags that do not read are read again from the page.
			unread = append(unread, len(notebook.Pages))
		}
		notebook.NextKey = max(notebook.NextKey, page.Key+1)
		notebook.Pages = append(notebook.Pages, page)
	}
	if err := rows.Close(); err != nil {
		return Notebook{}, err
	}
	if err := s.readUnread(ctx, notebook.Pages, unread); err != nil {
		return Notebook{}, err
	}
	if notebook.Library, err = s.loadLibrary(ctx, notebook.Pages); err != nil {
		return Notebook{}, err
	}
	var encoded string
	switch err := s.db.QueryRowContext(ctx,
		`SELECT value FROM layout WHERE key = ?`, layoutKey).Scan(&encoded); err {
	case nil:
		// A layout that does not read only loses which pages were open; the pages
		// themselves are all listed.
		notebook.HasLayout = json.Unmarshal([]byte(encoded), &notebook.Layout) == nil
	case sql.ErrNoRows:
	default:
		return Notebook{}, err
	}
	if notebook.Written, err = loadWritten(ctx, s.db); err != nil {
		return Notebook{}, err
	}
	return notebook, nil
}

// readUnread reads the tags, tasks and picture of the pages at unread from their
// content and keeps them in their records, once: a page saved before
// pages kept them has them from then on.
func (s *Store) readUnread(ctx context.Context, pages []Page, unread []int) error {
	if len(unread) == 0 {
		return nil
	}
	// Each page is read before the records are written: the notebook
	// reads and writes through one connection.
	marks := doc.NewInterner()
	var read []int
	for _, i := range unread {
		page := &pages[i]
		page.Tags, page.Tasks, page.Picture = nil, library.Tasks{}, ""
		// A page that does not load keeps them unread, for the page
		// itself to report when it is opened.
		snap, err := s.LoadPage(page.Key, profile.Schema(), marks)
		if err != nil {
			continue
		}
		if snap != nil {
			read := Summarize(snap.Root())
			page.Tags, page.Tasks, page.Picture = read.Tags, read.Tasks, read.Picture
		}
		read = append(read, i)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, i := range read {
		if _, err := tx.ExecContext(ctx, `UPDATE pages SET tags = ?, open_tasks = ?, done_tasks = ?, picture = ? WHERE id = ?`,
			encodeTags(pages[i].Tags), pages[i].Tasks.Open, pages[i].Tasks.Done, pages[i].Picture, pages[i].Key); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// loadLibrary reads the groups and the filing of the sound pages.
func (s *Store) loadLibrary(ctx context.Context, pages []Page) (library.Records, error) {
	var records library.Records
	rows, err := s.db.QueryContext(ctx, `SELECT id, parent, name, icon, color, query, position, trashed, pinned, listed FROM groups`)
	if err != nil {
		return records, err
	}
	for rows.Next() {
		var group library.GroupRecord
		var trashed sql.NullInt64
		if err := rows.Scan(&group.ID, &group.Parent, &group.Name, &group.Icon, &group.Color, &group.Query, &group.Position, &trashed, &group.Pinned, &group.Listed); err != nil {
			rows.Close()
			return records, err
		}
		group.Trashed = timeOf(trashed)
		records.Groups = append(records.Groups, group)
	}
	if err := rows.Close(); err != nil {
		return records, err
	}
	known := make(map[PageKey]Page, len(pages))
	for _, page := range pages {
		known[page.Key] = page
	}
	rows, err = s.db.QueryContext(ctx, `SELECT page, grp, position, trashed, pinned FROM sheets`)
	if err != nil {
		return records, err
	}
	defer rows.Close()
	for rows.Next() {
		var sheet library.SheetRecord
		var trashed sql.NullInt64
		if err := rows.Scan(&sheet.ID, &sheet.Group, &sheet.Position, &trashed, &sheet.Pinned); err != nil {
			return records, err
		}
		page, ok := known[PageKey(sheet.ID)]
		if !ok {
			continue
		}
		sheet.Trashed = timeOf(trashed)
		sheet.Created, sheet.Updated = page.Created, page.Updated
		records.Sheets = append(records.Sheets, sheet)
	}
	return records, rows.Err()
}

func timeOf(milliseconds sql.NullInt64) time.Time {
	if !milliseconds.Valid {
		return time.Time{}
	}
	return time.UnixMilli(milliseconds.Int64)
}

func millisecondsOf(at time.Time) sql.NullInt64 {
	if at.IsZero() {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: at.UnixMilli(), Valid: true}
}

// pageWrite is one page's record as it is now, and, when its content
// changed, the rows to write.
type pageWrite struct {
	key     PageKey
	summary Summary
	content *contentWrite
}

// change is one transaction's worth of writes: page changes, pages
// deleted for good, the library's filing, the layout, the settings and
// the words written on the days that changed.
type change struct {
	pages    []pageWrite
	deleted  []PageKey
	library  *library.Records
	layout   *Layout
	settings []byte
	written  map[string]int
}

// write commits a change in one transaction: all of it is on disk, or
// none of it.
func (s *Store) write(ctx context.Context, change change, now time.Time) error {
	pages, layout := change.pages, change.layout
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, page := range pages {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO pages (id, created, updated, title, excerpt, words, tags, open_tasks, done_tasks, picture)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (id) DO UPDATE SET updated = excluded.updated, title = excluded.title,
				excerpt = excluded.excerpt, words = excluded.words, tags = excluded.tags,
				open_tasks = excluded.open_tasks, done_tasks = excluded.done_tasks, picture = excluded.picture`,
			page.key, now.UnixMilli(), now.UnixMilli(), page.summary.Title, page.summary.Excerpt, page.summary.Words,
			encodeTags(page.summary.Tags), page.summary.Tasks.Open, page.summary.Tasks.Done, page.summary.Picture,
		); err != nil {
			return err
		}
	}
	for _, page := range pages {
		if page.content != nil {
			if err := writeContent(ctx, tx, page.key, page.content); err != nil {
				return err
			}
		}
	}
	for _, key := range change.deleted {
		if _, err := tx.ExecContext(ctx, `DELETE FROM pages WHERE id = ?`, key); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM nodes WHERE page = ?`, key); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM sheets WHERE page = ?`, key); err != nil {
			return err
		}
	}
	if change.library != nil {
		if err := writeLibrary(ctx, tx, *change.library); err != nil {
			return err
		}
	}
	if layout != nil {
		encoded, err := json.Marshal(layout)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO layout (key, value) VALUES (?, ?)
			ON CONFLICT (key) DO UPDATE SET value = excluded.value`,
			layoutKey, string(encoded)); err != nil {
			return err
		}
	}
	if change.settings != nil {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO layout (key, value) VALUES (?, ?)
			ON CONFLICT (key) DO UPDATE SET value = excluded.value`,
			settingsKey, string(change.settings)); err != nil {
			return err
		}
	}
	if err := writeWritten(ctx, tx, change.written); err != nil {
		return err
	}
	return tx.Commit()
}

// writeLibrary replaces the library's groups and filing: they are small,
// and change a few at a time.
func writeLibrary(ctx context.Context, tx *sql.Tx, records library.Records) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM groups`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sheets`); err != nil {
		return err
	}
	for _, group := range records.Groups {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO groups (id, parent, name, icon, color, query, position, trashed, pinned, listed) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			group.ID, group.Parent, group.Name, group.Icon, group.Color, group.Query, group.Position,
			millisecondsOf(group.Trashed), group.Pinned, group.Listed); err != nil {
			return err
		}
	}
	for _, sheet := range records.Sheets {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO sheets (page, grp, position, trashed, pinned) VALUES (?, ?, ?, ?, ?)`,
			sheet.ID, sheet.Group, sheet.Position, millisecondsOf(sheet.Trashed), sheet.Pinned); err != nil {
			return err
		}
	}
	return nil
}
