package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"maps"
	"slices"
	"time"

	"github.com/caelyreth/neverland/app/library"
)

// PageKey is a page's identity on disk, kept for the page's life.
type PageKey int64

// Summary is what a page's record shows of it in the library: its title,
// an excerpt and how many words it holds.
type Summary struct {
	Title, Excerpt string
	Words          int
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

// Layout is how the notebook was left: its open tabs in order, the page in
// front, where each page was scrolled, and each page's selection.
type Layout struct {
	Tabs      []PageKey             `json:"tabs"`
	Active    PageKey               `json:"active"`
	HasActive bool                  `json:"hasActive"`
	Scroll    map[PageKey]float32   `json:"scroll,omitempty"`
	Views     map[PageKey]ViewState `json:"views,omitempty"`
}

// Equal reports whether two layouts say the same thing.
func (l Layout) Equal(other Layout) bool {
	return l.Active == other.Active && l.HasActive == other.HasActive &&
		slices.Equal(l.Tabs, other.Tabs) && maps.Equal(l.Scroll, other.Scroll) &&
		maps.Equal(l.Views, other.Views)
}

// clone returns a layout that shares no mutable storage with the caller.
func (l Layout) clone() Layout {
	l.Tabs = slices.Clone(l.Tabs)
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
}

const layoutKey = "workspace"

// Load reads the whole notebook.
func (s *Store) Load() (Notebook, error) {
	ctx := context.Background()
	notebook := Notebook{NextKey: 1}
	rows, err := s.db.QueryContext(ctx, `SELECT id, created, updated, title, excerpt, words FROM pages ORDER BY id`)
	if err != nil {
		return Notebook{}, err
	}
	for rows.Next() {
		var page Page
		var created, updated int64
		if err := rows.Scan(&page.Key, &created, &updated, &page.Title, &page.Excerpt, &page.Words); err != nil {
			rows.Close()
			return Notebook{}, err
		}
		page.Created, page.Updated = time.UnixMilli(created), time.UnixMilli(updated)
		notebook.NextKey = max(notebook.NextKey, page.Key+1)
		notebook.Pages = append(notebook.Pages, page)
	}
	if err := rows.Close(); err != nil {
		return Notebook{}, err
	}
	if notebook.Library, err = s.loadLibrary(ctx, notebook.Pages); err != nil {
		return Notebook{}, err
	}
	var encoded string
	switch err := s.db.QueryRowContext(ctx,
		`SELECT value FROM layout WHERE key = ?`, layoutKey).Scan(&encoded); err {
	case nil:
		// A layout that does not read only loses tab order; the pages
		// themselves are all listed.
		notebook.HasLayout = json.Unmarshal([]byte(encoded), &notebook.Layout) == nil
	case sql.ErrNoRows:
	default:
		return Notebook{}, err
	}
	return notebook, nil
}

// loadLibrary reads the groups and the filing of the sound pages.
func (s *Store) loadLibrary(ctx context.Context, pages []Page) (library.Records, error) {
	var records library.Records
	rows, err := s.db.QueryContext(ctx, `SELECT id, parent, name, icon, color, position, trashed FROM groups`)
	if err != nil {
		return records, err
	}
	for rows.Next() {
		var group library.GroupRecord
		var trashed sql.NullInt64
		if err := rows.Scan(&group.ID, &group.Parent, &group.Name, &group.Icon, &group.Color, &group.Position, &trashed); err != nil {
			rows.Close()
			return records, err
		}
		group.Trashed = timeOf(trashed)
		records.Groups = append(records.Groups, group)
	}
	if err := rows.Close(); err != nil {
		return records, err
	}
	rows, err = s.db.QueryContext(ctx, `SELECT id, grp, name, position FROM chapters`)
	if err != nil {
		return records, err
	}
	for rows.Next() {
		var chapter library.ChapterRecord
		if err := rows.Scan(&chapter.ID, &chapter.Group, &chapter.Name, &chapter.Position); err != nil {
			rows.Close()
			return records, err
		}
		records.Chapters = append(records.Chapters, chapter)
	}
	if err := rows.Close(); err != nil {
		return records, err
	}
	known := make(map[PageKey]Page, len(pages))
	for _, page := range pages {
		known[page.Key] = page
	}
	rows, err = s.db.QueryContext(ctx, `SELECT page, grp, position, trashed FROM sheets`)
	if err != nil {
		return records, err
	}
	defer rows.Close()
	for rows.Next() {
		var sheet library.SheetRecord
		var trashed sql.NullInt64
		if err := rows.Scan(&sheet.ID, &sheet.Group, &sheet.Position, &trashed); err != nil {
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
// deleted for good, the library's filing and the layout.
type change struct {
	pages    []pageWrite
	deleted  []PageKey
	library  *library.Records
	layout   *Layout
	settings []byte
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
			INSERT INTO pages (id, created, updated, title, excerpt, words) VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (id) DO UPDATE SET updated = excluded.updated, title = excluded.title,
				excerpt = excluded.excerpt, words = excluded.words`,
			page.key, now.UnixMilli(), now.UnixMilli(), page.summary.Title, page.summary.Excerpt, page.summary.Words,
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
	if _, err := tx.ExecContext(ctx, `DELETE FROM chapters`); err != nil {
		return err
	}
	for _, group := range records.Groups {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO groups (id, parent, name, icon, color, position, trashed) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			group.ID, group.Parent, group.Name, group.Icon, group.Color, group.Position,
			millisecondsOf(group.Trashed)); err != nil {
			return err
		}
	}
	for _, sheet := range records.Sheets {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO sheets (page, grp, position, trashed) VALUES (?, ?, ?, ?)`,
			sheet.ID, sheet.Group, sheet.Position, millisecondsOf(sheet.Trashed)); err != nil {
			return err
		}
	}
	for _, chapter := range records.Chapters {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO chapters (id, grp, name, position) VALUES (?, ?, ?, ?)`,
			chapter.ID, chapter.Group, chapter.Name, chapter.Position); err != nil {
			return err
		}
	}
	return nil
}
