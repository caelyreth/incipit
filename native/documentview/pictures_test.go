package documentview

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// readyPicture asks for media until it is read.
func readyPicture(t *testing.T, pictures *Pictures, media string, now time.Time) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, state := pictures.Picture(media, now); state == PictureReady {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s was never read", media)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// A window that draws nothing for long keeps the pictures it showed: the
// first frame after shows them at once, without reading them again; one
// the page stopped drawing while it drew others is let go.
func TestPicturesShownSurviveAnIdleWindow(t *testing.T) {
	data := redPicture(t)
	var reads atomic.Int32
	pictures := &Pictures{Read: func(string) ([]byte, error) {
		reads.Add(1)
		return data, nil
	}}
	start := time.Now()
	readyPicture(t, pictures, "shown", start)
	readyPicture(t, pictures, "left", start)
	// The page goes on drawing one of them for a while.
	for at := start; at.Before(start.Add(2 * pictureLinger)); at = at.Add(10 * time.Second) {
		pictures.Picture("shown", at)
	}
	// Then nothing is drawn for an hour, until the pointer moves.
	later := start.Add(time.Hour)
	if bitmap, state := pictures.Picture("shown", later); state != PictureReady || bitmap == (*ui.Bitmap)(nil) {
		t.Fatalf("the picture shown before the idle spell is %v after it", state)
	}
	if reads.Load() != 2 {
		t.Fatalf("pictures were read %d times, not once each", reads.Load())
	}
	if _, state := pictures.Picture("left", later); state != PictureLoading {
		t.Fatal("a picture the page stopped drawing was kept")
	}
}
