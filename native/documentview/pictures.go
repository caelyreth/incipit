package documentview

import (
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
)

// Pictures reads and decodes the pictures pages show, off the UI thread,
// and keeps each while it shows: a page asks for a picture as it draws
// it, shows its place until it is ready, and is drawn again once it is.
// A picture no page has drawn for a while lets its pixels go, to be
// decoded again should it show again.
type Pictures struct {
	// Read gives a picture's bytes by its media's hash.
	Read func(media string) ([]byte, error)

	mu      sync.Mutex
	entries map[string]*pictureEntry
	wake    func()
	swept   time.Time
	// asked is when a picture was last asked for: the page's last frame
	// with pictures, before which the window may have drawn nothing.
	asked time.Time
}

// PictureState is how far a picture is from showing.
type PictureState uint8

const (
	PictureLoading PictureState = iota
	PictureReady
	// PictureMissing is a picture that cannot show: its file gone, or not
	// one that decodes.
	PictureMissing
)

type pictureEntry struct {
	bitmap *ui.Bitmap
	state  PictureState
	drawn  time.Time
}

// pictureLinger is how long a picture no page draws keeps its pixels,
// counted in the time the page went on drawing others: a window that draws
// nothing while it waits keeps all it showed.
const pictureLinger = time.Minute

// Picture is the picture media names, as far as it has come; asking for
// one not yet read starts reading it.
func (p *Pictures) Picture(media string, now time.Time) (*ui.Bitmap, PictureState) {
	if p == nil || p.Read == nil || media == "" {
		return nil, PictureMissing
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.entries == nil {
		p.entries = make(map[string]*pictureEntry)
	}
	p.sweep(now)
	p.asked = now
	entry, ok := p.entries[media]
	if !ok {
		entry = &pictureEntry{state: PictureLoading}
		p.entries[media] = entry
		go p.decode(media, entry)
	}
	entry.drawn = now
	return entry.bitmap, entry.state
}

// SetWake gives the redraw a decoded picture asks for, as the window's.
func (p *Pictures) SetWake(wake func()) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.wake = wake
	p.mu.Unlock()
}

func (p *Pictures) decode(media string, entry *pictureEntry) {
	data, err := p.Read(media)
	var bitmap *ui.Bitmap
	if err == nil {
		bitmap, err = ui.DecodeBitmap(data)
	}
	p.mu.Lock()
	entry.bitmap, entry.state = bitmap, PictureReady
	if err != nil {
		entry.bitmap, entry.state = nil, PictureMissing
	}
	wake := p.wake
	p.mu.Unlock()
	if wake != nil {
		wake()
	}
}

// sweep lets go of the pictures not drawn for a while before the last
// one asked for, at most once a second; the caller holds the lock. It
// measures from that ask, not from now: after an idle spell, the first
// frame asks again for what it showed before anything counts as unseen.
func (p *Pictures) sweep(now time.Time) {
	if now.Sub(p.swept) < time.Second {
		return
	}
	p.swept = now
	for media, entry := range p.entries {
		if entry.state != PictureLoading && p.asked.Sub(entry.drawn) > pictureLinger {
			delete(p.entries, media)
		}
	}
}
