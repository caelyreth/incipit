package window

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/caelyreth/neverland/app/export"
	"github.com/caelyreth/neverland/app/library"
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/locales"
	"github.com/caelyreth/neverland/native/page"
	"github.com/caelyreth/neverland/native/shell"
	"github.com/caelyreth/neverland/native/theme"
)

// Export writes a sheet or a group out of the notebook as files (package
// export). The window only gathers what to write: each sheet's title,
// times and tags and a way to read its document, from its engine when it
// is open and else from the notebook, and each picture's file. The export
// window, modal to the main one, shows the plan of it, every file by the
// name it will have and what it holds, and writes exactly that, off the UI
// thread (export_view.go).

// exportJob is one export as its window shows it.
type exportJob struct {
	// name is what is exported, as the app names it; group tells a group
	// from a sheet.
	name  string
	group bool
	// sheet or folder is what to write: a group's when group is set.
	sheet  export.Sheet
	folder export.Folder
	sheets int
	// dir is the folder written into.
	dir string
	// plan is what will be written into dir, nil while it is read;
	// planned counts the plans asked for, so one made for an earlier dir
	// is dropped.
	plan    *export.Plan
	planErr error
	planned int
	// shown is the planned file the preview shows, by its path.
	shown string
	// written counts the files written while writing; writeErr is why
	// writing last stopped.
	writing  bool
	written  int
	writeErr error
	// pictures are the previews of pictures, read as they are shown.
	pictures *page.Pictures
}

// exportPreviewSide is the longest side a picture's preview is read at.
const exportPreviewSide = 720

// Export is part of shell.Library: it opens the export window for an
// object, "sheet:" or "group:" and its ID.
func (a *AppState) Export(id string) {
	if a.exporting != nil {
		if a.exportWindow != nil {
			a.exportWindow.Focus()
		}
		return
	}
	job, ok := a.exportJob(id)
	if !ok {
		return
	}
	job.dir = a.exportDir
	if job.dir == "" || !isDir(job.dir) {
		job.dir, _ = mygo.App.Path(mygo.PathDesktop)
	}
	a.exporting = job
	a.openExportWindow()
	a.planExport()
}

// exportJob is what exporting the object id writes.
func (a *AppState) exportJob(id string) (*exportJob, bool) {
	if name, ok := strings.CutPrefix(id, "sheet:"); ok {
		session, ok := a.sessionOf(name)
		if !ok {
			return nil, false
		}
		sheet := a.exportSheet(session)
		return &exportJob{name: sheet.Title, sheet: sheet, sheets: 1}, true
	}
	if name, ok := strings.CutPrefix(id, "group:"); ok {
		group, ok := a.Group(name)
		if !ok {
			return nil, false
		}
		folder, count := a.exportFolder(group)
		return &exportJob{name: folder.Name, group: true, folder: folder, sheets: count}, true
	}
	return nil, false
}

// exportFolder is a group to write, its groups within, and how many sheets
// it holds in all. A smart folder holds what its query finds.
func (a *AppState) exportFolder(group shell.GroupItem) (export.Folder, int) {
	folder := export.Folder{Name: group.Title()}
	count := 0
	for _, item := range a.Sheets(shell.Place{Kind: shell.PlaceGroup, Group: group.ID}) {
		if session, ok := a.sessionOf(item.ID); ok {
			folder.Sheets = append(folder.Sheets, a.exportSheet(session))
			count++
		}
	}
	if !group.Smart() {
		for _, sub := range a.library.Groups(groupOf(group.ID)) {
			inner, n := a.exportFolder(a.groupItem(sub))
			folder.Folders = append(folder.Folders, inner)
			count += n
		}
	}
	return folder, count
}

// exportSheet is a sheet to write: its title, times and tags, and its
// document read from the snapshot it has now if it is open, which the
// notebook may not hold yet, and else from the notebook.
func (a *AppState) exportSheet(session *sheetSession) export.Sheet {
	title := strings.TrimSpace(session.Title())
	if title == "" {
		title = locales.Untitled()
	}
	sheet := export.Sheet{Title: title, Tags: tagNames(session.summary.Tags)}
	if s, ok := a.library.Sheet(library.SheetID(session.key)); ok {
		sheet.Created, sheet.Updated = s.Created, s.Updated
	}
	switch {
	case session.engine != nil:
		snap := session.engine.Snapshot()
		sheet.Read = func() (*doc.Snapshot, error) { return snap, nil }
	case a.notebook != nil:
		store, key := a.notebook.store, session.key
		sheet.Read = func() (*doc.Snapshot, error) {
			return store.LoadPage(key, profile.Schema(), doc.NewInterner())
		}
	}
	return sheet
}

// exportPictures finds a picture's file in the notebook, saved under the
// name it came with.
func (a *AppState) exportPictures() export.Pictures {
	if a.notebook == nil {
		return nil
	}
	store := a.notebook.store
	return func(media string) (export.Picture, bool) {
		m, ok := store.Media(media)
		if !ok {
			return export.Picture{}, false
		}
		return export.Picture{Path: store.MediaFile(m), Name: m.FileName(), Type: m.Type, Width: m.Width, Height: m.Height}, true
	}
}

// planExport plans the export into its folder off the UI thread: the
// sheets are read and written out in memory, and the folder looked at for
// names taken. A plan for a folder since left is dropped.
func (a *AppState) planExport() {
	job := a.exporting
	if job == nil {
		return
	}
	job.planned++
	planned, dir, window := job.planned, job.dir, a.exportWindow
	job.plan, job.planErr = nil, nil
	pictures := a.exportPictures()
	go func() {
		var plan export.Plan
		var err error
		if job.group {
			plan, err = export.PlanFolder(dir, job.folder, pictures)
		} else {
			plan, err = export.PlanSheet(dir, job.sheet, pictures)
		}
		onUI(window, func() {
			if a.exporting != job || job.planned != planned {
				return
			}
			plan.FrontMatter = a.exportFrontMatter
			job.plan, job.planErr = &plan, err
			if !planHas(plan, job.shown) {
				job.shown = firstSheet(plan)
			}
			a.invalidateExport()
		})
	}()
}

// planHas reports whether a plan makes the file at rel.
func planHas(p export.Plan, rel string) bool {
	for _, f := range p.Files {
		if f.Path == rel {
			return true
		}
	}
	return false
}

// firstSheet is the first sheet's file a plan makes, which the preview
// shows first.
func firstSheet(p export.Plan) string {
	for _, f := range p.Files {
		if f.Kind == export.KindSheet {
			return f.Path
		}
	}
	return ""
}

// setExportFrontMatter turns front matter on or off for this export and
// those after it this session.
func (a *AppState) setExportFrontMatter(on bool) {
	a.exportFrontMatter = on
	if job := a.exporting; job != nil && job.plan != nil {
		job.plan.FrontMatter = on
	}
	a.invalidateExport()
}

// exportInto has the export written into dir, planned there anew.
func (a *AppState) exportInto(dir string) {
	job := a.exporting
	if job == nil || job.writing || dir == "" {
		return
	}
	job.dir, job.writeErr = dir, nil
	a.exportDir = dir
	a.planExport()
	a.invalidateExport()
}

// exportPlaces are the folders offered to export into: the Desktop,
// Documents and Downloads, and the last chosen this session where it is
// another.
func (a *AppState) exportPlaces() []string {
	var places []string
	for _, name := range []mygo.PathName{mygo.PathDesktop, mygo.PathDocuments, mygo.PathDownloads} {
		if dir, err := mygo.App.Path(name); err == nil && isDir(dir) {
			places = append(places, dir)
		}
	}
	if dir := a.exportDir; dir != "" && isDir(dir) && !containsPath(places, dir) {
		places = append(places, dir)
	}
	return places
}

func containsPath(paths []string, p string) bool {
	for _, q := range paths {
		if filepath.Clean(q) == filepath.Clean(p) {
			return true
		}
	}
	return false
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// openExportWindow shows the export window, modal to the main window, as
// a sheet of it on macOS.
func (a *AppState) openExportWindow() {
	scale := a.Shell.Settings.UIFontSize / theme.DefaultUIFontSize
	a.exporting.pictures = &page.Pictures{Read: a.readPicture, MaxSide: exportPreviewSide, Wake: a.invalidateExport}
	window := mygo.NewWindow(mygo.WindowOptions{
		Title:           locales.ExportTitle(a.exporting.name),
		Width:           int(880 * scale),
		Height:          int(640 * scale),
		MinWidth:        int(720 * scale),
		MinHeight:       int(520 * scale),
		Parent:          a.mainWindow,
		Modal:           true,
		Content:         ui.View(a.exportView),
		BackgroundColor: colorHex(a.Shell.Settings.Semantic().Surface.Background),
	})
	window.OnClosed(func() {
		a.exporting, a.exportWindow = nil, nil
	})
	a.exportWindow = window
}

// invalidateExport redraws the export window.
func (a *AppState) invalidateExport() {
	if w := a.exportWindow; w != nil && !w.IsDestroyed() {
		w.Invalidate()
	}
}

// closeExport closes the export window, unless it is writing.
func (a *AppState) closeExport() {
	if job := a.exporting; job != nil && job.writing {
		return
	}
	if a.exportWindow != nil {
		a.exportWindow.Close()
	}
}

// chooseExportDir asks for another folder to export into.
func (a *AppState) chooseExportDir() {
	job, window := a.exporting, a.exportWindow
	if job == nil || job.writing {
		return
	}
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: window, Title: locales.ChooseExportFolder(), DefaultPath: job.dir,
			Directory: true, CreateDirectories: true, ButtonLabel: locales.ChooseFolder(),
		})
		if err != nil || len(paths) == 0 {
			return
		}
		onUI(window, func() { a.exportInto(paths[0]) })
	}()
}

// runExport writes the plan off the UI thread, counting the files as they
// are written. Done, the window closes and a notice tells where the files
// are; failed, it stays open saying why, planned anew to try again.
func (a *AppState) runExport() {
	job, window := a.exporting, a.exportWindow
	if job == nil || job.writing || job.plan == nil || job.planErr != nil {
		return
	}
	plan := *job.plan
	job.writing, job.written, job.writeErr = true, 0, nil
	go func() {
		top, err := plan.Write(func(n int) {
			onUI(window, func() {
				job.written = n
				a.invalidateExport()
			})
		})
		onUI(window, func() {
			job.writing = false
			if err != nil {
				job.writeErr = err
				a.planExport()
				return
			}
			a.closeExport()
			a.Notices.Post(Notice{Kind: NoticeSuccess, Title: locales.Exported(job.name), Description: shownPath(top),
				Action: locales.ShowInFinder(), OnAction: func() { mygo.Shell.ShowItemInFolder(top) }})
		})
	}()
}

// shownPath is a path as people read it: the home folder as ~.
func shownPath(path string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if rest, ok := strings.CutPrefix(path, home); ok && (rest == "" || rest[0] == os.PathSeparator) {
			return "~" + rest
		}
	}
	return path
}
