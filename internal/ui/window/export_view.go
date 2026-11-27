package window

import (
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/caelyreth/incipit/internal/app/export"
	"github.com/caelyreth/incipit/internal/ui/components"
	"github.com/caelyreth/incipit/internal/ui/page"
	"github.com/caelyreth/incipit/internal/ui/theme"
	"github.com/caelyreth/incipit/locales"
)

// The export window shows what an export will write before it is written,
// as it will be: a header naming it, its format and what it adds up to;
// the files, every folder, sheet and picture by the name it will have in
// the folder chosen, which heads the list and is chosen there; the
// preview of the file chosen in the list, a sheet's text line by line as
// it will be written, front matter included, a picture as it is, a folder
// by what it holds; the options, which change the preview as they change;
// and a foot saying where it goes, how far writing has come, or why it
// stopped, beside Cancel and Export.

// exportPreviewLines is how many lines of a sheet the preview shows.
const exportPreviewLines = 400

// exportMetrics are the export window's sizes at its UI size.
type exportMetrics struct {
	k, scale             float32
	pad, gap, row, inset float32
	radius, inner        float32
	// icon heads a row, inline stands beside small words.
	icon, inline float32
}

func newExportMetrics(c *ui.Context) exportMetrics {
	scale := components.UIFontSize(c)
	k := scale / theme.DefaultUIFontSize
	controls := theme.DefaultControls.Scaled(scale)
	return exportMetrics{
		k: k, scale: scale,
		pad: theme.Space24.DIPs(scale), gap: theme.Space12.DIPs(scale),
		row: controls.Height, inset: theme.Space12.DIPs(scale),
		radius: controls.PanelRadius, inner: controls.InnerRadius,
		icon: controls.RowIcon, inline: controls.InlineIcon,
	}
}

// exportView is the export window.
func (a *AppState) exportView(c *ui.Context) {
	job := a.exporting
	if job == nil || a.Shell == nil {
		return
	}
	// A window of its own, it takes the app's theme as the main window's
	// view does: its UI size, its face and its colors, through a change.
	semantic, changing := a.colors.At(a.Shell.Settings, c.Now())
	c.SetTheme(a.Shell.Settings.MyGoThemeFor(semantic))
	components.Menus(c, semantic, a.Shell.Settings.DrawnMenus)
	if changing {
		c.AnimationFrame()
	}
	a.exportKeys(c, job)
	m := newExportMetrics(c)
	ui.Column(c).WidthPercent(100).HeightPercent(100).Background(semantic.Surface.Background).
		Padding(m.pad, m.pad, m.pad, m.pad).Gap(theme.Space16.DIPs(m.scale)).Children(func() {
		a.exportHeader(c, semantic, m, job)
		ui.Row(c).Grow(1).MinHeight(0).Gap(m.gap).AlignItems(ui.Stretch).Children(func() {
			a.exportFiles(c, semantic, m, job)
			ui.Column(c).Grow(1).MinWidth(0).Gap(m.gap).Children(func() {
				a.exportPreview(c, semantic, m, job)
				a.exportOptions(c, semantic, job)
			})
		})
		a.exportFoot(c, semantic, m, job)
	})
}

// exportKeys answers the window's keys: Escape cancels, Return exports,
// and the arrows move the preview through the files.
func (a *AppState) exportKeys(c *ui.Context, job *exportJob) {
	if c.Shortcut(0, ui.KeyEscape) || (runtime.GOOS == "darwin" && c.Shortcut(ui.Cmd, ui.KeyPeriod)) {
		a.closeExport()
	}
	if c.Shortcut(0, ui.KeyEnter) {
		a.runExport()
	}
	if job.plan == nil || len(job.plan.Files) == 0 {
		return
	}
	step := 0
	if c.Shortcut(0, ui.KeyDown) {
		step = 1
	}
	if c.Shortcut(0, ui.KeyUp) {
		step = -1
	}
	if step == 0 {
		return
	}
	at := 0
	for i, f := range job.plan.Files {
		if f.Path == job.shown {
			at = i
		}
	}
	at = min(max(at+step, 0), len(job.plan.Files)-1)
	job.shown = job.plan.Files[at].Path
}

// exportHeader names what is exported: its icon, its name, and the format
// and what the export adds up to.
func (a *AppState) exportHeader(c *ui.Context, semantic theme.Theme, m exportMetrics, job *exportJob) {
	icon := components.IconFileText
	if job.group {
		icon = components.IconFolder
	}
	tile := 40 * m.k
	ui.Row(c).WidthPercent(100).MinWidth(0).Gap(m.gap).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(tile, tile).Shrink(0).Radius(10*m.k).Background(semantic.Accent.Hint).
			Border(1, semantic.Border.Variant).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			ui.Icon(c, icon.SVG()).FontSize(20 * m.k).TextColor(semantic.Icon.Accent)
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(theme.Space02.DIPs(m.scale)).Children(func() {
			components.Headline(c, locales.ExportTitle(job.name), theme.HeadlineSmall, semantic.Text.Heading).
				SingleLine().Role(ui.RoleHeading)
			components.Label(c, a.exportSummary(job), theme.TextSmall, semantic.Text.Muted).
				SingleLine().FontFeatures("tnum")
		})
	})
}

// exportSummary says the export's format and what it adds up to, or that
// it is being read.
func (a *AppState) exportSummary(job *exportJob) string {
	format := locales.ExportMarkdown()
	if a.exportPDF {
		format = locales.Pdf()
	}
	parts := []string{format}
	if job.plan == nil {
		return strings.Join(append(parts, locales.ExportReading(job.sheets)), " · ")
	}
	sheets, pictures, groups, bytes := job.plan.Totals()
	parts = append(parts, locales.SheetsCount(sheets))
	if pictures > 0 {
		parts = append(parts, locales.PicturesCount(pictures))
	}
	// A group's own folder is the export; those within it are its groups.
	if groups > 1 {
		parts = append(parts, locales.GroupsCount(groups-1))
	}
	return strings.Join(append(parts, components.Bytes(bytes)), " · ")
}

// panel is a bordered surface holding a part of the window.
func panel(c *ui.Context, semantic theme.Theme, m exportMetrics, key string) ui.Element {
	return ui.Column(c.Key(key)).MinWidth(0).MinHeight(0).Radius(m.radius).
		Border(1, semantic.Border.Variant).Background(semantic.Surface.Page).Clip()
}

// panelHead is a panel's heading row, what follows it under a line.
func panelHead(c *ui.Context, semantic theme.Theme, m exportMetrics, children func()) {
	ui.Row(c).WidthPercent(100).MinHeight(40 * m.k).Shrink(0).Gap(theme.Space08.DIPs(m.scale)).
		PaddingX(m.inset).AlignItems(ui.Center).Children(children)
	components.HorizontalDivider(c, semantic, components.VariantDivider)
}

// exportFiles lists what will be written, under the folder it goes into,
// which is chosen at its head.
func (a *AppState) exportFiles(c *ui.Context, semantic theme.Theme, m exportMetrics, job *exportJob) {
	panel(c, semantic, m, "export-files").Width(320 * m.k).Shrink(0).Children(func() {
		panelHead(c, semantic, m, func() {
			components.Label(c, locales.ExportFiles(), theme.TextSmall, semantic.Text.Muted).Grow(1).MinWidth(0).SingleLine()
			a.exportPlace(c, semantic, m, job)
		})
		ui.Scroll(c.Key("export-tree")).Grow(1).MinHeight(0).WidthPercent(100).Children(func() {
			ui.Column(c).WidthPercent(100).Padding(theme.Space06.DIPs(m.scale), theme.Space06.DIPs(m.scale),
				theme.Space06.DIPs(m.scale), theme.Space06.DIPs(m.scale)).Children(func() {
				a.exportTree(c, semantic, m, job)
			})
		})
	})
}

// exportPlace is the folder the export goes into, as a button that offers
// the usual places and any other.
func (a *AppState) exportPlace(c *ui.Context, semantic theme.Theme, m exportMetrics, job *exportJob) {
	button := ui.ButtonBase(c.Key("export-place")).Label(locales.ExportPlace()).Disabled(job.writing).
		Height(theme.DefaultControls.Scaled(m.scale).InlineHeight).PaddingStart(theme.Space08.DIPs(m.scale)).PaddingEnd(theme.Space06.DIPs(m.scale)).Gap(theme.Space06.DIPs(m.scale)).Radius(m.inner).
		AlignItems(ui.Center).FocusRing(true).Shrink(1).MinWidth(0)
	if job.writing {
		button.Opacity(components.DisabledOpacity)
	} else if button.Hovered() {
		button.Background(semantic.Element.Hover)
	}
	button.Children(func() {
		ui.Icon(c, components.IconFolder.SVG()).FontSize(m.inline).TextColor(semantic.Icon.Muted).Shrink(0)
		components.CenteredLabel(c, filepath.Base(job.dir), theme.RowNote, semantic.Text.Default).MinWidth(0).Shrink(1)
		ui.Icon(c, components.IconChevronDown.SVG()).FontSize(12 * m.k).TextColor(semantic.Icon.Muted).Shrink(0)
	})
	components.Hint(c, semantic, button, shownPath(job.dir), "", components.HintBelow)
	button.Menu(func(menu *ui.Menu) {
		for _, dir := range a.exportPlaces() {
			if menu.Item(filepath.Base(dir)).Subtitle(shownPath(dir)).Checked(containsPath([]string{job.dir}, dir)).Chosen() {
				a.exportInto(dir)
			}
		}
		menu.Separator()
		if menu.Item(locales.ChooseFolderEllipsis()).Chosen() {
			a.chooseExportDir()
		}
	})
}

// exportTree lists the plan's files under the folder they go into, each
// by its kind's icon, the name it will have, whether it was renamed, and
// its size or what it holds; pressed, the preview shows it.
func (a *AppState) exportTree(c *ui.Context, semantic theme.Theme, m exportMetrics, job *exportJob) {
	indent := theme.Space16.DIPs(m.scale)
	// The folder the files go into heads them, as the root they stand in.
	ui.Row(c).WidthPercent(100).Height(m.row).PaddingX(theme.Space08.DIPs(m.scale)).Gap(theme.Space08.DIPs(m.scale)).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, components.IconFolder.SVG()).FontSize(m.icon).TextColor(semantic.Icon.Muted).Shrink(0)
		components.CenteredLabel(c, shownPath(job.dir), theme.RowNote, semantic.Text.Muted).
			TruncateMiddle().MinWidth(0).Grow(1)
	})
	switch {
	case job.planErr != nil:
		exportFailure(c, semantic, m, job.planErr)
		return
	case job.plan == nil:
		ui.Row(c).PaddingStart(theme.Space08.DIPs(m.scale) + indent).Height(m.row).Gap(theme.Space08.DIPs(m.scale)).AlignItems(ui.Center).Children(func() {
			ui.Spinner(c).Size(14*m.k, 14*m.k)
			components.CenteredLabel(c, locales.ExportReading(job.sheets), theme.RowNote, semantic.Text.Muted)
		})
		return
	}
	for _, f := range job.plan.Files {
		a.exportRow(c, semantic, m, job, f, theme.Space08.DIPs(m.scale)+indent*float32(f.Depth+1))
	}
}

// exportRow is one file of the plan in the list.
func (a *AppState) exportRow(c *ui.Context, semantic theme.Theme, m exportMetrics, job *exportJob, f export.File, lead float32) {
	chosen := f.Path == job.shown
	row := ui.ButtonBase(c.Key("file:" + f.Path)).Label(f.Name()).WidthPercent(100).Height(m.row).
		PaddingStart(lead).PaddingEnd(theme.Space08.DIPs(m.scale)).Gap(theme.Space08.DIPs(m.scale)).AlignItems(ui.Center).Radius(m.inner).
		FocusRing(false).KeepFocus().Selected(chosen)
	switch {
	case chosen:
		row.Background(semantic.Surface.Chosen)
	case row.Hovered():
		row.Background(semantic.Element.Hover)
	}
	row.Children(func() {
		ui.Icon(c, fileIcon(f).SVG()).FontSize(m.icon).TextColor(fileInk(semantic, f)).Shrink(0)
		components.CenteredLabel(c, f.Name(), theme.RowTitle, semantic.Text.Default).
			TruncateMiddle().MinWidth(0).Grow(1)
		if f.Wanted != "" {
			badge := components.BadgeFrame(ui.Row(c), m.scale).Background(semantic.Status.Warning.Background).Children(func() {
				components.CenteredLabel(c, locales.ExportRenamed(), theme.TextXSmall, semantic.Status.Warning.Foreground)
			})
			components.Hint(c, semantic, badge, locales.ExportRenamedHint(f.Wanted), "", components.HintAbove)
		}
		components.CenteredLabel(c, fileMeasure(*job.plan, f), theme.RowMeta, semantic.Text.Muted).
			Shrink(0).FontFeatures("tnum")
	})
	if row.Clicked() {
		job.shown = f.Path
	}
}

func fileIcon(f export.File) components.IconName {
	switch f.Kind {
	case export.KindFolder:
		return components.IconFolder
	case export.KindPicture:
		return components.IconImage
	}
	return components.IconFileText
}

func fileInk(semantic theme.Theme, f export.File) ui.Color {
	if f.Kind == export.KindSheet {
		return semantic.Icon.Accent
	}
	return semantic.Icon.Muted
}

// fileMeasure is a file's size, or a folder's count of what it holds.
func fileMeasure(p export.Plan, f export.File) string {
	if f.Kind != export.KindFolder {
		return components.Bytes(p.Size(f))
	}
	n := f.Sheets + f.Pictures + f.Folders
	return components.Thousands(n)
}

// exportPreview shows the file chosen in the list as it will be written.
func (a *AppState) exportPreview(c *ui.Context, semantic theme.Theme, m exportMetrics, job *exportJob) {
	panel(c, semantic, m, "export-preview").Grow(1).WidthPercent(100).Children(func() {
		f, ok := shownFile(job)
		if !ok {
			ui.Box(c).Grow(1)
			return
		}
		plan := *job.plan
		panelHead(c, semantic, m, func() {
			ui.Icon(c, fileIcon(f).SVG()).FontSize(m.icon).TextColor(fileInk(semantic, f)).Shrink(0)
			components.CenteredLabel(c, f.Path, theme.RowTitle, semantic.Text.Default).
				TruncateStart().MinWidth(0).Grow(1)
			components.CenteredLabel(c, fileMeasure(plan, f), theme.RowMeta, semantic.Text.Muted).
				Shrink(0).FontFeatures("tnum")
		})
		switch f.Kind {
		case export.KindSheet:
			if f.Typeset != nil {
				previewTypeset(c, semantic, m, plan, f)
				break
			}
			previewText(c, semantic, m, plan, f)
		case export.KindPicture:
			previewPicture(c, semantic, m, job, plan, f)
		default:
			previewFolder(c, semantic, m, f)
		}
	})
}

// shownFile is the file the preview shows.
func shownFile(job *exportJob) (export.File, bool) {
	if job.plan == nil {
		return export.File{}, false
	}
	for _, f := range job.plan.Files {
		if f.Path == job.shown {
			return f, true
		}
	}
	return export.File{}, false
}

// previewText shows a sheet's file line by line as it will be written,
// numbered, its front matter in the accent's ink.
func previewText(c *ui.Context, semantic theme.Theme, m exportMetrics, plan export.Plan, f export.File) {
	text := strings.TrimSuffix(plan.Text(f), "\n")
	if text == "" {
		ui.Box(c).Grow(1).WidthPercent(100).MinWidth(0).PaddingX(m.inset).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			components.Label(c, locales.ExportEmptyFile(), theme.TextSmall, semantic.Text.Muted).MinWidth(0)
		})
		return
	}
	lines := strings.Split(text, "\n")
	front := 0
	if plan.FrontMatter && f.Front != "" {
		front = strings.Count(f.Front, "\n")
	}
	more := max(len(lines)-exportPreviewLines, 0)
	lines = lines[:len(lines)-more]
	size := theme.RowNote.Size(m.scale)
	lineH := size * 1.55
	gutter := float32(len(strconv.Itoa(len(lines))))*size*0.62 + 4*m.k
	ui.Scroll(c.Key("preview:" + f.Path)).Grow(1).MinHeight(0).WidthPercent(100).Children(func() {
		ui.Column(c).WidthPercent(100).Padding(m.inset, m.inset, m.inset, m.inset).Children(func() {
			for i, line := range lines {
				ink := semantic.Text.Default
				if i < front {
					ink = semantic.Accent.Foreground
				}
				ui.Row(c).WidthPercent(100).Gap(m.inset).AlignItems(ui.Start).Children(func() {
					ui.Text(c, strconv.Itoa(i+1)).Font(theme.MonoFontFamily).FontSize(size * 0.9).
						FixedLineHeight(lineH).TextColor(semantic.Text.Muted).Width(gutter).Shrink(0).
						TextAlign(ui.End).FontFeatures("tnum").IgnoreInput(true)
					ui.Text(c, line).Font(theme.MonoFontFamily).FontSize(size).FixedLineHeight(lineH).
						TextColor(ink).Grow(1).MinWidth(0)
				})
			}
			if more > 0 {
				ui.Row(c).Padding(4*m.k, 0, 0, gutter+m.inset).Children(func() {
					components.Label(c, locales.ExportMoreLines(more), theme.RowNote, semantic.Text.Muted)
				})
			}
		})
	})
}

// previewPicture shows a picture as it will be copied, and its size.
func previewPicture(c *ui.Context, semantic theme.Theme, m exportMetrics, job *exportJob, plan export.Plan, f export.File) {
	bitmap, state := job.pictures.Picture(f.Media, c.Now())
	ui.Column(c).Grow(1).MinHeight(0).Padding(m.inset, m.inset, m.inset, m.inset).Gap(m.gap).Children(func() {
		ui.Box(c).Grow(1).MinHeight(0).WidthPercent(100).Radius(8 * m.k).Background(semantic.Surface.Inset).
			AlignItems(ui.Center).Justify(ui.Center).Clip().Children(func() {
			switch {
			case bitmap != nil:
				ui.Image(c, bitmap).WidthPercent(100).HeightPercent(100).Fit(ui.Contain)
			case state == page.PictureMissing:
				ui.Icon(c, components.IconImageOff.SVG()).FontSize(28 * m.k).TextColor(semantic.Text.Muted)
			default:
				ui.Spinner(c).Size(16*m.k, 16*m.k)
			}
		})
		note := locales.ExportPictureNote(components.Thousands(f.Picture.Width), components.Thousands(f.Picture.Height),
			components.Bytes(plan.Size(f)))
		components.Label(c, note, theme.TextSmall, semantic.Text.Muted).FontFeatures("tnum").Shrink(0)
	})
}

// previewFolder says what a folder holds.
// previewTypeset says what a sheet's PDF holds: its pages and its size.
func previewTypeset(c *ui.Context, semantic theme.Theme, m exportMetrics, plan export.Plan, f export.File) {
	ui.Column(c).Grow(1).WidthPercent(100).MinWidth(0).PaddingX(m.inset).AlignItems(ui.Center).Justify(ui.Center).Gap(m.gap).Children(func() {
		ui.Icon(c, components.IconFileText.SVG()).FontSize(36 * m.k).TextColor(semantic.Icon.Muted).Shrink(0)
		components.Label(c, f.Name(), theme.RowTitle, semantic.Text.Default).MaxWidthPercent(100).MinWidth(0)
		components.Label(c, locales.PagesCount(f.Pages)+" · "+components.Bytes(plan.Size(f)), theme.RowNote, semantic.Text.Muted).
			MaxWidthPercent(100).MinWidth(0).FontFeatures("tnum")
	})
}

func previewFolder(c *ui.Context, semantic theme.Theme, m exportMetrics, f export.File) {
	var parts []string
	if f.Sheets > 0 {
		parts = append(parts, locales.SheetsCount(f.Sheets))
	}
	if f.Pictures > 0 {
		parts = append(parts, locales.PicturesCount(f.Pictures))
	}
	if f.Folders > 0 {
		parts = append(parts, locales.GroupsCount(f.Folders))
	}
	ui.Column(c).Grow(1).WidthPercent(100).MinWidth(0).PaddingX(m.inset).AlignItems(ui.Center).Justify(ui.Center).Gap(m.gap).Children(func() {
		ui.Icon(c, components.IconFolder.SVG()).FontSize(36 * m.k).TextColor(semantic.Icon.Muted).Shrink(0)
		components.Label(c, f.Name(), theme.RowTitle, semantic.Text.Default).MaxWidthPercent(100).MinWidth(0)
		components.Label(c, strings.Join(parts, " · "), theme.RowNote, semantic.Text.Muted).
			MaxWidthPercent(100).MinWidth(0).FontFeatures("tnum")
	})
}

// exportOptions are the choices of how the files are written, each
// shown in the preview as it is changed.
func (a *AppState) exportOptions(c *ui.Context, semantic theme.Theme, job *exportJob) {
	components.SettingGroup{Key: "export-options", Label: locales.ExportOptions(), Children: func() {
		components.SettingRow{Label: locales.ExportFormat(), Description: locales.ExportFormatNote(), Last: a.exportPDF,
			Control: func() {
				components.Segmented{Key: "export-format", Label: locales.ExportFormat(), Animate: true, Options: []components.SegmentOption{
					{Key: "markdown", Label: locales.Markdown(), Selected: !a.exportPDF, Action: func() { a.setExportPDF(false) }},
					{Key: "pdf", Label: locales.Pdf(), Selected: a.exportPDF, Action: func() { a.setExportPDF(true) }},
				}}.View(c, semantic)
			}}.View(c, semantic)
		// A PDF is the sheet as it reads: no front matter to put in it.
		if !a.exportPDF {
			components.SettingRow{Label: locales.ExportFrontMatter(), Description: locales.ExportFrontMatterNote(), Last: true,
				Control: func() {
					components.Switch{Key: "front-matter", Label: locales.ExportFrontMatter(), Selected: a.exportFrontMatter,
						Disabled: job.writing, Animate: true, Action: a.setExportFrontMatter}.View(c, semantic)
				}}.View(c, semantic)
		}
	}}.View(c, semantic).Shrink(0)
}

// exportFoot says where the export goes, how far writing has come or why
// it stopped, beside Cancel and Export.
func (a *AppState) exportFoot(c *ui.Context, semantic theme.Theme, m exportMetrics, job *exportJob) {
	ui.Row(c).WidthPercent(100).Gap(m.gap).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Row(c).Grow(1).MinWidth(0).Gap(theme.Space08.DIPs(m.scale)).AlignItems(ui.Center).Children(func() {
			switch {
			case job.writing && job.plan != nil:
				total := len(job.plan.Files)
				ui.Progress(c, float64(job.written)/float64(max(total, 1))).Width(140 * m.k).Shrink(0)
				components.Label(c, locales.ExportWriting(components.Thousands(job.written), components.Thousands(total)), theme.TextSmall, semantic.Text.Muted).
					SingleLine().FontFeatures("tnum")
			case job.writeErr != nil:
				exportFailure(c, semantic, m, job.writeErr)
			case job.plan != nil && job.planErr == nil:
				components.Label(c, locales.ExportInto(len(job.plan.Files), shownPath(job.dir)), theme.TextSmall,
					semantic.Text.Muted).SingleLine().TruncateMiddle().MinWidth(0).FontFeatures("tnum")
			}
		})
		components.Button{Key: "cancel", Label: locales.Cancel(), Style: components.SubtleButton,
			Size: components.MediumButton, Disabled: job.writing, Action: a.closeExport}.View(c, semantic)
		components.Button{Key: "export", Label: locales.Export(), Style: components.PrimaryButton,
			Size: components.MediumButton, Disabled: job.writing || job.plan == nil || job.planErr != nil,
			Action: a.runExport}.View(c, semantic)
	})
}

// exportFailure says why the export cannot go on, in the danger's ink.
func exportFailure(c *ui.Context, semantic theme.Theme, m exportMetrics, err error) {
	ui.Row(c).WidthPercent(100).MinWidth(0).Gap(theme.Space06.DIPs(m.scale)).AlignItems(ui.Center).PaddingX(theme.Space08.DIPs(m.scale)).Children(func() {
		ui.Icon(c, components.IconError.SVG()).FontSize(m.inline).TextColor(semantic.Status.Danger.Foreground).Shrink(0)
		components.Label(c, describeNotice(err), theme.TextSmall, semantic.Status.Danger.Foreground).Grow(1).MinWidth(0).MaxLines(2)
	})
}
