package overlay

import (
	"github.com/egoist/mygo/ui"
)

func (p *CommandPalette) View(c *ui.Context) {
	if !p.Open {
		return
	}
	open := p.Open
	ui.Modal(c, &open, func() {
		items := p.Filtered()
		if p.Selected >= len(items) {
			p.Selected = max(0, len(items)-1)
		}
		if c.Shortcut(0, ui.KeyDown) {
			p.Move(1)
		}
		if c.Shortcut(0, ui.KeyUp) {
			p.Move(-1)
		}
		if c.Shortcut(0, ui.KeyEscape) {
			p.Open = false
			return
		}
		ui.Column(c).Width(620).MaxWidthPercent(90).MaxHeightPercent(70).
			Padding(12).Gap(8).Radius(12).Background(c.Theme().Background).
			Border(1, c.Theme().Border).Children(func() {
			search := ui.SearchField(c.Key("command-query"), &p.Query).AutoFocus()
			if search.Changed() {
				p.Selected = 0
			}
			if search.Submitted() {
				p.Submit()
			}
			ui.Divider(c)
			if len(items) == 0 {
				ui.Text(c, "No commands match").TextColor(c.Theme().TextMuted)
				return
			}
			ui.Scroll(c).MaxHeight(420).Children(func() {
				for index, command := range items {
					button := ui.Button(c.Key("command-"+string(command.ID)), command.Label).FillWidth()
					if index == p.Selected {
						button.Background(c.Theme().Selection)
					}
					if command.Binding != "" {
						button.Label(command.Label + " " + command.Binding)
					}
					if button.Clicked() {
						p.Selected = index
						p.Submit()
					}
				}
			})
		})
	})
	p.Open = open && p.Open
}
