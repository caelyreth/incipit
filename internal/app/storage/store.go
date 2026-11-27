package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

var (
	// ErrLocked reports a notebook another Incipit process holds.
	ErrLocked = errors.New("storage: the notebook is open in another Incipit window")
	// ErrForeign reports a database that is not a Incipit notebook.
	ErrForeign = errors.New("storage: the notebook file belongs to another application")
	// ErrNewer reports a notebook written by a newer Incipit.
	ErrNewer = errors.New("storage: the notebook was written by a newer Incipit")
	// ErrClosed reports use of a closed store.
	ErrClosed = errors.New("storage: the notebook is closed")
)

const (
	databaseName   = "notebook.db"
	lockName       = ".lock"
	restoredMarker = ".restored"
	backupDir      = "Backups"
	recoveryDir    = "Recovery"
)

// Store is one open notebook, held by this process alone until Close.
type Store struct {
	root string
	db   *sql.DB
	lock *os.File
	// media is what the notebook keeps of its pictures, by hash.
	media map[string]Media
	// Recovery is what opening found wrong and did about it.
	Recovery Recovery

	// launched is the backup this launch took, which a whole close pins
	// (pinBackup); "" for none.
	launched string

	mu     sync.Mutex
	closed bool
}

// Recovery reports what Open repaired. The zero value means the notebook
// opened as it was left.
type Recovery struct {
	// Damaged is the folder the damaged database was moved into, kept for
	// inspection; it is never deleted by the app.
	Damaged string
	// Restored is the backup that replaced it, or nothing when no backup
	// checked out and the notebook starts empty.
	Restored string
}

// Repaired reports whether opening had to replace the notebook.
func (r Recovery) Repaired() bool { return r.Damaged != "" }

func acquireLock(root string) (*os.File, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(filepath.Join(root, lockName), os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	if err := tryLock(lock); err != nil {
		_ = lock.Close()
		return nil, err
	}
	// The holder's process, for whoever finds the lock held.
	_ = lock.Truncate(0)
	_, _ = lock.WriteAt([]byte(fmt.Sprintf("%d\n", os.Getpid())), 0)
	return lock, nil
}

func releaseLock(lock *os.File) error {
	if lock == nil {
		return nil
	}
	err := unlock(lock)
	if closeErr := lock.Close(); err == nil {
		err = closeErr
	}
	return err
}

// Open takes the notebook in root for this process, creating the folder
// and the notebook as needed. A damaged notebook is moved aside and
// replaced from its newest sound backup; Recovery says so.
func Open(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	keepLocal(root)
	lock, err := acquireLock(root)
	if err != nil {
		return nil, err
	}

	store := &Store{root: root, lock: lock}
	if err := store.open(); err != nil {
		_ = releaseLock(lock)
		return nil, err
	}
	return store, nil
}

// Root is the folder the notebook is in.
func (s *Store) Root() string { return s.root }

func (s *Store) path() string { return filepath.Join(s.root, databaseName) }

func (s *Store) open() error {
	ctx := context.Background()
	info, statErr := os.Stat(s.path())
	existed := statErr == nil
	var db *sql.DB
	var err error
	if existed && info.Size() == 0 {
		// An existing 0-byte database file (crash/sync truncation) passes
		// SQLite quick_check as an empty DB and would otherwise wipe the
		// notebook instead of triggering backup recovery.
		err = fmt.Errorf("%w: empty notebook database", errDamaged)
	} else {
		db, err = openDatabase(s.path())
		if err == nil {
			err = check(ctx, db)
		}
	}
	if err != nil && damaged(err) {
		if db != nil {
			_ = db.Close()
		}
		db, err = s.recover(ctx)
	}
	if err != nil {
		if db != nil {
			_ = db.Close()
		}
		return err
	}
	if err := migrate(ctx, db); err != nil {
		_ = db.Close()
		return err
	}
	s.db = db
	closeOnErr := func(opErr error) error {
		_ = s.db.Close()
		s.db = nil
		return opErr
	}
	// A launch is a moment worth keeping: what the last session left.
	if existed && !s.Recovery.Repaired() {
		if err := s.backupIfStale(ctx, time.Now()); err != nil {
			return closeOnErr(err)
		}
	}
	if err := s.loadMedia(ctx); err != nil {
		return closeOnErr(err)
	}
	// Consume any one-launch marker left by RestoreBackup so post-backup media
	// belonging to Recovery/ is not purged immediately on first reopen.
	markerPath := filepath.Join(s.root, restoredMarker)
	_, restErr := os.Stat(markerPath)
	restored := restErr == nil
	if restored {
		_ = os.Remove(markerPath)
	}
	// A sweep that cannot finish tries again at the next launch: it never
	// stops the notebook opening. A notebook just repaired or restored is not
	// swept: the prior database moved to Recovery may still name newer pictures.
	if !s.Recovery.Repaired() && !restored {
		_ = s.sweepMedia(ctx, time.Now())
	}
	return nil
}

// openDatabase opens a notebook file with the durability it relies on:
// a write-ahead log, a commit counting only once it is synced, a full
// flush to the platter on macOS, and foreign keys enforced.
func openDatabase(path string) (*sql.DB, error) {
	if strings.ContainsRune(path, '?') {
		return nil, fmt.Errorf("storage: the notebook path cannot contain '?': %s", path)
	}
	pragmas := []string{
		"busy_timeout(5000)",
		"journal_mode(WAL)",
		"synchronous(FULL)",
		"fullfsync(1)",
		"checkpoint_fullfsync(1)",
		"foreign_keys(1)",
	}
	dsn := path + "?_txlock=immediate"
	for _, pragma := range pragmas {
		dsn += "&_pragma=" + pragma
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// One connection: the Saver is the one writer, and loading happens
	// before it starts.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	if err := db.Ping(); err != nil {
		return db, err
	}
	return db, nil
}

// errDamaged reports a database whose check found faults.
var errDamaged = errors.New("storage: the notebook is damaged")

// check runs SQLite's structural check over the whole file.
func check(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `PRAGMA quick_check`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var faults []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return err
		}
		if line != "ok" {
			faults = append(faults, line)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(faults) > 0 {
		return fmt.Errorf("%w: %s", errDamaged, strings.Join(faults, "; "))
	}
	return nil
}

// damaged reports whether an error says the file itself is broken, as
// opposed to unreadable for now (permissions, a full disk, a held lock),
// which must never send a notebook to recovery.
func damaged(err error) bool {
	if errors.Is(err, errDamaged) {
		return true
	}
	var failure *sqlite.Error
	if errors.As(err, &failure) {
		switch failure.Code() & 0xff {
		case sqlite3.SQLITE_CORRUPT, sqlite3.SQLITE_NOTADB:
			return true
		}
	}
	return false
}

// recover moves the damaged notebook, its log and its index into a
// Recovery folder, then installs the newest backup that checks out.
func (s *Store) recover(ctx context.Context) (*sql.DB, error) {
	backups, err := s.backups()
	if err != nil {
		return nil, err
	}
	for _, backup := range backups {
		if err := verifyFile(ctx, backup.path); err != nil {
			continue
		}
		folder, err := swapDatabase(ctx, s.root, backup.path)
		if err != nil {
			return nil, err
		}
		s.Recovery.Damaged = folder
		s.Recovery.Restored = backup.path
		break
	}
	if s.Recovery.Restored == "" {
		folder, _, err := moveDBAside(s.root)
		if err != nil {
			return nil, err
		}
		s.Recovery.Damaged = folder
		if err := syncDir(s.root); err != nil {
			return nil, err
		}
	}
	db, err := openDatabase(s.path())
	if err != nil {
		return db, err
	}
	return db, check(ctx, db)
}

// Close finishes the notebook's upkeep and lets another process take it.
// The caller stops its Saver first.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	ctx := context.Background()
	err := s.maintain(ctx)
	if pinErr := s.pinBackup(); err == nil {
		err = pinErr
	}
	if closeErr := s.db.Close(); err == nil {
		err = closeErr
	}
	if unlockErr := unlock(s.lock); err == nil {
		err = unlockErr
	}
	if closeErr := s.lock.Close(); err == nil {
		err = closeErr
	}
	return err
}

// maintain folds the log into the file, refreshes the planner's
// statistics, and hands freed pages back once they are a quarter of it.
func (s *Store) maintain(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA optimize`); err != nil {
		return err
	}
	var free, total int64
	if err := s.db.QueryRowContext(ctx, `PRAGMA freelist_count`).Scan(&free); err != nil {
		return err
	}
	if err := s.db.QueryRowContext(ctx, `PRAGMA page_count`).Scan(&total); err != nil {
		return err
	}
	if total > 0 && free*4 > total {
		if _, err := s.db.ExecContext(ctx, `PRAGMA incremental_vacuum`); err != nil {
			return err
		}
	}
	_, err := s.db.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`)
	return err
}
