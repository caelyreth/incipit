package window

import (
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo/ui"
)

// toastWidth is how wide a toast is at the default type size, as narrow
// as a sentence or two reads in.
const toastWidth = 340

// toastSpring moves toasts in, out and along as others come and go.
var toastSpring = components.Settle(components.MotionNormal)

// View shows what was posted since the last frame, and the toasts showing,
// at the window's bottom right, bottom clear of the window's foot. Call it
// once a frame, from the window's view.
func (n *NoticeCenter) View(c *ui.Context, semantic theme.Theme, animate bool, bottom float32) {
	for _, next := range n.take() {
		if next.dismiss != "" {
			c.CloseToast(next.dismiss)
			continue
		}
		c.AddToast(noticeToast(next.notice))
	}
	scale := components.UIFontSize(c)
	margin := theme.Space16.DIPs(scale)
	ui.ToastViewportBase(c, func(viewport ui.Element, toasts []ui.Toast) {
		viewport.AlignItems(ui.End).Gap(theme.Space08.DIPs(scale)).
			Padding(margin, margin, bottom+margin, margin)
		for _, shown := range toasts {
			noticeToastView(c, semantic, animate, shown)
		}
	})
}

// toast is a notice as MyGo keeps it, its kind in Data.
func noticeToast(notice Notice) ui.Toast {
	timeout := notice.Kind.shown()
	if notice.Sticky {
		timeout = -1
	}
	return ui.Toast{
		ID: notice.id(), Title: notice.Title, Description: notice.Description,
		Type: notice.Kind.String(), Timeout: timeout,
		Action: notice.Action, OnAction: notice.OnAction, Data: notice.Kind,
	}
}

// view is one toast in the window's dialog material: its kind's icon in
// its hue, what it says, its action, and a button to close it.
func noticeToastView(c *ui.Context, semantic theme.Theme, animate bool, shown ui.Toast) {
	scale := components.UIFontSize(c)
	kind, _ := shown.Data.(NoticeKind)
	parts := ui.ToastBase(c, shown)
	components.ElevatedSurface(parts.Root, semantic, scale, components.PanelElevation)
	parts.Root.Row().Width(toastWidth*scale/theme.DefaultUIFontSize).MaxWidthPercent(100).
		AlignItems(ui.Start).Gap(theme.Space08.DIPs(scale)).
		Padding(theme.Space12.DIPs(scale), theme.Space08.DIPs(scale),
			theme.Space12.DIPs(scale), theme.Space12.DIPs(scale)).
		Transition(noticeTransition(animate, scale)).
		Children(func() {
			line := theme.RowTitle.LineHeight(scale)
			// The icon stands on the title's line.
			ui.Row(c).Height(line).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, kind.icon().SVG()).FontSize(16 * scale / theme.DefaultUIFontSize).
					TextColor(kind.color(semantic))
			})
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).
				Gap(theme.Space02.DIPs(scale)).Children(func() {
				components.Label(c, shown.Title, theme.RowTitle, semantic.Text.Default).
					FontWeight(600)
				if shown.Description != "" {
					components.Label(c, shown.Description, theme.RowNote, semantic.Text.Muted)
				}
				if shown.Action == "" {
					return
				}
				ui.Row(c).Padding(theme.Space06.DIPs(scale), 0, 0, 0).Children(func() {
					components.Button{
						Key: "action", Label: shown.Action,
						Style: components.FilledButton, Size: components.CompactButton,
						Animate: animate,
						Action: func() {
							if shown.OnAction != nil {
								shown.OnAction()
							}
							c.CloseToast(shown.ID)
						},
					}.View(c, semantic)
				})
			})
			components.IconButton{
				Key: "close", Label: locales.Close(), Icon: components.IconClose.SVG(),
				Style: components.SubtleButton, Size: components.CompactButton,
				IconSize: 14 * scale / theme.DefaultUIFontSize, Animate: animate,
				Action: func() { c.CloseToast(shown.ID) },
			}.View(c, semantic).Shrink(0)
		})
}

// transition brings a toast up into its place and lets it fade where it
// stood, the toasts above gliding down to close the gap; without motion
// they come and go at once.
func noticeTransition(animate bool, scale float32) ui.ElementTransition {
	if !animate {
		return ui.ElementTransition{Duration: 1}
	}
	rise := 12 * scale / theme.DefaultUIFontSize
	return ui.ElementTransition{
		Animation: &toastSpring,
		Position:  true,
		Enter:     &ui.Motion{Y: rise, Scale: 0.96, Opacity: 0},
		Exit:      &ui.Motion{Scale: 0.96, Opacity: 0},
	}
}

func (k NoticeKind) icon() components.IconName {
	switch k {
	case NoticeSuccess:
		return components.IconCheckCircle
	case NoticeWarning:
		return components.IconWarning
	case NoticeError:
		return components.IconError
	}
	return components.IconInfo
}

func (k NoticeKind) color(semantic theme.Theme) ui.Color {
	switch k {
	case NoticeSuccess:
		return semantic.Status.Success.Foreground
	case NoticeWarning:
		return semantic.Status.Warning.Foreground
	case NoticeError:
		return semantic.Status.Danger.Foreground
	}
	return semantic.Status.Info.Foreground
}
