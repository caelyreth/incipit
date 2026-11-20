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
		id      INTEGER PRIMARY KEY,
		created INTEGER NOT NULL,
		updated INTEGER NOT NULL,
		title   TEXT    NOT NULL DEFAULT '',
		excerpt TEXT    NOT NULL DEFAULT '',
		words   INTEGER NOT NULL DEFAULT 0
	) STRICT;
	CREATE TABLE groups (
		id       INTEGER PRIMARY KEY,
		parent   INTEGER NOT NULL,
		name     TEXT    NOT NULL,
		icon     TEXT    NOT NULL DEFAULT '',
		color    INTEGER NOT NULL DEFAULT 0,
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
	CREATE TABLE media (
		hash   TEXT    PRIMARY KEY,
		type   TEXT    NOT NULL,
		width  INTEGER NOT NULL,
		height INTEGER NOT NULL,
		bytes  INTEGER NOT NULL,
		name   TEXT    NOT NULL DEFAULT '',
		added  INTEGER NOT NULL
	) STRICT, WITHOUT ROWID;
	`,
	// Page content as node rows (editor/store): one row per node held in
	// a block slot, plus the root; text stored once and indexed.
	`
	ALTER TABLE pages ADD COLUMN format INTEGER NOT NULL DEFAULT 0;
	ALTER TABLE pages ADD COLUMN root   INTEGER NOT NULL DEFAULT 0;
	ALTER TABLE pages ADD COLUMN seed   INTEGER NOT NULL DEFAULT 0;
	CREATE TABLE nodes (
		page   INTEGER NOT NULL,
		id     INTEGER NOT NULL,
		parent INTEGER NOT NULL,
		slot   INTEGER NOT NULL,
		ord    TEXT    NOT NULL,
		kind   INTEGER NOT NULL,
		attrs  BLOB    NOT NULL,
		text   TEXT    NOT NULL,
		runs   BLOB    NOT NULL,
		rev    INTEGER NOT NULL DEFAULT 1,
		PRIMARY KEY (page, id)
	) STRICT;
	CREATE INDEX nodes_children ON nodes (page, parent, slot, ord);
	CREATE VIRTUAL TABLE nodes_fts USING fts5 (text, content = 'nodes', content_rowid = 'rowid');
	CREATE TRIGGER nodes_ai AFTER INSERT ON nodes BEGIN
		INSERT INTO nodes_fts (rowid, text) VALUES (new.rowid, new.text);
	END;
	CREATE TRIGGER nodes_ad AFTER DELETE ON nodes BEGIN
		INSERT INTO nodes_fts (nodes_fts, rowid, text) VALUES ('delete', old.rowid, old.text);
	END;
	CREATE TRIGGER nodes_au AFTER UPDATE OF text ON nodes BEGIN
		INSERT INTO nodes_fts (nodes_fts, rowid, text) VALUES ('delete', old.rowid, old.text);
		INSERT INTO nodes_fts (rowid, text) VALUES (new.rowid, new.text);
	END;
	`,
	// The tags a page writes, most written first, as JSON; NULL until they
	// are read from the page's content (readTags).
	`
	ALTER TABLE pages ADD COLUMN tags TEXT;
	`,
	// Search moved to its own index beside the notebook (app/search): the
	// text index inside it goes, with the triggers that kept it.
	`
	DROP TRIGGER nodes_ai;
	DROP TRIGGER nodes_ad;
	DROP TRIGGER nodes_au;
	DROP TABLE nodes_fts;
	`,
	// A group with a query is a smart folder: its sheets are what the
	// query finds (app/library).
	`
	ALTER TABLE groups ADD COLUMN query TEXT NOT NULL DEFAULT '';
	`,
	// A pinned group or sheet stands before the others where it is shown
	// (app/library).
	`
	ALTER TABLE groups ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0;
	ALTER TABLE sheets ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0;
	`,
	// A group can be kept shown as a list (app/library).
	`
	ALTER TABLE groups ADD COLUMN listed INTEGER NOT NULL DEFAULT 0;
	`,
	// The words written each day, by the day's date where it was written
	// (DayOf): what writing added to the pages, less what it took away.
	`
	CREATE TABLE days (
		day   TEXT    PRIMARY KEY,
		words INTEGER NOT NULL
	) STRICT, WITHOUT ROWID;
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
