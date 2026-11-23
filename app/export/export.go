// Package export writes sheets out of the notebook as files people keep
// elsewhere: a sheet as a Markdown file, a group as a folder of them, its
// groups as folders within. It is a sink: structure flows into it and
// files out, and nothing it writes is read back. It knows no library,
// store, window or engine; the host hands it what to write, each sheet a
// way to read its document and each picture a file to copy, so the
// notebook's own shape never reaches the files but by what is given.
//
// A sheet's pictures are copied into an assets folder beside its file, and
// its figures link there. Nothing in the destination is overwritten: a
// name taken there is given a number, as Finder does.
package export

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/caelyreth/neverland/app/markdown"
	"github.com/caelyreth/neverland/editor/doc"
)

// Sheet is one sheet to write: what to name its file, and how to read its
// document.
type Sheet struct {
	// Title names its file.
	Title string
	// Read is its document now, or nil for a sheet that holds nothing.
	Read func() (*doc.Snapshot, error)
}

// Folder is a group to write: its name, its sheets in order, and its
// groups, each a folder of its own.
type Folder struct {
	Name    string
	Sheets  []Sheet
	Folders []Folder
}

// Picture is a kept picture as a file: where it is, and the name to save
// it under.
type Picture struct {
	Path, Name string
}

// Pictures finds the file of a picture a figure names by its media id.
type Pictures func(media string) (Picture, bool)

// Written is what an export wrote: where, and how much.
type Written struct {
	// Path is the file or the folder made at the destination.
	Path string
	// Sheets and Pictures count what was written.
	Sheets, Pictures int
}

// assetsDir is the folder beside a sheet's file holding its pictures.
const assetsDir = "assets"

// WriteSheet writes a sheet into dir as a Markdown file, its pictures in
// dir's assets folder.
func WriteSheet(dir string, sheet Sheet, pictures Pictures) (Written, error) {
	w := newFolderWriter(dir, pictures)
	path, err := w.sheet(sheet)
	return Written{Path: path, Sheets: 1, Pictures: w.copied}, err
}

// WriteFolder writes a group into dir as a folder of its own.
func WriteFolder(dir string, folder Folder, pictures Pictures) (Written, error) {
	var out Written
	path, err := writeFolder(dir, folder, pictures, &out)
	out.Path = path
	return out, err
}

func writeFolder(dir string, folder Folder, pictures Pictures, out *Written) (string, error) {
	path := free(dir, fileName(folder.Name), "")
	if err := os.Mkdir(path, 0o755); err != nil {
		return "", err
	}
	w := newFolderWriter(path, pictures)
	for _, sheet := range folder.Sheets {
		if _, err := w.sheet(sheet); err != nil {
			return path, err
		}
		out.Sheets++
	}
	out.Pictures += w.copied
	for _, sub := range folder.Folders {
		if _, err := writeFolder(path, sub, pictures, out); err != nil {
			return path, err
		}
	}
	return path, nil
}

// folderWriter writes sheets into one folder, sharing its assets: a
// picture used twice is copied once.
type folderWriter struct {
	dir      string
	pictures Pictures
	// linked is each picture's link, by media id, once copied.
	linked map[string]string
	copied int
}

func newFolderWriter(dir string, pictures Pictures) *folderWriter {
	return &folderWriter{dir: dir, pictures: pictures, linked: map[string]string{}}
}

// sheet writes one sheet and is the file it wrote.
func (w *folderWriter) sheet(sheet Sheet) (string, error) {
	var text string
	if sheet.Read != nil {
		snap, err := sheet.Read()
		if err != nil {
			return "", fmt.Errorf("%s: %w", sheet.Title, err)
		}
		if snap != nil {
			var failed error
			text = markdown.Document(snap.Root(), func(media string) string {
				link, err := w.picture(media)
				if err != nil && failed == nil {
					failed = err
				}
				return link
			})
			if failed != nil {
				return "", failed
			}
		}
	}
	path := free(w.dir, fileName(sheet.Title), ".md")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// picture copies a picture into the assets folder, once, and is its link
// from the folder's sheets. One the host cannot find keeps its media id.
func (w *folderWriter) picture(media string) (string, error) {
	if link, ok := w.linked[media]; ok {
		return link, nil
	}
	if w.pictures == nil {
		return media, nil
	}
	pic, ok := w.pictures(media)
	if !ok {
		return media, nil
	}
	assets := filepath.Join(w.dir, assetsDir)
	if err := os.MkdirAll(assets, 0o755); err != nil {
		return "", err
	}
	ext := filepath.Ext(pic.Name)
	path := free(assets, fileName(strings.TrimSuffix(pic.Name, ext)), ext)
	if err := copyFile(pic.Path, path); err != nil {
		return "", err
	}
	w.copied++
	link := assetsDir + "/" + filepath.Base(path)
	w.linked[media] = link
	return link, nil
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// fileName is a title as a file may be named on every system: without
// the characters one forbids or treats as a path, without leading dots
// or trailing spaces and dots, and not too long. The host names what has
// no title; one left empty here is "_".
func fileName(title string) string {
	name := strings.Map(func(r rune) rune {
		switch {
		case strings.ContainsRune(`/\:*?"<>|`, r), unicode.IsControl(r):
			return ' '
		}
		return r
	}, title)
	name = strings.Join(strings.Fields(name), " ")
	name = strings.TrimLeft(name, ".")
	name = strings.TrimRight(name, ". ")
	const most = 120
	if r := []rune(name); len(r) > most {
		name = strings.TrimRight(string(r[:most]), ". ")
	}
	if name == "" {
		name = "_"
	}
	return name
}

// free is a path in dir for name and ext that nothing holds: the name,
// else the name and the first number from 2 that is free.
func free(dir, name, ext string) string {
	path := filepath.Join(dir, name+ext)
	for n := 2; exists(path); n++ {
		path = filepath.Join(dir, name+" "+strconv.Itoa(n)+ext)
	}
	return path
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
