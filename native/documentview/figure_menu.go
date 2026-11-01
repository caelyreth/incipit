package documentview

import (
	"runtime"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/egoist/mygo/ui"
)

// FigureHost does for a figure what the page cannot do itself: it views,
// copies, saves and shows its picture's file, replaces it with another,
// and asks for the words that describe it.
type FigureHost interface {
	ViewPicture(figure surface.Figure)
	CopyPicture(media string)
	SavePicture(media string)
	RevealPicture(media string)
	ReplacePicture(node model.NodeID)
	EditAlt(node model.NodeID, alt string)
}

// A figure's menu opens over it on a secondary click, which selects it
// first, as a click does: what is done to the picture, then how it is set
// on the page, then what describes it, its caption among them. Elsewhere
// the page's menu is the editing commands, as any text's.
func (e *Element) contextMenu(m *ui.Menu) {
	bridge, ok := e.Input.(*Bridge)
	if !ok || bridge == nil || bridge.Figures == nil {
		m.EditItems()
		return
	}
	figure, ok := bridge.figureSelection()
	if !ok || figure.Node != e.menuFigure {
		if !e.tableMenu(m, bridge) {
			m.EditItems()
		}
		return
	}
	host := bridge.Figures
	if m.Item("View Image").Chosen() {
		host.ViewPicture(figure)
	}
	m.Separator()
	if m.Item("Copy Image").Chosen() {
		host.CopyPicture(figure.Media)
	}
	if m.Item("Save Image As…").Chosen() {
		host.SavePicture(figure.Media)
	}
	if m.Item(revealLabel()).Chosen() {
		host.RevealPicture(figure.Media)
	}
	if m.Item("Replace Image…").Chosen() {
		host.ReplacePicture(figure.Node)
	}
	m.Separator()
	shape := func(change func(*editor.ImageShape)) {
		_, _ = bridge.Controller.ShapeImage(figure.Node, change)
	}
	m.Submenu("Align", func(m *ui.Menu) {
		if m.Item("Center").Radio(figure.Align == surface.FigureCenter).Chosen() {
			shape(func(s *editor.ImageShape) { s.Align = surface.FigureCenter })
		}
		if m.Item("Start").Radio(figure.Align == surface.FigureStart).Chosen() {
			shape(func(s *editor.ImageShape) { s.Align = surface.FigureStart })
		}
	})
	if m.Item("Fill Column Width").Checked(figure.Span == 1).Chosen() {
		shape(func(s *editor.ImageShape) { s.Span = 1 })
	}
	if m.Item("Original Size").Disabled(figure.Span == 0).Chosen() {
		shape(func(s *editor.ImageShape) { s.Span = 0 })
	}
	m.Separator()
	if figure.Caption == 0 && m.Item("Add Caption").Chosen() {
		caption := int(figure.CaptionStart())
		_ = bridge.Controller.SetSelection(controller.UTF16Range{Start: caption, End: caption})
	}
	if m.Item("Edit Alt Text…").Chosen() {
		host.EditAlt(figure.Node, figure.Alt)
	}
	m.Separator()
	if m.Item("Delete Image").Chosen() {
		_, _ = bridge.Controller.ReplaceSelection("", kernel.EventOriginSemantic)
	}
}

// revealLabel names showing a file where it is, as the system does.
func revealLabel() string {
	if runtime.GOOS == "darwin" {
		return "Show in Finder"
	}
	return "Show in Folder"
}

// figureSelection is the figure the selection is, as its whole line.
func (b *Bridge) figureSelection() (surface.Figure, bool) {
	if b == nil || b.Controller == nil {
		return surface.Figure{}, false
	}
	flow, err := b.Controller.Session().SurfaceFlow()
	if err != nil {
		return surface.Figure{}, false
	}
	selection := b.Controller.Selection().Range
	figure, ok := flow.FigureAt(model.UTF16Offset(selection.Start))
	if !ok {
		return surface.Figure{}, false
	}
	line, ok := b.Controller.Session().BlockRange(figure.Node)
	if !ok || line != model.NewUTF16Range(model.UTF16Offset(selection.Start), model.UTF16Offset(selection.End)) {
		return surface.Figure{}, false
	}
	return figure, true
}

// copyFigure copies the selected figure's picture, when the selection is
// one, as Copy copies text.
func (b *Bridge) copyFigure() bool {
	if b.Figures == nil {
		return false
	}
	figure, ok := b.figureSelection()
	if !ok {
		return false
	}
	b.Figures.CopyPicture(figure.Media)
	return true
}

// viewSelectedFigure views the selected figure's picture.
func (b *Bridge) viewSelectedFigure() bool {
	if b == nil || b.Figures == nil {
		return false
	}
	figure, ok := b.figureSelection()
	if ok {
		b.Figures.ViewPicture(figure)
	}
	return ok
}
