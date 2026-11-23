package window

import (
	"os"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/caelyreth/neverland/app/export"
	"github.com/caelyreth/neverland/app/storage"
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/locales"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/shell"
	"github.com/caelyreth/neverland/native/theme"
)

// Export writes a sheet or a group out of the notebook as files (package
// export). The window only gathers what to write: each sheet's title and a
// way to read its document, from its engine when it is open and else from
// the notebook, and each picture's file. The export window, modal to the
// main one, says what will be written and where, and writes it there off
// the UI thread.

// exportJob is one export as its window shows it.
type exportJob struct {
	// name is what is exported, as the app names it.
	name string
	// sheet or folder is what to write: a group's when folder is set.
	sheet  export.Sheet
	folder *export.Folder
	count  int
	// dir is the folder written into, chosen in the window; it starts at
	// the last one chosen this session, else the Desktop.
	dir     string
	writing bool
}

// Export is part of shell.Library: it opens the export window for an
// object, "sheet:" or "group:" and its ID.
func (a *AppState) Export(id string) {
	job, ok := a.exportJob(id)
	if !ok || a.exporting != nil {
		return
	}
	job.dir = a.exportDir
	if job.dir == "" {
		job.dir, _ = mygo.App.Path(mygo.PathDesktop)
	}
	a.exporting = job
	a.openExportWindow()
}

// exportJob is what exporting the object id writes.
func (a *AppState) exportJob(id string) (*exportJob, bool) {
	if name, ok := strings.CutPrefix(id, "sheet:"); ok {
		session, ok := a.sessionOf(name)
		if !ok {
			return nil, false
		}
		sheet := a.exportSheet(session)
		return &exportJob{name: sheet.Title, sheet: sheet, count: 1}, true
	}
	if name, ok := strings.CutPrefix(id, "group:"); ok {
		group, ok := a.Group(name)
		if !ok {
			return nil, false
		}
		folder, count := a.exportFolder(group)
		return &exportJob{name: folder.Name, folder: &folder, count: count}, true
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

// exportSheet is a sheet to write: its document read from the snapshot it
// has now if it is open, which the notebook may not hold yet, and else
// from the notebook.
func (a *AppState) exportSheet(session *sheetSession) export.Sheet {
	title := strings.TrimSpace(session.Title())
	if title == "" {
		title = locales.Untitled()
	}
	if session.engine != nil {
		snap := session.engine.Snapshot()
		return export.Sheet{Title: title, Read: func() (*doc.Snapshot, error) { return snap, nil }}
	}
	if a.notebook == nil {
		return export.Sheet{Title: title}
	}
	store, key := a.notebook.store, session.key
	return export.Sheet{Title: title, Read: func() (*doc.Snapshot, error) {
		return store.LoadPage(key, profile.Schema(), doc.NewInterner())
	}}
}

// exportPictures finds a picture's file in the notebook, saved under the
// name it came with.
func exportPictures(store *storage.Store) export.Pictures {
	if store == nil {
		return nil
	}
	return func(media string) (export.Picture, bool) {
		m, ok := store.Media(media)
		if !ok {
			return export.Picture{}, false
		}
		return export.Picture{Path: store.MediaFile(m), Name: m.FileName()}, true
	}
}

// openExportWindow shows the export window, modal to the main window, as
// a sheet of it on macOS.
func (a *AppState) openExportWindow() {
	scale := a.Shell.Settings.UIFontSize / theme.DefaultUIFontSize
	window := mygo.NewWindow(mygo.WindowOptions{
		Title:           locales.ExportTitle(a.exporting.name),
		Width:           int(480 * scale),
		Height:          int(260 * scale),
		DisableResize:   true,
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

// closeExport closes the export window.
func (a *AppState) closeExport() {
	if a.exportWindow != nil {
		a.exportWindow.Close()
	}
}

// chooseExportDir asks for the folder to export into.
func (a *AppState) chooseExportDir() {
	job, window := a.exporting, a.exportWindow
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: window, Title: locales.ChooseExportFolder(), DefaultPath: job.dir,
			Directory: true, CreateDirectories: true, ButtonLabel: locales.ChooseEllipsis(),
		})
		if err != nil || len(paths) == 0 {
			return
		}
		onUI(window, func() {
			job.dir = paths[0]
			a.exportDir = paths[0]
			if window != nil {
				window.Invalidate()
			}
		})
	}()
}

// runExport writes the job off the UI thread, then closes the window and
// tells what was written, with a way to it in Finder.
func (a *AppState) runExport() {
	job, main := a.exporting, a.mainWindow
	if job == nil || job.writing || job.dir == "" {
		return
	}
	job.writing = true
	a.exportDir = job.dir
	var store *storage.Store
	if a.notebook != nil {
		store = a.notebook.store
	}
	pictures := exportPictures(store)
	go func() {
		var written export.Written
		var err error
		if job.folder != nil {
			written, err = export.WriteFolder(job.dir, *job.folder, pictures)
		} else {
			written, err = export.WriteSheet(job.dir, job.sheet, pictures)
		}
		onUI(main, func() {
			a.closeExport()
			if err != nil {
				a.fail(locales.CouldntExport(job.name), err)
				return
			}
			path := written.Path
			a.Notices.Post(Notice{Kind: NoticeSuccess, Title: locales.Exported(job.name),
				Action: locales.ShowInFinder(), OnAction: func() { mygo.Shell.ShowItemInFolder(path) }})
		})
	}()
}

// exportView is the export window: what will be written, in which
// format, where, and the buttons that cancel or write it.
func (a *AppState) exportView(c *ui.Context) {
	job := a.exporting
	if job == nil || a.Shell == nil {
		return
	}
	// Escape cancels and Return exports, as a sheet's buttons answer.
	if c.Shortcut(0, ui.KeyEscape) && !job.writing {
		a.closeExport()
	}
	if c.Shortcut(0, ui.KeyEnter) {
		a.runExport()
	}
	semantic := a.Shell.Settings.Semantic()
	scale := components.UIFontSize(c)
	pad := theme.Space24.DIPs(scale)
	summary := locales.ExportSheetSummary()
	if job.folder != nil {
		summary = locales.ExportGroupSummary(job.count)
	}
	ui.Column(c).WidthPercent(100).HeightPercent(100).Background(semantic.Surface.Background).
		Padding(pad, pad, pad, pad).Gap(theme.Space16.DIPs(scale)).Children(func() {
		ui.Column(c).Gap(theme.Space04.DIPs(scale)).Children(func() {
			components.Headline(c, locales.ExportTitle(job.name), theme.HeadlineSmall, semantic.Text.Default).
				Role(ui.RoleHeading)
			components.Label(c, summary, theme.TextSmall, semantic.Text.Muted)
		})
		components.SettingGroup{Key: "export-options", Label: locales.ExportTitle(job.name), Children: func() {
			components.SettingRow{Label: locales.ExportFormat(), Control: func() {
				components.Label(c, locales.ExportMarkdown(), theme.TextDefault, semantic.Text.Muted)
			}}.View(c, semantic)
			components.SettingRow{Label: locales.ExportDestination(), Description: shownPath(job.dir), Last: true, Control: func() {
				components.Button{Key: "choose", Label: locales.ChooseEllipsis(), Style: components.OutlinedButton,
					Size: components.CompactButton, Disabled: job.writing, Action: a.chooseExportDir}.View(c, semantic)
			}}.View(c, semantic)
		}}.View(c, semantic)
		ui.Box(c).Grow(1)
		ui.Row(c).Justify(ui.End).Gap(theme.Space08.DIPs(scale)).Children(func() {
			components.Button{Key: "cancel", Label: locales.Cancel(), Style: components.SubtleButton,
				Size: components.MediumButton, Disabled: job.writing, Action: a.closeExport}.View(c, semantic)
			components.Button{Key: "export", Label: locales.Export(), Style: components.PrimaryButton,
				Size: components.MediumButton, Disabled: job.writing || job.dir == "", Action: a.runExport}.View(c, semantic)
		})
	})
}

// shownPath is a folder as people read it: the home folder as ~.
func shownPath(path string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if rest, ok := strings.CutPrefix(path, home); ok && (rest == "" || rest[0] == os.PathSeparator) {
			return "~" + rest
		}
	}
	return path
}
