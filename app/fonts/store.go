package fonts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Store keeps downloaded faces in Dir, fetched from Source.
type Store struct {
	Dir string
	// Source is the repository's ofl folder at its pinned commit, Source
	// unless a test serves its own.
	Source string
	Client *http.Client
}

// NewStore is a store in dir, fetching from the pinned repository.
func NewStore(dir string) *Store {
	return &Store{Dir: dir, Source: Source, Client: http.DefaultClient}
}

// ErrMismatch is a file whose bytes are not the ones pinned.
var ErrMismatch = errors.New("fonts: a downloaded file is not the one pinned")

// path is where file is kept.
func (s *Store) path(file File) string {
	return filepath.Join(s.Dir, path.Base(file.Path))
}

// Installed is whether every file of f is kept, at its size: what it
// holds is verified as it is read.
func (s *Store) Installed(f Face) bool {
	for _, file := range f.Files {
		info, err := os.Stat(s.path(file))
		if err != nil || info.Size() != file.Size {
			return false
		}
	}
	return len(f.Files) > 0
}

// Read is the bytes of each of f's files, each verified; a file that is
// not the one pinned is removed and read as missing.
func (s *Store) Read(f Face) ([][]byte, error) {
	out := make([][]byte, 0, len(f.Files))
	for _, file := range f.Files {
		data, err := os.ReadFile(s.path(file))
		if err != nil {
			return nil, err
		}
		if !matches(data, file) {
			os.Remove(s.path(file))
			return nil, ErrMismatch
		}
		out = append(out, data)
	}
	return out, nil
}

// Install downloads f's files that are not kept yet, telling progress how
// many of its bytes have come of how many, and keeps each once it is
// verified. What has come of a file stays beside it, cancelled or failed,
// as the app quitting leaves it, and the next Install goes on from there.
func (s *Store) Install(ctx context.Context, f Face, progress func(done, total int64)) error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return err
	}
	total, done := f.Size(), int64(0)
	for _, file := range f.Files {
		if info, err := os.Stat(s.path(file)); err == nil && info.Size() == file.Size {
			done += file.Size
			continue
		}
		base := done
		err := s.fetch(ctx, file, func(n int64) {
			if progress != nil {
				progress(base+n, total)
			}
		})
		if err != nil {
			return err
		}
		done += file.Size
	}
	if progress != nil {
		progress(total, total)
	}
	return nil
}

// How a download meets a network that fails: how many times a file is
// asked for again, and how long it waits before the first, doubling; how
// long a connection may give nothing before it is given up.
var (
	attempts = 6
	backoff  = time.Second
	stall    = 30 * time.Second
)

// errStalled is a connection that gave nothing for stall.
var errStalled = errors.New("fonts: the download stalled")

// partial is where what has come of file is kept until it is whole.
func (s *Store) partial(file File) string { return s.path(file) + ".part" }

// fetch downloads file into its place, going on from what has come of it,
// asking again with a growing wait when the network fails, and verifying
// it whole; bytes that are not the pinned ones are let go and asked for
// once more from the start.
func (s *Store) fetch(ctx context.Context, file File, progress func(int64)) error {
	wait, fresh := backoff, false
	var err error
	for try := 0; try < attempts; try++ {
		if try > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
			wait *= 2
		}
		err = s.resume(ctx, file, progress)
		if errors.Is(err, ErrMismatch) && !fresh {
			// What had come may have been of a failed write: once, from
			// the start.
			os.Remove(s.partial(file))
			fresh, try, wait = true, -1, backoff
			continue
		}
		if err == nil || !transient(ctx, err) {
			break
		}
	}
	if errors.Is(err, ErrMismatch) {
		os.Remove(s.partial(file))
	}
	if err != nil {
		return err
	}
	return os.Rename(s.partial(file), s.path(file))
}

// resume asks for what has not come of file yet and appends it, verifying
// the file once it is whole.
func (s *Store) resume(ctx context.Context, file File, progress func(int64)) error {
	part, err := os.OpenFile(s.partial(file), os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	defer part.Close()
	info, err := part.Stat()
	if err != nil {
		return err
	}
	have := info.Size()
	if have > file.Size {
		have = 0
	}
	if have < file.Size {
		if have, err = s.download(ctx, part, have, file, progress); err != nil {
			return err
		}
	}
	if have != file.Size {
		return ErrMismatch
	}
	if _, err := part.Seek(0, io.SeekStart); err != nil {
		return err
	}
	h := sha256.New()
	if _, err := io.Copy(h, part); err != nil {
		return err
	}
	if hex.EncodeToString(h.Sum(nil)) != file.SHA256 {
		return ErrMismatch
	}
	return nil
}

// download asks for file from have on and writes what comes into part,
// reporting how much of it there is: a server that does not go on from
// have sends the whole, which replaces what was there.
func (s *Store) download(ctx context.Context, part *os.File, have int64, file File, progress func(int64)) (int64, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	url := strings.TrimSuffix(s.Source, "/") + "/" + escape(file.Path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return have, err
	}
	if have > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(have, 10)+"-")
	}
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return have, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusPartialContent && have > 0:
	case resp.StatusCode == http.StatusOK:
		have = 0
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable:
		// What was kept is not of this file: from the start.
		if err := part.Truncate(0); err != nil {
			return 0, err
		}
		return 0, &statusError{resp.StatusCode, resp.Status}
	default:
		return have, &statusError{resp.StatusCode, resp.Status}
	}
	if err := part.Truncate(have); err != nil {
		return have, err
	}
	if _, err := part.Seek(have, io.SeekStart); err != nil {
		return have, err
	}
	if progress != nil {
		progress(have)
	}
	// A connection that gives nothing for stall is given up, to be asked
	// for again; one that keeps giving may take as long as it needs.
	watch := time.AfterFunc(stall, func() { cancel(errStalled) })
	defer watch.Stop()
	buf := make([]byte, 64<<10)
	for have < file.Size {
		n, rerr := resp.Body.Read(buf[:min(int64(len(buf)), file.Size-have)])
		if n > 0 {
			watch.Reset(stall)
			if _, err := part.Write(buf[:n]); err != nil {
				return have, err
			}
			have += int64(n)
			if progress != nil {
				progress(have)
			}
		}
		// Stopped, it stops here, what came kept.
		if err := ctx.Err(); err != nil && !errors.Is(context.Cause(ctx), errStalled) {
			return have, err
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			if cause := context.Cause(ctx); errors.Is(cause, errStalled) {
				return have, errStalled
			}
			return have, rerr
		}
	}
	return have, nil
}

// statusError is a server's answer that is not the file.
type statusError struct {
	code   int
	status string
}

func (e *statusError) Error() string { return "fonts: the server answered " + e.status }

// transient is whether err may pass if the file is asked for again: the
// network failing, a stall, the server busy or failing, not the download
// cancelled or the file not there.
func transient(ctx context.Context, err error) bool {
	if ctx.Err() != nil || errors.Is(err, ErrMismatch) {
		return false
	}
	var se *statusError
	if errors.As(err, &se) {
		return se.code == http.StatusRequestedRangeNotSatisfiable || se.code == http.StatusTooManyRequests ||
			se.code == http.StatusRequestTimeout || se.code >= 500
	}
	var pe *os.PathError
	return !errors.As(err, &pe)
}

// Remove deletes f's files, and what had come of them.
func (s *Store) Remove(f Face) error {
	var errs []error
	for _, file := range f.Files {
		for _, p := range []string{s.path(file), s.partial(file)} {
			if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// Partial is how much of f has come, kept and in part, as a download left
// it.
func (s *Store) Partial(f Face) int64 {
	var n int64
	for _, file := range f.Files {
		if info, err := os.Stat(s.path(file)); err == nil && info.Size() == file.Size {
			n += file.Size
		} else if info, err := os.Stat(s.partial(file)); err == nil {
			n += min(info.Size(), file.Size)
		}
	}
	return n
}

func matches(data []byte, file File) bool {
	sum := sha256.Sum256(data)
	return int64(len(data)) == file.Size && hex.EncodeToString(sum[:]) == file.SHA256
}

// escape escapes the brackets and commas of variable fonts' file names.
func escape(p string) string {
	return strings.NewReplacer("[", "%5B", "]", "%5D", ",", "%2C").Replace(p)
}
