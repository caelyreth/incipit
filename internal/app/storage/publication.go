package storage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// publicationOps are the file-system edges of staged publication. Keeping
// them together lets failure injection exercise the sequence without
// changing the resource-specific writer, verifier, or cleanup policy.
type publicationOps struct {
	createTemp func(string, string) (*os.File, error)
	chmod      func(string, os.FileMode) error
	syncOpen   func(*os.File) error
	syncPath   func(string) error
	close      func(*os.File) error
	remove     func(string) error
	rename     func(string, string) error
	syncDir    func(string) error
}

func defaultPublicationOps() publicationOps {
	return publicationOps{
		createTemp: os.CreateTemp,
		chmod:      os.Chmod,
		syncOpen:   func(file *os.File) error { return file.Sync() },
		syncPath:   syncFile,
		close:      func(file *os.File) error { return file.Close() },
		remove:     os.Remove,
		rename:     replaceFile,
		syncDir:    syncDir,
	}
}

type preparedFile struct {
	target string
	temp   string
}

// writePreparedFile writes beside target, syncs and closes the staged file,
// verifies it when requested, then atomically gives it the target's name.
// The target and its mode are not touched until every prior step succeeds.
func writePreparedFile(
	target string,
	write func(io.Writer) error,
	verify func(string) error,
) error {
	return writePreparedFileWith(target, write, verify, defaultPublicationOps())
}

func writePreparedFileWith(
	target string,
	write func(io.Writer) error,
	verify func(string) error,
	ops publicationOps,
) error {
	folder := filepath.Dir(target)
	file, err := ops.createTemp(folder, "."+filepath.Base(target)+".*.partial")
	if err != nil {
		return err
	}
	prepared := preparedFile{target: target, temp: file.Name()}
	var operationErr error
	if err := preserveTargetMode(prepared.temp, target, ops); err != nil {
		operationErr = err
	} else {
		operationErr = write(file)
	}
	if operationErr == nil {
		operationErr = ops.syncOpen(file)
	}
	operationErr = errors.Join(operationErr, ops.close(file))
	if operationErr != nil {
		return prepared.discard(operationErr, ops)
	}
	return prepared.publish(verify, ops)
}

// completePreparedFile finishes a staged file produced by another owner, as
// SQLite's VACUUM INTO does for backups.
func completePreparedFile(target, temp string, verify func(string) error) error {
	return completePreparedFileWith(target, temp, verify, defaultPublicationOps())
}

func completePreparedFileWith(
	target string,
	temp string,
	verify func(string) error,
	ops publicationOps,
) error {
	prepared := preparedFile{target: target, temp: temp}
	if err := preserveTargetMode(temp, target, ops); err != nil {
		return prepared.discard(err, ops)
	}
	if err := ops.syncPath(temp); err != nil {
		return prepared.discard(err, ops)
	}
	return prepared.publish(verify, ops)
}

func (p preparedFile) publish(verify func(string) error, ops publicationOps) error {
	if verify != nil {
		if err := verify(p.temp); err != nil {
			return p.discard(err, ops)
		}
	}
	if err := ops.rename(p.temp, p.target); err != nil {
		return p.discard(err, ops)
	}
	if err := ops.syncDir(filepath.Dir(p.target)); err != nil {
		// The rename is already atomic and complete. There is no safe way to
		// restore the old destination after the new one is visible.
		return fmt.Errorf("storage: sync published file directory: %w", err)
	}
	return nil
}

func (p preparedFile) discard(cause error, ops publicationOps) error {
	if err := ops.remove(p.temp); err != nil && !errors.Is(err, os.ErrNotExist) {
		return errors.Join(cause, fmt.Errorf("storage: remove partial file %s: %w", p.temp, err))
	}
	return cause
}

// New staging files retain their original, umask-respecting mode; only an
// existing destination supplies permissions to copy to its replacement.
func preserveTargetMode(temp, target string, ops publicationOps) error {
	info, err := os.Stat(target)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return ops.chmod(temp, info.Mode().Perm())
}

// copyDurably copies a file in full and synced before it takes the
// target's name, so the target is never half written.
func copyDurably(from, to string) error {
	source, err := os.Open(from)
	if err != nil {
		return err
	}
	defer source.Close()
	return writePreparedFile(to, func(target io.Writer) error {
		_, err := io.Copy(target, source)
		return err
	}, nil)
}

func syncFile(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}
