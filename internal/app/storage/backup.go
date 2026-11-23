package storage

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	// backupInterval is how old the newest backup may grow while the
	// notebook changes.
	backupInterval = 30 * time.Minute
	// recentBackups are kept whatever their age.
	recentBackups = 12
	// dailyBackups is how many days keep their newest backup beyond the
	// recent ones.
	dailyBackups = 30

	backupPrefix = "notebook-"
	backupSuffix = ".db"
	backupLayout = "20060102-150405"
)

type backup struct {
	path  string
	taken time.Time
}

// backups lists the notebook's backups, newest first.
func (s *Store) backups() ([]backup, error) {
	entries, err := os.ReadDir(filepath.Join(s.root, backupDir))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var found []backup
	for _, entry := range entries {
		name := entry.Name()
		stamp, ok := strings.CutPrefix(name, backupPrefix)
		if !ok || entry.IsDir() {
			continue
		}
		stamp, ok = strings.CutSuffix(stamp, backupSuffix)
		if !ok {
			continue
		}
		taken, err := time.ParseInLocation(backupLayout, stamp, time.Local)
		if err != nil {
			continue
		}
		found = append(found, backup{path: filepath.Join(s.root, backupDir, name), taken: taken})
	}
	slices.SortFunc(found, func(left, right backup) int { return right.taken.Compare(left.taken) })
	return found, nil
}

// backupIfStale takes a backup when the newest is older than the interval.
func (s *Store) backupIfStale(ctx context.Context, now time.Time) error {
	backups, err := s.backups()
	if err != nil {
		return err
	}
	if len(backups) > 0 && now.Sub(backups[0].taken) < backupInterval {
		return nil
	}
	return s.backup(ctx, now)
}

// backup writes a consistent copy of the notebook, checks it, and only
// then gives it a backup's name, so a listed backup is always a sound one.
func (s *Store) backup(ctx context.Context, now time.Time) error {
	folder := filepath.Join(s.root, backupDir)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return err
	}
	name := filepath.Join(folder, backupPrefix+now.Format(backupLayout)+backupSuffix)
	partial := name + ".partial"
	_ = os.Remove(partial)
	if _, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, partial); err != nil {
		if removeErr := os.Remove(partial); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return errors.Join(err, removeErr)
		}
		return err
	}
	if err := completePreparedFile(name, partial, func(path string) error {
		return verifyFile(ctx, path)
	}); err != nil {
		return err
	}
	return s.prune()
}

// prune keeps the recent backups, then the newest of each day.
func (s *Store) prune() error {
	backups, err := s.backups()
	if err != nil {
		return err
	}
	days := map[string]bool{}
	for index, backup := range backups {
		day := backup.taken.Format("20060102")
		keep := index < recentBackups || (!days[day] && len(days) < dailyBackups)
		days[day] = true
		if !keep {
			if err := os.Remove(backup.path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return nil
}

// verifyFile opens a database file read-only and checks it is a sound
// notebook.
func verifyFile(ctx context.Context, path string) error {
	db, err := sql.Open("sqlite", path+"?_pragma=query_only(1)")
	if err != nil {
		return err
	}
	defer db.Close()
	if err := check(ctx, db); err != nil {
		return err
	}
	var identity int64
	if err := db.QueryRowContext(ctx, `PRAGMA application_id`).Scan(&identity); err != nil {
		return err
	}
	if identity != applicationID {
		return ErrForeign
	}
	return nil
}
