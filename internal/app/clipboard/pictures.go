package clipboard

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

// Line is one line of text another application copied: its text, and the
// picture file it names when it is a Markdown picture, ![alt](source),
// standing alone. A picture's alt text is its figure's caption, which
// assistive technology reads as the picture's name.
type Line struct {
	Text string
	// Path is the picture's file, if the line is one.
	Path string
	Alt  string
}

// PictureText reads text that names pictures in Markdown, when it does:
// its lines, each a line of text or a picture whose source is a file on
// this computer, a path from the root or the home folder or a file URL,
// that accept allows. A picture on the web stays text: pasting asks no
// server for anything.
func PictureText(text string, accept func(path string) bool) ([]Line, bool) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	out := make([]Line, len(lines))
	found := false
	for i, l := range lines {
		out[i].Text = l
		if alt, source, ok := markdownPicture(strings.TrimSpace(l)); ok {
			if path, ok := localFile(source); ok && accept != nil && accept(path) {
				out[i].Path, out[i].Alt, found = path, alt, true
			}
		}
	}
	return out, found
}

// markdownPicture reads ![alt](source) and ![alt](source "title"), the
// source between angle brackets when it has spaces.
func markdownPicture(l string) (alt, source string, ok bool) {
	rest, ok := strings.CutPrefix(l, "![")
	if !ok {
		return "", "", false
	}
	end := strings.LastIndex(rest, "](")
	if end < 0 || !strings.HasSuffix(rest, ")") {
		return "", "", false
	}
	alt, inner := rest[:end], strings.TrimSpace(rest[end+2:len(rest)-1])
	if strings.HasPrefix(inner, "<") {
		close := strings.Index(inner, ">")
		if close < 0 {
			return "", "", false
		}
		source, inner = inner[1:close], strings.TrimSpace(inner[close+1:])
	} else {
		source, inner, _ = strings.Cut(inner, " ")
		inner = strings.TrimSpace(inner)
	}
	if inner != "" && !(len(inner) >= 2 && strings.ContainsRune(`"'`, rune(inner[0])) && inner[len(inner)-1] == inner[0]) {
		return "", "", false
	}
	return strings.TrimSpace(alt), source, source != ""
}

// localFile is the file source names on this computer, if it names one.
func localFile(source string) (string, bool) {
	switch {
	case strings.HasPrefix(source, "file://"):
		u, err := url.Parse(source)
		if err != nil || (u.Host != "" && u.Host != "localhost") {
			return "", false
		}
		return filepath.FromSlash(u.Path), u.Path != ""
	case strings.HasPrefix(source, "~/"):
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		return filepath.Join(home, source[2:]), true
	case filepath.IsAbs(source):
		return source, true
	}
	return "", false
}

// PictureSlice is lines as a slice, each picture kept (by lines' order)
// a figure captioned by its alt text, and each other line a paragraph: a
// picture not kept stays the text it was.
func PictureSlice(s *doc.Schema, lines []Line, kept []profile.Picture) vocab.Slice {
	var nodes []*doc.Node
	k := 0
	for _, l := range lines {
		if l.Path != "" {
			pic := kept[k]
			k++
			if pic.Media != "" {
				nodes = append(nodes, profile.NewFigure(s, pic, l.Alt))
				continue
			}
		}
		var text []doc.Item
		if l.Text != "" {
			text = append(text, doc.TextItem(l.Text, nil))
		}
		nodes = append(nodes, vocab.NewBlock(s, s.Plain(), doc.Attrs{}, text...))
	}
	slice := vocab.Slice{Nodes: nodes}
	if nodes[0].Kind() == s.Plain() {
		slice.OpenStart = 1
	}
	if nodes[len(nodes)-1].Kind() == s.Plain() {
		slice.OpenEnd = 1
	}
	return slice
}
