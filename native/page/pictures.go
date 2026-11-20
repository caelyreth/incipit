package page

import (
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/caelyreth/neverland/app/clipboard"
	"github.com/caelyreth/neverland/app/storage"
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

// Pictures reads and decodes the pictures figures show, off the UI thread,
// and keeps each while it shows: a figure asks for its picture as it is
// drawn, shows its place until the picture is ready, and is drawn again
// once it is. A picture no figure has drawn for a while lets its pixels
// go, to be decoded again should it show again.
type Pictures struct {
	// Read gives a picture's bytes by its media id.
	Read func(media string) ([]byte, error)
	// Wake asks for a frame once a picture is decoded.
	Wake func()

	mu      sync.Mutex
	entries map[string]*pictureEntry
	swept   time.Time
	// asked is when a picture was last asked for: the last frame with
	// pictures, before which the window may have drawn nothing.
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

// pictureLinger is how long a picture no figure draws keeps its pixels,
// counted in the time the page went on drawing others.
const pictureLinger = time.Minute

// Picture is the picture media names, as far as it has come; asking for
// one not yet read starts reading it. Without a reader, as on a showcase
// stage, every picture stays on its way.
func (p *Pictures) Picture(media string, now time.Time) (*ui.Bitmap, PictureState) {
	if p == nil || p.Read == nil {
		return nil, PictureLoading
	}
	if media == "" {
		return nil, PictureMissing
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.entries == nil {
		p.entries = map[string]*pictureEntry{}
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

// State is how far the picture media names has come, without asking for
// it: one not asked for, or let go, is on its way.
func (p *Pictures) State(media string) PictureState {
	if p == nil || p.Read == nil {
		return PictureLoading
	}
	if media == "" {
		return PictureMissing
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if entry, ok := p.entries[media]; ok {
		return entry.state
	}
	return PictureLoading
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
	wake := p.Wake
	p.mu.Unlock()
	if wake != nil {
		wake()
	}
}

// sweep lets go of the pictures not drawn for a while before the last one
// asked for, at most once a second; the caller holds the lock.
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

// Incoming is a picture the page is given, as a paste or a drop brings it:
// its bytes, or the file holding them, and the name it came with.
type Incoming struct {
	Data []byte
	Path string
	Name string
}

// Keep keeps the pictures a page is given, as the host does, off the UI
// thread, then gives then them kept, on the UI thread and in order: one it
// could not keep, which it says, has no media.
type Keep func(pictures []Incoming, then func(kept []profile.Picture))

// PlacePictures keeps pictures and places them as figures at at (nil:
// where the selection is then), as a drop, a paste or the host's picker
// gives them.
func (ed *Editor) PlacePictures(pictures []Incoming, at *doc.Caret) {
	ed.Keep(pictures, func(kept []profile.Picture) {
		kept = slices.DeleteFunc(kept, func(p profile.Picture) bool { return p.Media == "" })
		if len(kept) > 0 {
			ed.Do(profile.PlaceFigures{Pictures: kept, At: at}, ed.now())
		}
	})
}

// pastePictureText keeps the pictures pasted text names, then pastes it
// with each picture kept a figure.
func (ed *Editor) pastePictureText(lines []clipboard.Line, text string) {
	var pictures []Incoming
	for _, l := range lines {
		if l.Path != "" {
			pictures = append(pictures, Incoming{Path: l.Path, Name: filepath.Base(l.Path)})
		}
	}
	ed.Keep(pictures, func(kept []profile.Picture) {
		slice := clipboard.PictureSlice(ed.Engine.Program().Profile().Schema(), lines, kept)
		ed.Do(vocab.Paste{Slice: slice, Text: text}, ed.now())
	})
}

// pictureFormats are the transfers that bring pictures.
var pictureFormats = []transfer.Format{transfer.FileList, transfer.PNG}

// picturesOf reads the pictures data holds: its picture files, else its
// picture, unless it holds text too, which reads as the text it copied (a
// document's text often comes with a picture of itself).
func picturesOf(data transfer.Data, formats []transfer.Format) []Incoming {
	has := func(f transfer.Format) bool {
		for _, o := range formats {
			if o == f {
				return true
			}
		}
		return false
	}
	var out []Incoming
	if has(transfer.FileList) {
		files, _ := data.Files()
		for _, path := range files {
			if storage.AcceptsMediaFile(path) {
				out = append(out, Incoming{Path: path, Name: filepath.Base(path)})
			}
		}
		return out
	}
	if has(transfer.PNG) && !has(transfer.Text) {
		if b, err := data.Read(transfer.PNG); err == nil && len(b) > 0 {
			out = append(out, Incoming{Data: b})
		}
	}
	return out
}
