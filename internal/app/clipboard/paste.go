package clipboard

import (
	"strings"

	"github.com/caelyreth/neverland/internal/app/html"
	"github.com/caelyreth/neverland/internal/app/markdown"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

// The clipboard formats a paste reads besides Format, as the system names
// them.
const (
	FormatHTML     = "text/html"
	FormatMarkdown = "text/markdown"
	FormatText     = "text/plain"
)

// PasteFormats are the formats a paste reads, in the order it prefers them.
var PasteFormats = []string{Format, FormatHTML, FormatMarkdown, FormatText}

// Clip is what a paste reads from the clipboard: the paste, and the local
// pictures its Markdown names on lines of their own, which the host keeps
// before pasting (Kept).
type Clip struct {
	Paste    vocab.Paste
	Pictures []markdown.LocalPicture
	text     string
}

// Read reads a paste from what the clipboard holds, each representation
// parsed once: Neverland's slice, else rich HTML, else a table of tabs,
// else Markdown (the clipboard's own, else its text), else the text as
// it is. Local pictures are found only in Markdown text, where accept
// takes their files; nil finds none.
func Read(read func(format string) []byte, accept func(path string) bool) Clip {
	if raw := read(Format); len(raw) > 0 {
		if slice, err := Decode(raw); err == nil {
			return Clip{Paste: vocab.Paste{Slice: slice, Text: vocab.PlainText(profile.Schema(), slice)}}
		}
	}
	text := string(read(FormatText))
	if raw := read(FormatHTML); len(raw) > 0 {
		if slice, rich := html.Parse(raw); rich && !slice.Empty() {
			if text == "" {
				text = vocab.PlainText(profile.Schema(), slice)
			}
			return Clip{Paste: vocab.Paste{Slice: slice, Text: text}}
		}
	}
	if strings.Contains(text, "\t") {
		if slice, ok := TableText(text); ok {
			return Clip{Paste: vocab.Paste{Slice: slice, Text: text}}
		}
	}
	source := text
	if md := string(read(FormatMarkdown)); md != "" {
		source = md
	}
	if accept != nil {
		if pics := markdown.LocalPictures(source, accept); len(pics) > 0 {
			return Clip{Paste: vocab.Paste{Text: text}, Pictures: pics, text: source}
		}
	}
	if slice, ok := markdown.Parse(source); ok && !slice.Empty() {
		return Clip{Paste: vocab.Paste{Slice: slice, Text: text}}
	}
	return Clip{Paste: vocab.Paste{Text: text}}
}

// Kept is the paste once the host kept the clip's pictures, kept[i] for
// Pictures[i] (no media where it could not): each kept picture a figure,
// one not kept left as its Markdown.
func (c Clip) Kept(kept []profile.Picture) vocab.Paste {
	used := make([]bool, len(c.Pictures))
	slice, ok := markdown.ParseWith(c.text, markdown.Options{
		Picture: func(source string) (profile.Picture, bool) {
			for i, p := range c.Pictures {
				if !used[i] && p.Source == source && i < len(kept) && kept[i].Media != "" {
					used[i] = true
					return kept[i], true
				}
			}
			return profile.Picture{}, false
		},
	})
	if ok && !slice.Empty() {
		return vocab.Paste{Slice: slice, Text: c.Paste.Text}
	}
	return c.Paste
}
