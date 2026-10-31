package storage

import (
	"context"
	"database/sql"
	"fmt"
)

// applicationID marks a database as a Neverland notebook ("NVRL"), so the
// store never adopts, migrates or repairs a file that is someone else's.
const applicationID = 0x4e56524c

// migrations take a notebook from one schema version to the next: the
// database's user_version is how many have run. Each runs in the
// transaction that records it, so a notebook is never half migrated.
var migrations = []string{
	`
	CREATE TABLE pages (
		id        INTEGER PRIMARY KEY,
		created   INTEGER NOT NULL,
		updated   INTEGER NOT NULL,
		root      INTEGER NOT NULL,
		next_node INTEGER NOT NULL,
		title     TEXT    NOT NULL DEFAULT ''
	) STRICT;
	CREATE TABLE nodes (
		page       INTEGER NOT NULL REFERENCES pages (id) ON DELETE CASCADE,
		id         INTEGER NOT NULL,
		kind       TEXT    NOT NULL,
		parent     INTEGER,
		children   TEXT    NOT NULL,
		attributes TEXT    NOT NULL,
		marks      TEXT    NOT NULL,
		next_mark  INTEGER NOT NULL,
		text       TEXT,
		PRIMARY KEY (page, id)
	) STRICT, WITHOUT ROWID;
	CREATE TABLE groups (
		id       INTEGER PRIMARY KEY,
		parent   INTEGER NOT NULL,
		name     TEXT    NOT NULL,
		position INTEGER NOT NULL,
		trashed  INTEGER
	) STRICT;
	CREATE TABLE sheets (
		page     INTEGER PRIMARY KEY,
		grp      INTEGER NOT NULL,
		position INTEGER NOT NULL,
		trashed  INTEGER
	) STRICT;
	CREATE TABLE layout (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	) STRICT;
	`,
}

// migrate brings the notebook to the current schema, claiming a new file
// as a notebook first.
func migrate(ctx context.Context, db *sql.DB) error {
	var identity, version int64
	if err := db.QueryRowContext(ctx, `PRAGMA application_id`).Scan(&identity); err != nil {
		return err
	}
	if err := db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if identity != applicationID {
		var tables int
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_schema`).Scan(&tables); err != nil {
			return err
		}
		if identity != 0 || tables != 0 {
			return ErrForeign
		}
		// Freed pages go back to the file in steps (maintain), which
		// must be chosen before the first table.
		if _, err := db.ExecContext(ctx, `PRAGMA auto_vacuum = INCREMENTAL`); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`PRAGMA application_id = %d`, applicationID)); err != nil {
			return err
		}
	}
	if version > int64(len(migrations)) {
		return fmt.Errorf("%w: schema %d is newer than this build's %d", ErrNewer, version, len(migrations))
	}
	for next := version; next < int64(len(migrations)); next++ {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, migrations[next]); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("storage: migration %d: %w", next+1, err)
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, next+1)); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
