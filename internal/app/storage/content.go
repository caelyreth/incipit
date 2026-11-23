package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/store"
)

// ErrFormat reports a page encoded in a format this build does not read;
// it is not opened.
var ErrFormat = errors.New("storage: unknown page format")

// contentWrite is a page's content save: a pointer diff of the last saved
// snapshot and the current one.
type contentWrite struct {
	root doc.ID
	seed uint64
	diff store.Diff
}

func writeContent(ctx context.Context, tx *sql.Tx, key PageKey, c *contentWrite) error {
	if _, err := tx.ExecContext(ctx, `UPDATE pages SET format = ?, root = ?, seed = ? WHERE id = ?`,
		store.Format, int64(c.root), int64(c.seed), key); err != nil {
		return err
	}
	for _, id := range c.diff.Delete {
		if _, err := tx.ExecContext(ctx, `DELETE FROM nodes WHERE page = ? AND id = ?`, key, int64(id)); err != nil {
			return err
		}
	}
	for _, r := range c.diff.Upsert {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO nodes (page, id, parent, slot, ord, kind, attrs, text, runs) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (page, id) DO UPDATE SET parent = excluded.parent, slot = excluded.slot, ord = excluded.ord,
				kind = excluded.kind, attrs = excluded.attrs, text = excluded.text, runs = excluded.runs, rev = rev + 1`,
			key, int64(r.ID), int64(r.Parent), r.Slot, r.Ord, int(r.Kind), r.Attrs, r.Text, r.Runs); err != nil {
			return err
		}
	}
	return nil
}

// LoadPage reads a page's content and validates every node. A page never
// given content, or not yet written at all, returns nil: the caller starts
// it empty.
func (s *Store) LoadPage(key PageKey, schema *doc.Schema, marks *doc.Interner) (*doc.Snapshot, error) {
	ctx := context.Background()
	var format, root, seed int64
	err := s.db.QueryRowContext(ctx, `SELECT format, root, seed FROM pages WHERE id = ?`, key).Scan(&format, &root, &seed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if format == 0 {
		return nil, nil
	}
	if format != store.Format {
		return nil, fmt.Errorf("%w: page %d has format %d", ErrFormat, key, format)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, parent, slot, ord, kind, attrs, text, runs FROM nodes WHERE page = ?`, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.Row
	for rows.Next() {
		var r store.Row
		var id, parent int64
		var kind int
		if err := rows.Scan(&id, &parent, &r.Slot, &r.Ord, &kind, &r.Attrs, &r.Text, &r.Runs); err != nil {
			return nil, err
		}
		r.ID, r.Parent, r.Kind = doc.ID(id), doc.ID(parent), doc.Kind(kind)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	snap, err := store.Load(schema, marks, doc.ID(root), uint64(seed), out)
	if err != nil {
		return nil, fmt.Errorf("storage: page %d: %w", key, err)
	}
	return snap, nil
}
