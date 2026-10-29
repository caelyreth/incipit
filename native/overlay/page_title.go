package overlay

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// PageTitle is the single transient dialog used to rename a retained page.
type PageTitle struct {
	Open    bool
	PageID  string
	Value   string
	Confirm func(pageID, title string)
	Cancel  func(pageID string)
}

// Begin opens the dialog for one owned page.
func (p *PageTitle) Begin(pageID, title string) {
	if p == nil {
		return
	}
	p.Open = true
	p.PageID = pageID
	p.Value = title
}

// View composes the rename dialog and commits only on explicit confirmation
// or Enter. Closing the modal is treated as cancellation.
func (p *PageTitle) View(c *ui.Context, semantic theme.Theme, animate bool) {
	if p == nil || !p.Open {
		return
	}
	open := p.Open
	committed := false
	scale := semanticScale(c)
	components.Dialog{
		Key: "page-title-dialog", Label: "Page title dialog", Width: 560, Animate: animate,
		Content: func(panel ui.Element) {
			panel.Padding(24 * scale).Children(func() {
				ui.Column(c).Gap(16 * scale).Children(func() {
					components.Headline(c, "Name this page",
						theme.HeadlineSmall, semantic.Text.Heading)
					components.Label(c,
						"A title for navigation. It stays in memory with this session.",
						theme.TextSmall, semantic.Text.Muted,
					)
					components.TextField{
						Key: "page-title-field", Label: "Page title", Placeholder: "Page title",
						Value: &p.Value, Icon: components.IconDocument.SVG(),
						ClearIcon: components.IconClose.SVG(), Animate: animate,
						AutoFocus: true, SelectAll: true,
						Submit: func() { committed = true },
					}.View(c, semantic)
					ui.Row(c).Justify(ui.End).Gap(8 * scale).Children(func() {
						components.Button{
							Key: "cancel-page-title", Label: "Cancel",
							Style: components.SubtleButton, Size: components.MediumButton,
							Animate: animate, Action: func() { open = false },
						}.View(c, semantic)
						components.Button{
							Key: "confirm-page-title", Label: "Rename page",
							Style: components.PrimaryButton, Size: components.MediumButton,
							Animate: animate, Action: func() { committed = true },
						}.View(c, semantic)
					})
				})
			})
		},
	}.View(c, semantic, &open)
	p.Open = open && !committed
	switch {
	case committed:
		p.Open = false
		if p.Confirm != nil {
			p.Confirm(p.PageID, p.Value)
		}
	case !p.Open:
		if p.Cancel != nil {
			p.Cancel(p.PageID)
		}
	}
}

func semanticScale(c *ui.Context) float32 {
	if c == nil || c.Theme() == nil || c.Theme().FontSize <= 0 {
		return 1
	}
	return c.Theme().FontSize / theme.DefaultUIFontSize
}
