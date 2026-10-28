package overlay

import "github.com/egoist/mygo/ui"

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
func (p *PageTitle) View(c *ui.Context) {
	if p == nil || !p.Open {
		return
	}
	open := p.Open
	cancelled := false
	committed := false
	ui.Modal(c, &open, func() {
		ui.Column(c).Key("page-title-dialog").Width(460).MaxWidthPercent(90).
			Padding(24).Gap(12).Radius(12).Background(c.Theme().Background).
			Border(1, c.Theme().Border).Children(func() {
			ui.Text(c, "Name this page").FontSize(20).Bold().TextColor(c.Theme().Text)
			ui.Text(c, "A title for navigation. It stays in memory with this session.").
				FontSize(12).TextColor(c.Theme().TextMuted)
			field := ui.TextInput(c.Key("page-title-field"), &p.Value).
				Label("Page title").AutoFocus()
			ui.Row(c).Justify(ui.End).Gap(8).Children(func() {
				if ui.Button(c.Key("cancel-page-title"), "Cancel").Clicked() {
					cancelled = true
				}
				if ui.PrimaryButton(c.Key("confirm-page-title"), "Rename page").Clicked() ||
					field.Submitted() {
					committed = true
				}
			})
			if c.Shortcut(0, ui.KeyEscape) {
				cancelled = true
			}
		})
	})
	p.Open = open && !cancelled && !committed
	switch {
	case committed:
		p.Open = false
		if p.Confirm != nil {
			p.Confirm(p.PageID, p.Value)
		}
	case cancelled || !p.Open:
		p.Open = false
		if p.Cancel != nil {
			p.Cancel(p.PageID)
		}
	}
}
