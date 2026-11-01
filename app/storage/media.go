package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	// The formats a picture may come in, as MyGo draws them.
	_ "image/gif"
	_ "image/jpeg"

	_ "golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// The notebook's pictures are files in its Media folder, each named by the
// SHA-256 of its bytes and never changed once written, while the notebook
// keeps what each is (the media table) and its pages name them by that
// hash. So a picture is written once however often it is used; a backup
// of the notebook stays whole without copying a picture, as every file it
// names stays as it was; and a page moving or renamed moves no file.
//
// A file takes its name only once it is complete and synced, and its row
// is written only after that, so the notebook never names a file that is
// not whole. A picture no page names any more is swept when the notebook
// next opens, but only once no backup names it either: undoing, or
// restoring a backup, always finds its pictures.

const (
	mediaDir = "Media"
	// MediaAttribute is the attribute through which a page's node names a
	// picture by its hash. Sweeping reads it from every page, the
	// backups' too, whatever the node's kind.
	MediaAttribute = "media"
	// maxMediaBytes and maxMediaPixels bound what a picture may be: one
	// past them would take more memory to show than a page should.
	maxMediaBytes  = 64 << 20
	maxMediaPixels = 64 << 20
)

var (
	// ErrMediaFormat reports bytes that are not a picture Neverland shows.
	ErrMediaFormat = errors.New("storage: not a picture Neverland can show")
	// ErrMediaSize reports a picture too large to keep.
	ErrMediaSize = errors.New("storage: the picture is too large")
)

// Media is one picture the notebook keeps.
type Media struct {
	// Hash is the hex SHA-256 of its bytes, which names it.
	Hash string
	// Type is its MIME type.
	Type string
	// Width and Height are its size in pixels.
	Width, Height int
	// Bytes is the file's size.
	Bytes int64
	// Name is the file name it came with, for saving it out again.
	Name  string
	Added time.Time
}

// mediaTypes are the formats kept as they come, by the name image.Decode
// gives each, with the extension their file takes.
var mediaTypes = map[string]struct{ mime, extension string }{
	"png":  {"image/png", "png"},
	"jpeg": {"image/jpeg", "jpg"},
	"gif":  {"image/gif", "gif"},
	"webp": {"image/webp", "webp"},
	"bmp":  {"image/bmp", "bmp"},
}

func mediaExtension(mime string) string {
	for _, kind := range mediaTypes {
		if kind.mime == mime {
			return kind.extension
		}
	}
	return "bin"
}

// AddMedia keeps a picture: written once, the same bytes adding nothing
// more. name is the file name it came with, if any. A TIFF, as the system
// copies some pictures, is kept as a PNG.
func (s *Store) AddMedia(data []byte, name string) (Media, error) {
	if len(data) > maxMediaBytes {
		return Media{}, ErrMediaSize
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return Media{}, fmt.Errorf("%w: %v", ErrMediaFormat, err)
	}
	if config.Width <= 0 || config.Height <= 0 {
		return Media{}, ErrMediaFormat
	}
	if config.Width*config.Height > maxMediaPixels {
		return Media{}, ErrMediaSize
	}
	if format == "tiff" {
		if data, err = tiffAsPNG(data); err != nil {
			return Media{}, fmt.Errorf("%w: %v", ErrMediaFormat, err)
		}
		format = "png"
		name = strings.TrimSuffix(name, filepath.Ext(name))
		if name != "" {
			name += ".png"
		}
	}
	kind, ok := mediaTypes[format]
	if !ok {
		return Media{}, ErrMediaFormat
	}
	sum := sha256.Sum256(data)
	media := Media{
		Hash:   hex.EncodeToString(sum[:]),
		Type:   kind.mime,
		Width:  config.Width,
		Height: config.Height,
		Bytes:  int64(len(data)),
		Name:   name,
		Added:  time.Now(),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Media{}, ErrClosed
	}
	if known, ok := s.media[media.Hash]; ok {
		if _, err := os.Stat(s.MediaFile(known)); err == nil {
			return known, nil
		}
	}
	if err := writeMediaFile(s.MediaFile(media), data); err != nil {
		return Media{}, err
	}
	if _, err := s.db.ExecContext(context.Background(), `
		INSERT INTO media (hash, type, width, height, bytes, name, added)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (hash) DO NOTHING`,
		media.Hash, media.Type, media.Width, media.Height, media.Bytes, media.Name,
		media.Added.UnixMilli(),
	); err != nil {
		return Media{}, err
	}
	if known, ok := s.media[media.Hash]; ok {
		return known, nil
	}
	s.media[media.Hash] = media
	return media, nil
}

func tiffAsPNG(data []byte) ([]byte, error) {
	picture, err := tiff.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, picture); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}

// Media is the picture a hash names, if the notebook keeps it.
func (s *Store) Media(hash string) (Media, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	media, ok := s.media[hash]
	return media, ok
}

// MediaFile is where a picture's file is: in a folder named by its hash's
// first two digits, so no one folder holds them all.
func (s *Store) MediaFile(media Media) string {
	if len(media.Hash) < 2 {
		return filepath.Join(s.root, mediaDir, media.Hash)
	}
	return filepath.Join(s.root, mediaDir, media.Hash[:2], media.Hash+"."+mediaExtension(media.Type))
}

// writeMediaFile writes a picture under its name once it is whole.
func writeMediaFile(path string, data []byte) error {
	folder := filepath.Dir(path)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return err
	}
	partial, err := os.CreateTemp(folder, ".*.partial")
	if err != nil {
		return err
	}
	fail := func(err error) error {
		_ = partial.Close()
		_ = os.Remove(partial.Name())
		return err
	}
	if _, err := partial.Write(data); err != nil {
		return fail(err)
	}
	if err := partial.Sync(); err != nil {
		return fail(err)
	}
	if err := partial.Close(); err != nil {
		_ = os.Remove(partial.Name())
		return err
	}
	if err := os.Rename(partial.Name(), path); err != nil {
		_ = os.Remove(partial.Name())
		return err
	}
	return syncDir(folder)
}

// loadMedia reads what the notebook keeps of its pictures.
func (s *Store) loadMedia(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT hash, type, width, height, bytes, name, added FROM media`)
	if err != nil {
		return err
	}
	defer rows.Close()
	s.media = map[string]Media{}
	for rows.Next() {
		var media Media
		var added int64
		if err := rows.Scan(&media.Hash, &media.Type, &media.Width, &media.Height,
			&media.Bytes, &media.Name, &added); err != nil {
			return err
		}
		media.Added = time.UnixMilli(added)
		s.media[media.Hash] = media
	}
	return rows.Err()
}

// sweepMedia lets go of the pictures nothing names: no page of the
// notebook, nor of any backup. A backup that cannot be read stops the
// sweep, as what it names is not known; files half written by a crash go
// whatever names them.
func (s *Store) sweepMedia(ctx context.Context) error {
	named, err := mediaNamed(ctx, s.db)
	if err != nil {
		return err
	}
	backups, err := s.backups()
	if err != nil {
		return err
	}
	for _, backup := range backups {
		db, err := sql.Open("sqlite", backup.path+"?_pragma=query_only(1)")
		if err != nil {
			return err
		}
		more, err := mediaNamed(ctx, db)
		_ = db.Close()
		if err != nil {
			return err
		}
		for hash := range more {
			named[hash] = true
		}
	}
	for hash := range s.media {
		if named[hash] {
			continue
		}
		if _, err := s.db.ExecContext(ctx, `DELETE FROM media WHERE hash = ?`, hash); err != nil {
			return err
		}
		if err := os.Remove(s.MediaFile(s.media[hash])); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		delete(s.media, hash)
	}
	// Files the notebook has no row for: a crash between a file and its
	// row, or a partial file.
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

// mediaNamed is every hash a node of a notebook's pages names.
func mediaNamed(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT attributes FROM nodes WHERE attributes LIKE ?`, `%"`+MediaAttribute+`"%`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	named := map[string]bool{}
	for rows.Next() {
		var encoded string
		if err := rows.Scan(&encoded); err != nil {
			return nil, err
		}
		var attributes map[string]storedValue
		if err := json.Unmarshal([]byte(encoded), &attributes); err != nil {
			return nil, err
		}
		if value, ok := attributes[MediaAttribute]; ok && value.Text != nil {
			named[*value.Text] = true
		}
	}
	return named, rows.Err()
}
