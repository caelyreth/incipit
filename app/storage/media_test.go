package storage

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/model"
	"golang.org/x/image/tiff"
)

func picture(t *testing.T, width, height int, shade uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := range width {
		img.Set(x, 0, color.RGBA{R: shade, A: 255})
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

// naming makes a page whose one block names a picture.
func naming(t *testing.T, hash string) *model.Document {
	t.Helper()
	document := page(t, "")
	edit(t, document, func(draft *model.DocumentDraft) error {
		return draft.SetAttribute(block(document, 0), MediaAttribute, model.TextValue(hash))
	})
	return document
}

func TestTheMediaAttributeIsTheEditors(t *testing.T) {
	if MediaAttribute != editor.MediaAttribute {
		t.Fatalf("storage sweeps %q, the editor names pictures by %q", MediaAttribute, editor.MediaAttribute)
	}
}

func TestAPictureIsKeptOnceUnderItsHash(t *testing.T) {
	store := openStore(t, t.TempDir())
	defer store.Close()
	data := picture(t, 3, 2, 10)
	first, err := store.AddMedia(data, "shot.png")
	if err != nil {
		t.Fatal(err)
	}
	if first.Width != 3 || first.Height != 2 || first.Type != "image/png" || first.Name != "shot.png" {
		t.Fatalf("kept %+v", first)
	}
	again, err := store.AddMedia(data, "copy.png")
	if err != nil {
		t.Fatal(err)
	}
	if again != first {
		t.Fatalf("the same bytes were kept again as %+v", again)
	}
	written, err := os.ReadFile(store.MediaFile(first))
	if err != nil || !bytes.Equal(written, data) {
		t.Fatalf("the file holds %d bytes (%v), want the picture's %d", len(written), err, len(data))
	}
	if filepath.Base(filepath.Dir(store.MediaFile(first))) != first.Hash[:2] {
		t.Fatalf("filed at %s", store.MediaFile(first))
	}
	if _, err := store.AddMedia([]byte("not a picture"), ""); !errors.Is(err, ErrMediaFormat) {
		t.Fatalf("text was taken as a picture: %v", err)
	}
}

func TestATIFFIsKeptAsAPNG(t *testing.T) {
	store := openStore(t, t.TempDir())
	defer store.Close()
	var encoded bytes.Buffer
	if err := tiff.Encode(&encoded, image.NewGray(image.Rect(0, 0, 4, 4)), nil); err != nil {
		t.Fatal(err)
	}
	media, err := store.AddMedia(encoded.Bytes(), "scan.tiff")
	if err != nil {
		t.Fatal(err)
	}
	if media.Type != "image/png" || media.Name != "scan.png" || filepath.Ext(store.MediaFile(media)) != ".png" {
		t.Fatalf("kept %+v at %s", media, store.MediaFile(media))
	}
	written, err := os.ReadFile(store.MediaFile(media))
	if err != nil {
		t.Fatal(err)
	}
	if _, format, err := image.Decode(bytes.NewReader(written)); err != nil || format != "png" {
		t.Fatalf("stored format = %q, error = %v; want PNG", format, err)
	}
}

func TestPicturesNoPageNamesAreSweptAtTheNextLaunch(t *testing.T) {
	root := t.TempDir()
	store := openStore(t, root)
	kept, err := store.AddMedia(picture(t, 2, 2, 1), "")
	if err != nil {
		t.Fatal(err)
	}
	dropped, err := store.AddMedia(picture(t, 2, 2, 2), "")
	if err != nil {
		t.Fatal(err)
	}
	saver := store.Autosave(nil)
	saver.Page(1, "", naming(t, kept.Hash).Snapshot())
	if err := saver.Close(); err != nil {
		t.Fatal(err)
	}
	// What a crash leaves: a half-written file, and a whole one whose row
	// was never written.
	stray := filepath.Join(filepath.Dir(store.MediaFile(kept)), ".1.partial")
	if err := os.WriteFile(stray, []byte("half"), 0o644); err != nil {
		t.Fatal(err)
	}
	orphan := filepath.Join(root, mediaDir, "ff", "ff00.png")
	if err := os.MkdirAll(filepath.Dir(orphan), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(orphan, picture(t, 1, 1, 3), 0o644); err != nil {
		t.Fatal(err)
	}
	store.Close()

	store = openStore(t, root)
	defer store.Close()
	if _, ok := store.Media(kept.Hash); !ok {
		t.Fatal("the picture a page names was swept")
	}
	if _, err := os.Stat(store.MediaFile(kept)); err != nil {
		t.Fatalf("the named picture's file went: %v", err)
	}
	if _, ok := store.Media(dropped.Hash); ok {
		t.Fatal("a picture nothing names was kept")
	}
	for _, path := range []string{store.MediaFile(dropped), stray, orphan} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s is still there (%v)", path, err)
		}
	}
}

func TestABackupKeepsThePicturesItNames(t *testing.T) {
	root := t.TempDir()
	store := openStore(t, root)
	media, err := store.AddMedia(picture(t, 2, 2, 4), "")
	if err != nil {
		t.Fatal(err)
	}
	saver := store.Autosave(nil)
	saver.Page(1, "", naming(t, media.Hash).Snapshot())
	if err := saver.Flush(); err != nil {
		t.Fatal(err)
	}
	// An hour back, so the closing backup does not take its name.
	if err := store.backup(context.Background(), time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	// The page lets go of it, but the backup still names it.
	saver.Page(1, "", page(t, "").Snapshot())
	if err := saver.Close(); err != nil {
		t.Fatal(err)
	}
	store.Close()

	store = openStore(t, root)
	defer store.Close()
	if _, ok := store.Media(media.Hash); !ok {
		t.Fatal("a picture a backup names was swept")
	}
	if _, err := os.Stat(store.MediaFile(media)); err != nil {
		t.Fatalf("its file went: %v", err)
	}
}
