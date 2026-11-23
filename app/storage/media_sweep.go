package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/store"
)

// A picture is let go only at launch, before any page is open: then no
// history, tray, clipboard of this session or unsaved page can name it,
// and the saved pages are all there is. While the app runs no picture
// goes, so undoing a figure's deletion, pasting it again or putting back
// a sheet always finds its picture.
//
// What names a picture is counted afresh from the pages' rows each time,
// never kept beside them, so the counts cannot drift from the pages: one
// use per figure showing it, in every page the notebook keeps, sheets in
// the trash included. A picture goes once nothing names it, nor any
// backup kept (a backup restored finds every picture it shows), and once
// it is older than mediaGrace, which covers a figure a past session left
// on the clipboard. Its row goes first, then its file, so a crash between
// leaves only a file without a row, which the same sweep removes.
//
// A sweep that cannot read a page or a backup removes no picture: what
// it cannot read may name any of them.

// mediaGrace is how long a picture is kept after it was added, used or
// not.
const mediaGrace = 7 * 24 * time.Hour

// errUnreadable reports a page whose content a sweep cannot read.
var errUnreadable = errors.New("storage: a page's content cannot be read for its pictures")

// mediaUses counts, by media, the figures a notebook's pages show it in.
func mediaUses(ctx context.Context, db *sql.DB) (map[string]int, error) {
	var foreign int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM pages WHERE format NOT IN (0, ?)`,
		store.Format).Scan(&foreign); err != nil {
		return nil, err
	}
	if foreign > 0 {
		return nil, fmt.Errorf("%w: %d pages in another format", errUnreadable, foreign)
	}
	rows, err := db.QueryContext(ctx, `SELECT attrs FROM nodes WHERE kind = ?`, int(profile.Figure))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	uses := map[string]int{}
	for rows.Next() {
		var encoded []byte
		if err := rows.Scan(&encoded); err != nil {
			return nil, err
		}
		attrs, err := store.DecodeAttrs(encoded, nil)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errUnreadable, err)
		}
		if media, ok := attrs.Get(profile.FieldMedia); ok && media.Text() != "" {
			uses[media.Text()]++
		}
	}
	return uses, rows.Err()
}

// sweepMedia lets go of the pictures nothing names any more, then of the
// files no picture's row names.
func (s *Store) sweepMedia(ctx context.Context, now time.Time) error {
	if err := s.dropUnusedMedia(ctx, now); err != nil {
		return err
	}
	return s.sweepMediaFiles()
}

// dropUnusedMedia removes the rows of the pictures no page and no backup
// names, older than mediaGrace.
func (s *Store) dropUnusedMedia(ctx context.Context, now time.Time) error {
	uses, err := mediaUses(ctx, s.db)
	if err != nil {
		return err
	}
	unused := map[string]bool{}
	for hash, media := range s.media {
		if uses[hash] == 0 && now.Sub(media.Added) >= mediaGrace {
			unused[hash] = true
		}
	}
	if len(unused) == 0 {
		return nil
	}
	backups, err := s.backups()
	if err != nil {
		return err
	}
	for _, backup := range backups {
		if err := keepUsedIn(ctx, backup.path, unused); err != nil {
			return err
		}
		if len(unused) == 0 {
			return nil
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for hash := range unused {
		if _, err := tx.ExecContext(ctx, `DELETE FROM media WHERE hash = ?`, hash); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	for hash := range unused {
		delete(s.media, hash)
	}
	return nil
}

// keepUsedIn takes out of unused the pictures a backup's pages name.
func keepUsedIn(ctx context.Context, path string, unused map[string]bool) error {
	db, err := sql.Open("sqlite", path+"?_pragma=query_only(1)")
	if err != nil {
		return err
	}
	defer db.Close()
	// A backup from before pages were kept as rows shows no figure.
	var rowed int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = 'nodes'`).Scan(&rowed); err != nil {
		return err
	}
	if rowed == 0 {
		return nil
	}
	uses, err := mediaUses(ctx, db)
	if err != nil {
		return fmt.Errorf("storage: backup %s: %w", filepath.Base(path), err)
	}
	for hash := range uses {
		delete(unused, hash)
	}
	return nil
}

// sweepMediaFiles removes the files the notebook has no row for: a
// picture let go, a crash between a file and its row, or a partial file.
func (s *Store) sweepMediaFiles() error {
	folders, err := os.ReadDir(filepath.Join(s.root, mediaDir))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, folder := range folders {
		if !folder.IsDir() {
			continue
		}
		path := filepath.Join(s.root, mediaDir, folder.Name())
		files, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, file := range files {
			hash := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
			if known, ok := s.media[hash]; ok && filepath.Base(s.MediaFile(known)) == file.Name() {
				continue
			}
			if err := os.Remove(filepath.Join(path, file.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return nil
}
