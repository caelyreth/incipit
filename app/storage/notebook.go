package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/caelyreth/neverland/app/library"
	"github.com/caelyreth/neverland/engine/model"
)

// PageKey is a page's identity on disk, kept for the page's life.
type PageKey int64

// Page is one stored page, rebuilt as a document.
type Page struct {
	Key      PageKey
	Document *model.Document
	Created  time.Time
	Updated  time.Time
}

// Layout is how the notebook was left: its open tabs in order, the page in
// front, and where each page was scrolled and selected.
type Layout struct {
	Tabs       []PageKey             `json:"tabs"`
	Active     PageKey               `json:"active"`
	HasActive  bool                  `json:"hasActive"`
	Scroll     map[PageKey]float32   `json:"scroll,omitempty"`
	Selections map[PageKey]Selection `json:"selections,omitempty"`
}

// Position is a place in a page's text, as the model addresses it: a node,
// a UTF-16 offset in its text, and the side of that boundary it keeps to.
type Position struct {
	Node     uint64 `json:"node"`
	Offset   int    `json:"offset"`
	Affinity uint8  `json:"affinity"`
}

// Selection is a page's selection, from where it was started to where it
// was moved; a caret is one whose ends are the same.
type Selection struct {
	Anchor Position `json:"anchor"`
	Head   Position `json:"head"`
}

// Damage is a page whose records no longer make a document. Its rows stay
// untouched on disk.
type Damage struct {
	Key PageKey
	Err error
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
	// Damaged pages are left on disk as they are, and their keys are never
	// given to a new page.
	Damaged []Damage
	// NextKey is the key the next new page takes.
	NextKey PageKey
}

const layoutKey = "workspace"

// Load reads the whole notebook.
func (s *Store) Load() (Notebook, error) {
	ctx := context.Background()
	notebook := Notebook{NextKey: 1}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, created, updated, root, next_node FROM pages ORDER BY id`)
	if err != nil {
		return Notebook{}, err
	}
	type header struct {
		key              PageKey
		created, updated int64
		root, nextNode   int64
	}
	var headers []header
	for rows.Next() {
		var row header
		if err := rows.Scan(&row.key, &row.created, &row.updated, &row.root, &row.nextNode); err != nil {
			rows.Close()
			return Notebook{}, err
		}
		headers = append(headers, row)
	}
	if err := rows.Close(); err != nil {
		return Notebook{}, err
	}
	for _, row := range headers {
		notebook.NextKey = max(notebook.NextKey, row.key+1)
		document, err := s.loadDocument(ctx, row.key, model.NodeID(row.root), uint64(row.nextNode))
		if err != nil {
			notebook.Damaged = append(notebook.Damaged, Damage{Key: row.key, Err: err})
			continue
		}
		notebook.Pages = append(notebook.Pages, Page{
			Key:      row.key,
			Document: document,
			Created:  time.UnixMilli(row.created),
			Updated:  time.UnixMilli(row.updated),
		})
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

func (s *Store) loadDocument(
	ctx context.Context,
	key PageKey,
	root model.NodeID,
	nextNode uint64,
) (*model.Document, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, kind, parent, children, attributes, marks, next_mark, text
		FROM nodes WHERE page = ? ORDER BY id`, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []model.NodeRecord
	for rows.Next() {
		var (
			id, nextMark               int64
			kind, children, attributes string
			marks                      string
			parent                     sql.NullInt64
			text                       sql.NullString
		)
		if err := rows.Scan(&id, &kind, &parent, &children, &attributes, &marks, &nextMark, &text); err != nil {
			return nil, err
		}
		record := model.NodeRecord{
			ID:         model.NodeID(id),
			Kind:       model.NodeTypeID(kind),
			Parent:     model.NodeID(parent.Int64),
			HasParent:  parent.Valid,
			NextMarkID: uint64(nextMark),
		}
		if record.Children, err = decodeChildren(children); err != nil {
			return nil, fmt.Errorf("node %d children: %w", id, err)
		}
		if record.Attributes, err = decodeAttributes(attributes); err != nil {
			return nil, fmt.Errorf("node %d attributes: %w", id, err)
		}
		if record.Marks, err = decodeMarks(marks); err != nil {
			return nil, fmt.Errorf("node %d marks: %w", id, err)
		}
		if text.Valid {
			record.Text = &text.String
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return model.RestoreDocument(root, nextNode, records)
}

// pageWrite is one page's change since what is on disk: its whole tree
// when nothing of it is (stored false), else the nodes written and removed
// since the stored snapshot.
type pageWrite struct {
	key      PageKey
	title    string
	snapshot model.DocumentSnapshot
	stored   model.DocumentSnapshot
	known    bool
}

// change is one transaction's worth of writes: page changes, pages
// deleted for good, the library's filing and the layout.
type change struct {
	pages   []pageWrite
	deleted []PageKey
	library *library.Records
	layout  *Layout
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
	upsertNode, err := tx.PrepareContext(ctx, `
		INSERT INTO nodes (page, id, kind, parent, children, attributes, marks, next_mark, text)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (page, id) DO UPDATE SET
			kind = excluded.kind, parent = excluded.parent,
			children = excluded.children, attributes = excluded.attributes,
			marks = excluded.marks, next_mark = excluded.next_mark,
			text = excluded.text`)
	if err != nil {
		return err
	}
	defer upsertNode.Close()
	for _, page := range pages {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO pages (id, created, updated, root, next_node, title)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (id) DO UPDATE SET
				updated = excluded.updated, root = excluded.root,
				next_node = excluded.next_node, title = excluded.title`,
			page.key, now.UnixMilli(), now.UnixMilli(),
			int64(page.snapshot.Root()), int64(page.snapshot.NextID()), page.title,
		); err != nil {
			return err
		}
		var written []*model.Node
		var removed []model.NodeID
		if page.known {
			written, removed = page.snapshot.Since(page.stored)
		} else {
			if _, err := tx.ExecContext(ctx, `DELETE FROM nodes WHERE page = ?`, page.key); err != nil {
				return err
			}
			written = page.snapshot.Nodes()
		}
		for _, id := range removed {
			if _, err := tx.ExecContext(ctx,
				`DELETE FROM nodes WHERE page = ? AND id = ?`, page.key, int64(id)); err != nil {
				return err
			}
		}
		for _, node := range written {
			if err := writeNode(ctx, upsertNode, page.key, node.Record()); err != nil {
				return err
			}
		}
	}
	for _, key := range change.deleted {
		if _, err := tx.ExecContext(ctx, `DELETE FROM pages WHERE id = ?`, key); err != nil {
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

func writeNode(ctx context.Context, statement *sql.Stmt, page PageKey, record model.NodeRecord) error {
	children, err := encodeChildren(record.Children)
	if err != nil {
		return err
	}
	attributes, err := encodeAttributes(record.Attributes)
	if err != nil {
		return err
	}
	marks, err := encodeMarks(record.Marks)
	if err != nil {
		return err
	}
	var parent sql.NullInt64
	if record.HasParent {
		parent = sql.NullInt64{Int64: int64(record.Parent), Valid: true}
	}
	var text sql.NullString
	if record.Text != nil {
		text = sql.NullString{String: *record.Text, Valid: true}
	}
	_, err = statement.ExecContext(ctx,
		page, int64(record.ID), string(record.Kind), parent,
		children, attributes, marks, int64(record.NextMarkID), text)
	return err
}
