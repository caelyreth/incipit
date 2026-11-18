package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	// The formats a picture may come in, as MyGo draws them.
	_ "image/gif"
	_ "image/jpeg"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// The notebook's pictures are files in its Media folder, each named by the
// SHA-256 of its bytes and never changed once written, while the notebook
// keeps what each is (the media table) and pages refer to them by that
// hash. So a picture is written once however often it is used; a backup
// of the notebook stays whole without copying a picture, as every file it
// names stays as it was; and a page moving or renamed moves no file.
//
// A file takes its name only once it is complete and synced, and its row
// is written only after that, so the notebook never names a file that is
// not whole. A file left without a row, by a crash between the two, is
// removed when the notebook next opens.

const (
	mediaDir = "Media"
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
	kind, ok := MediaDecoderFormat(format)
	if !ok {
		return Media{}, ErrMediaFormat
	}
	if kind.NormalizePNG {
		if data, err = asPNG(data); err != nil {
			return Media{}, fmt.Errorf("%w: %v", ErrMediaFormat, err)
		}
		name = strings.TrimSuffix(name, filepath.Ext(name))
		if name != "" {
			name += "." + kind.StoredExtension
		}
	}
	sum := sha256.Sum256(data)
	media := Media{
		Hash:   hex.EncodeToString(sum[:]),
		Type:   kind.StoredMIME,
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

func asPNG(data []byte) ([]byte, error) {
	picture, _, err := image.Decode(bytes.NewReader(data))
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
	return filepath.Join(s.root, mediaDir, media.Hash[:2], media.Hash+"."+StoredMediaExtension(media.Type))
}

// writeMediaFile writes a picture under its name once it is whole.
func writeMediaFile(path string, data []byte) error {
	folder := filepath.Dir(path)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return err
	}
	return writePreparedFile(path, func(target io.Writer) error {
		_, err := target.Write(data)
		return err
	}, nil)
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

// sweepMedia removes the files the notebook has no row for: a crash
// between a file and its row, or a partial file.
func (s *Store) sweepMedia() error {
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
