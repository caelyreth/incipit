// Package export writes sheets out of the notebook as files people keep
// elsewhere: a sheet as a Markdown file, a group as a folder of them, its
// groups as folders within. It is a sink: structure flows into it and
// files out, and nothing it writes is read back. It knows no library,
// store, window or engine; the host hands it what to write, each sheet a
// way to read its document and each picture a file to copy, so the
// notebook's own shape never reaches the files but by what is given.
//
// An export is planned before it is written. A Plan is every file it will
// make, by the name it will have there, with its text or its source and
// its size: what a person is shown before they agree, and then exactly
// what Write makes. A sheet's pictures are copied into an assets folder
// beside its file, a picture used twice once, and its figures link there.
// Nothing at the destination is overwritten: a name taken there, or by a
// file planned before it, is given a number, as Finder does, and the file
// says the name it wanted.
//
// A plan with a Typeset writes each sheet as a PDF instead, typeset by the
// host, its pictures within it: no assets folder, and no front matter.
package export

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/caelyreth/neverland/internal/app/markdown"
	"github.com/caelyreth/neverland/internal/editor/doc"
)

// Sheet is one sheet to write: what to name its file, what its front
// matter says of it, and how to read its document.
type Sheet struct {
	// Title names its file and its front matter's title.
	Title            string
	Created, Updated time.Time
	Tags             []string
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

// Picture is a kept picture as a file: where it is, the name to save it
// under, and what it is.
type Picture struct {
	Path, Name    string
	Type          string
	Width, Height int
}

// Pictures finds the file of a picture a figure names by its media id.
type Pictures func(media string) (Picture, bool)

// Typeset makes a sheet's document into a PDF, titled title, and says how
// many pages it takes; snap is nil for a sheet that holds nothing.
type Typeset func(snap *doc.Snapshot, title string) (pdf []byte, pages int, err error)

// Kind is what a planned file is.
type Kind uint8

const (
	KindFolder Kind = iota + 1
	KindSheet
	KindPicture
)

// File is one file or folder a plan makes.
type File struct {
	Kind Kind
	// Path is where it goes, from the destination, its parts between
	// slashes; Depth is how many folders of the plan hold it.
	Path  string
	Depth int
	// Wanted is the name it would have had, had that not been taken; empty
	// when it has it.
	Wanted string
	// A sheet's front matter and body, written together as front matter
	// says (Plan.Text); a typeset sheet's body is what it says, shown
	// before it is written, and Typeset the file.
	Front, Body string
	Typeset     []byte
	Pages       int
	// A picture's media id, the file it copies, and what it is.
	Media   string
	Picture Picture
	Bytes   int64
	// A folder's sheets, pictures and groups, all it holds; Assets marks
	// the folder holding its sheets' pictures.
	Sheets, Pictures, Folders int
	Assets                    bool
}

// Name is the file's name, the last part of its path.
func (f File) Name() string { return path.Base(f.Path) }

// Plan is an export as it will be written.
type Plan struct {
	// Dir is the destination, a folder that is there.
	Dir string
	// Files are what is made there, in order: a folder before what it
	// holds; in a folder its sheets, then their assets folder and its
	// pictures, then its groups.
	Files []File
	// Top is the path of what the export makes at the destination for the
	// person to find: the sheet's file, or the group's folder.
	Top string
	// FrontMatter puts each sheet's front matter at the head of its file.
	FrontMatter bool
}

// Text is a sheet's file's text as the plan writes it.
func (p Plan) Text(f File) string {
	if p.FrontMatter && f.Front != "" {
		if f.Body == "" {
			return f.Front
		}
		return f.Front + "\n" + f.Body
	}
	return f.Body
}

// Size is how many bytes a file takes as the plan writes it.
func (p Plan) Size(f File) int64 {
	if f.Typeset != nil {
		return int64(len(f.Typeset))
	}
	if f.Kind == KindSheet {
		return int64(len(p.Text(f)))
	}
	return f.Bytes
}

// Totals counts what the plan writes: sheets, pictures and the folders of
// groups, and the bytes of its files.
func (p Plan) Totals() (sheets, pictures, groups int, bytes int64) {
	for _, f := range p.Files {
		switch {
		case f.Kind == KindSheet:
			sheets++
		case f.Kind == KindPicture:
			pictures++
		case f.Kind == KindFolder && !f.Assets:
			groups++
		}
		bytes += p.Size(f)
	}
	return
}

// assetsDir is the folder beside a sheet's file holding its pictures.
const assetsDir = "assets"

// PlanSheet plans a sheet written into dir as a Markdown file, its
// pictures in dir's assets folder, or as a PDF where typeset is set.
func PlanSheet(dir string, sheet Sheet, pictures Pictures, typeset Typeset) (Plan, error) {
	p := planner{dir: dir, pictures: pictures, typeset: typeset, taken: map[string]bool{}}
	err := p.folder("", 0, Folder{Sheets: []Sheet{sheet}})
	plan := Plan{Dir: dir, Files: p.files}
	if err == nil {
		plan.Top = p.files[0].Path
	}
	return plan, err
}

// PlanFolder plans a group written into dir as a folder of its own.
func PlanFolder(dir string, folder Folder, pictures Pictures, typeset Typeset) (Plan, error) {
	p := planner{dir: dir, pictures: pictures, typeset: typeset, taken: map[string]bool{}}
	err := p.group("", 0, folder)
	return Plan{Dir: dir, Files: p.files, Top: p.files[0].Path}, err
}

// planner gathers a plan's files, each name taken as it is given.
type planner struct {
	dir      string
	pictures Pictures
	typeset  Typeset
	files    []File
	// taken is every path planned, from the destination.
	taken map[string]bool
}

// group plans a folder for f in parent, and what it holds.
func (p *planner) group(parent string, depth int, f Folder) error {
	at := len(p.files)
	rel, wanted := p.free(parent, fileName(f.Name), "")
	p.files = append(p.files, File{Kind: KindFolder, Path: rel, Depth: depth, Wanted: wanted})
	if err := p.folder(rel, depth+1, f); err != nil {
		return err
	}
	for _, sub := range f.Folders {
		if err := p.group(rel, depth+1, sub); err != nil {
			return err
		}
	}
	for _, held := range p.files[at+1:] {
		switch held.Kind {
		case KindSheet:
			p.files[at].Sheets++
		case KindPicture:
			p.files[at].Pictures++
		case KindFolder:
			if !held.Assets {
				p.files[at].Folders++
			}
		}
	}
	return nil
}

// folder plans f's sheets in rel, then the assets folder they share
// and its pictures, each copied once however often it is used.
func (p *planner) folder(rel string, depth int, f Folder) error {
	var sheets, pictures []File
	assets := ""
	linked := map[string]string{}
	link := func(media string) string {
		if p.typeset != nil {
			return media // the picture is within the PDF
		}
		if to, ok := linked[media]; ok {
			return to
		}
		pic, ok := Picture{}, false
		if p.pictures != nil {
			pic, ok = p.pictures(media)
		}
		if !ok {
			return media
		}
		if assets == "" {
			assets, _ = p.free(rel, assetsDir, "")
		}
		ext := path.Ext(pic.Name)
		name, wanted := p.free(assets, fileName(strings.TrimSuffix(pic.Name, ext)), ext)
		var size int64
		if info, err := os.Stat(pic.Path); err == nil {
			size = info.Size()
		}
		pictures = append(pictures, File{Kind: KindPicture, Path: name, Depth: depth + 1, Wanted: wanted,
			Media: media, Picture: pic, Bytes: size})
		to := path.Base(assets) + "/" + path.Base(name)
		linked[media] = to
		return to
	}
	for _, sheet := range f.Sheets {
		var body string
		var snap *doc.Snapshot
		if sheet.Read != nil {
			var err error
			if snap, err = sheet.Read(); err != nil {
				return fmt.Errorf("%s: %w", sheet.Title, err)
			}
			if snap != nil {
				body = markdown.Document(snap.Root(), link)
			}
		}
		file := File{Kind: KindSheet, Depth: depth, Body: body}
		ext := ".md"
		if p.typeset != nil {
			out, pages, err := p.typeset(snap, sheet.Title)
			if err != nil {
				return fmt.Errorf("%s: %w", sheet.Title, err)
			}
			file.Typeset, file.Pages, ext = out, pages, ".pdf"
		} else {
			file.Front = frontMatter(sheet)
		}
		file.Path, file.Wanted = p.free(rel, fileName(sheet.Title), ext)
		sheets = append(sheets, file)
	}
	p.files = append(p.files, sheets...)
	if assets != "" {
		wanted := ""
		if path.Base(assets) != assetsDir {
			wanted = assetsDir
		}
		p.files = append(p.files, File{Kind: KindFolder, Path: assets, Depth: depth, Wanted: wanted, Pictures: len(pictures), Assets: true})
		p.files = append(p.files, pictures...)
	}
	return nil
}

// free is a path in parent for name and ext that nothing holds, at the
// destination or in the plan: the name, else the name and the first
// number from 2 that is free, with the name wanted.
func (p *planner) free(parent, name, ext string) (rel, wanted string) {
	join := func(n string) string {
		if parent == "" {
			return n + ext
		}
		return parent + "/" + n + ext
	}
	rel = join(name)
	for n := 2; p.taken[rel] || exists(filepath.Join(p.dir, filepath.FromSlash(rel))); n++ {
		wanted = name + ext
		rel = join(name + " " + strconv.Itoa(n))
	}
	p.taken[rel] = true
	return rel, wanted
}

// ErrTaken reports a file that came to be at the destination after the
// plan was made, which the plan does not overwrite.
var ErrTaken = errors.New("export: a file is already where the export would write")

// Write makes the plan's files, in order, telling done after each, and
// is the path of Top.
func (p Plan) Write(done func(n int)) (string, error) {
	for i, f := range p.Files {
		to := filepath.Join(p.Dir, filepath.FromSlash(f.Path))
		var err error
		switch f.Kind {
		case KindFolder:
			err = os.Mkdir(to, 0o755)
		case KindSheet:
			err = writeNew(to, func(w io.Writer) error {
				if f.Typeset != nil {
					_, err := w.Write(f.Typeset)
					return err
				}
				_, err := io.WriteString(w, p.Text(f))
				return err
			})
		case KindPicture:
			err = writeNew(to, func(w io.Writer) error {
				in, err := os.Open(f.Picture.Path)
				if err != nil {
					return err
				}
				defer in.Close()
				_, err = io.Copy(w, in)
				return err
			})
		}
		if errors.Is(err, os.ErrExist) {
			err = fmt.Errorf("%w: %s", ErrTaken, f.Path)
		}
		if err != nil {
			return "", err
		}
		if done != nil {
			done(i + 1)
		}
	}
	return filepath.Join(p.Dir, filepath.FromSlash(p.Top)), nil
}

// writeNew writes a file that is not there yet.
func writeNew(to string, write func(io.Writer) error) error {
	out, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if err := write(out); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// frontMatter is a sheet's front matter: YAML between dashed lines, its
// title, times and tags.
func frontMatter(s Sheet) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("title: " + yamlString(s.Title) + "\n")
	if !s.Created.IsZero() {
		b.WriteString("created: " + s.Created.Format(time.RFC3339) + "\n")
	}
	if !s.Updated.IsZero() {
		b.WriteString("updated: " + s.Updated.Format(time.RFC3339) + "\n")
	}
	if len(s.Tags) > 0 {
		b.WriteString("tags:\n")
		for _, t := range s.Tags {
			b.WriteString("  - " + yamlString(t) + "\n")
		}
	}
	b.WriteString("---\n")
	return b.String()
}

// yamlString is s as YAML reads it back: plain where it is plain words,
// else double-quoted.
func yamlString(s string) string {
	plain := s != "" && !strings.ContainsAny(s[:1], "-?:,[]{}#&*!|>'\"%@`~ ")
	for _, r := range s {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune(" _-./", r)) {
			plain = false
			break
		}
	}
	if plain && strings.TrimSpace(s) == s && !strings.Contains(s, ": ") {
		switch strings.ToLower(s) {
		case "true", "false", "yes", "no", "on", "off", "null", "~":
		default:
			if _, err := strconv.ParseFloat(s, 64); err != nil {
				return s
			}
		}
	}
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if unicode.IsControl(r) {
				fmt.Fprintf(&b, `\u%04x`, r)
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
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

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
