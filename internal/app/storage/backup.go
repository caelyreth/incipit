package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

	// pinnedName names, in the backups' folder, the backup pinned: the one
	// a launch took of what it opened, kept by the session that then
	// closed whole. Pruning never lets it go, so launches that crash, as
	// many as they are, never rotate out the last state known to open.
	pinnedName = "pinned"

	backupPrefix = "notebook-"
	backupSuffix = ".db"
	backupLayout = "20060102-150405"
)

type backup struct {
	path  string
	taken time.Time
}

// BackupInfo describes a verified backup stored in Backups/.
type BackupInfo struct {
	Path  string
	Name  string
	Taken time.Time
	Size  int64
}

// ListBackups returns all verified notebook backups in root newest first.
func ListBackups(root string) ([]BackupInfo, error) {
	ctx := context.Background()
	raw, err := (&Store{root: root}).backups()
	if err != nil {
		return nil, err
	}
	var out []BackupInfo
	for _, b := range raw {
		if err := verifyFile(ctx, b.path); err != nil {
			continue
		}
		info, err := os.Stat(b.path)
		if err != nil {
			continue
		}
		out = append(out, BackupInfo{
			Path:  b.path,
			Name:  filepath.Base(b.path),
			Taken: b.taken,
			Size:  info.Size(),
		})
	}
	return out, nil
}

// RestoreBackup locks root, moves existing database files aside into Recovery/,
// and transactionally installs backupPath. If copying fails, any moved files
// are rolled back from Recovery/ so root is never left without notebook.db.
func RestoreBackup(root, backupPath string) error {
	ctx := context.Background()
	lock, err := acquireLock(root)
	if err != nil {
		return err
	}
	defer func() { _ = releaseLock(lock) }()

	if _, err := swapDatabase(ctx, root, backupPath); err != nil {
		return err
	}
	// Mark so the immediate reopen skips purging media referenced only by
	// the pre-restore database preserved in Recovery/.
	_ = os.WriteFile(filepath.Join(root, restoredMarker), nil, 0o644)
	_ = syncDir(root)
	return nil
}

// swapDatabase verifies backupPath, moves existing DB files aside into
// Recovery/, and copies backupPath to notebook.db, rolling back on failure.
func swapDatabase(ctx context.Context, root, backupPath string) (string, error) {
	if err := verifyFile(ctx, backupPath); err != nil {
		return "", err
	}
	folder, moved, err := moveDBAside(root)
	if err != nil {
		return "", err
	}
	if err := syncDir(root); err != nil {
		rollbackDBAside(root, folder, moved)
		return "", err
	}
	dbPath := filepath.Join(root, databaseName)
	if err := copyDurably(backupPath, dbPath); err != nil {
		_ = os.Remove(dbPath)
		rollbackDBAside(root, folder, moved)
		return "", err
	}
	return folder, nil
}

func moveDBAside(root string) (folder string, moved []string, err error) {
	folder, err = uniqueSubdir(filepath.Join(root, recoveryDir), time.Now().Format("20060102-150405"))
	if err != nil {
		return "", nil, err
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		name := databaseName + suffix
		from := filepath.Join(root, name)
		if _, statErr := os.Stat(from); statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				continue
			}
			rollbackDBAside(root, folder, moved)
			return "", nil, statErr
		}
		if renameErr := os.Rename(from, filepath.Join(folder, name)); renameErr != nil {
			rollbackDBAside(root, folder, moved)
			return "", nil, renameErr
		}
		moved = append(moved, name)
	}
	if len(moved) == 0 {
		_ = os.Remove(folder)
		return "", nil, nil
	}
	return folder, moved, nil
}

func rollbackDBAside(root, folder string, moved []string) {
	if folder == "" {
		return
	}
	for i := len(moved) - 1; i >= 0; i-- {
		_ = os.Rename(filepath.Join(folder, moved[i]), filepath.Join(root, moved[i]))
	}
	_ = syncDir(root)
	_ = os.Remove(folder)
}

func uniqueSubdir(parent, base string) (string, error) {
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", err
	}
	for n := 0; ; n++ {
		name := base
		if n > 0 {
			name = fmt.Sprintf("%s-%d", base, n+1)
		}
		candidate := filepath.Join(parent, name)
		err := os.Mkdir(candidate, 0o755)
		if err == nil {
			return candidate, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
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
	if name, err := s.backupNamed(ctx, now); err != nil {
		return err
	} else {
		s.launched = name
	}
	return nil
}

// pinBackup pins the backup this launch took, as the session closes
// whole: what it opened, it opened and kept working with.
func (s *Store) pinBackup() error {
	if s.launched == "" {
		return nil
	}
	folder := filepath.Join(s.root, backupDir)
	partial := filepath.Join(folder, pinnedName+".partial")
	if err := os.WriteFile(partial, []byte(s.launched), 0o644); err != nil {
		return err
	}
	// Synced before it is named, as a backup is: a crash leaves the pin as
	// it was or as written, never empty, so pruning keeps what it names.
	return completePreparedFile(filepath.Join(folder, pinnedName), partial, nil)
}

// pinned is the name of the backup pinned, "" for none.
func (s *Store) pinned() string {
	data, err := os.ReadFile(filepath.Join(s.root, backupDir, pinnedName))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// backup writes a consistent copy of the notebook, checks it, and only
// then gives it a backup's name, so a listed backup is always a sound one.
func (s *Store) backup(ctx context.Context, now time.Time) error {
	_, err := s.backupNamed(ctx, now)
	return err
}

func (s *Store) backupNamed(ctx context.Context, now time.Time) (string, error) {
	folder := filepath.Join(s.root, backupDir)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return "", err
	}
	// Sweep orphaned .partial files from prior interrupted backup runs.
	if partials, err := filepath.Glob(filepath.Join(folder, backupPrefix+"*"+backupSuffix+".partial")); err == nil {
		for _, p := range partials {
			_ = os.Remove(p)
		}
	}
	name, _ := uniqueBackupPath(folder, now)
	partial := name + ".partial"
	_ = os.Remove(partial)
	if _, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, partial); err != nil {
		if removeErr := os.Remove(partial); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return "", errors.Join(err, removeErr)
		}
		return "", err
	}
	if err := completePreparedFile(name, partial, func(path string) error {
		return verifyFile(ctx, path)
	}); err != nil {
		return "", err
	}
	if err := s.prune(); err != nil {
		return "", err
	}
	return filepath.Base(name), nil
}

func uniqueBackupPath(folder string, now time.Time) (string, time.Time) {
	t := now
	for range 60 {
		candidate := filepath.Join(folder, backupPrefix+t.Format(backupLayout)+backupSuffix)
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, t
		}
		t = t.Add(time.Second)
	}
	return filepath.Join(folder, backupPrefix+t.Format(backupLayout)+backupSuffix), t
}

// prune keeps the recent backups, then the newest of each day, and the
// one pinned whatever its age.
func (s *Store) prune() error {
	backups, err := s.backups()
	if err != nil {
		return err
	}
	pinned := s.pinned()
	days := map[string]bool{}
	for index, backup := range backups {
		day := backup.taken.Format("20060102")
		keep := index < recentBackups || (!days[day] && len(days) < dailyBackups) || filepath.Base(backup.path) == pinned
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
// notebook compatible with this build's schema.
func verifyFile(ctx context.Context, path string) error {
	db, err := sql.Open("sqlite", path+"?_pragma=query_only(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return err
	}
	defer db.Close()
	if err := check(ctx, db); err != nil {
		return err
	}
	var identity, version int64
	if err := db.QueryRowContext(ctx, `PRAGMA application_id`).Scan(&identity); err != nil {
		return err
	}
	if identity != applicationID {
		return ErrForeign
	}
	if err := db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version > int64(len(migrations)) {
		return fmt.Errorf("%w: backup schema %d is newer than this build's %d", ErrNewer, version, len(migrations))
	}
	return nil
}
