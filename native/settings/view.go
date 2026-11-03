// Package settings composes the source-faithful settings window.
package settings

import (
	"runtime"
	"strings"

	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/scrollfx"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

const (
	// contentWidth bounds the settings column so controls stay near their
	// labels in wide windows.
	contentWidth = float32(680)
	// spyOffset is how far below the viewport top a section's title may sit
	// and still count as the section being read.
	spyOffset = float32(48)
)

func (s *State) View(c *ui.Context) {
	// A recording begun or ended while building, as a capture that lost
	// the keyboard, is told before the frame ends.
	defer s.reportRecording()
	semantic, changing := s.Colors.At(s.Theme, c.Now())
	c.SetTheme(s.Theme.MyGoThemeFor(semantic))
	if changing {
		c.AnimationFrame()
	}
	scale := s.Theme.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	s.followPending(c)
	ui.Row(c).Fill().Background(semantic.Surface.Background).Children(func() {
		s.navigation(c, semantic, metrics)
		s.page(c, semantic, metrics)
	})
}

func (s *State) navigation(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
) {
	scale := s.Theme.UIFontSize
	top := theme.Space12.DIPs(scale)
	if runtime.GOOS == "darwin" {
		top = theme.DefaultLayout.Titlebar * scale / theme.DefaultUIFontSize
	}
	animate := s.motion(c)
	ui.Column(c).Width(240*scale/theme.DefaultUIFontSize).HeightPercent(100).
		Shrink(0).Background(semantic.Surface.Panel).
		BorderWidth(0, 1, 0, 0).BorderColor(semantic.Border.Variant).Children(func() {
		// The sidebar's top, around the window controls, is the window's
		// title bar: it drags the window, and the page beside it stays free
		// to show content right up to the window's edge.
		ui.Box(c).WidthPercent(100).Height(top).Shrink(0).DragWindow()
		ui.Column(c).Grow(1).MinHeight(0).Padding(
			0, theme.Space12.DIPs(scale), 0, theme.Space12.DIPs(scale),
		).Children(func() {
			components.TextField{
				Key: "settings-search", Label: "Search settings",
				Placeholder: "Search settings…", Value: &s.Query,
				Icon: components.IconSearch.SVG(), ClearIcon: components.IconClose.SVG(),
				Animate: animate,
			}.View(c, semantic)
			ui.Box(c).Height(theme.Space12.DIPs(scale))
			scroll := ui.Scroll(c).Grow(1).MinHeight(0).TrackScroll(&s.navScroll)
			scroll = components.BindOverscroll(
				scroll,
				&s.navScroll,
				&s.navOver,
				scrollfx.Vertical,
				components.OverscrollList,
				scroll.LayoutBounds().H,
				scale,
			)
			scroll.Children(func() {
				list := ui.Column(c).Role(ui.RoleTree).Label("Settings Navigation").
					FocusGroup(ui.Vertical).
					Padding(0, 0, theme.Space12.DIPs(scale), 0).
					Gap(theme.Space02.DIPs(scale))
				s.navOver.Apply(c, list, animate)
				track := components.TrackSelection(list,
					components.NavigationSelection(c, semantic), animate)
				list.Children(func() {
					s.tree(c, semantic, metrics, animate, track)
				})
			})
		})
		ui.Row(c).Height(theme.DefaultLayout.Status * scale / theme.DefaultUIFontSize).
			Shrink(0).PaddingX(theme.Space16.DIPs(scale)).AlignItems(ui.Center).
			Children(func() {
				components.Label(c, s.statusText(), theme.TextXSmall, semantic.Text.Muted).
					Grow(1).MinWidth(0).SingleLine()
			})
	})
}

// tree draws the categories and their sections. A category's sections open
// and close by animating their height, the chevron turning with them; the
// section being read is highlighted and kept in view as the page scrolls.
func (s *State) tree(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
	animate bool,
	track *components.SelectionTrack,
) {
	scale := s.Theme.UIFontSize
	query := strings.ToLower(strings.TrimSpace(s.Query))
	active := s.activeSection()
	activeChanged := active != s.lastActive
	s.lastActive = active
	gap := theme.Space02.DIPs(scale)
	for _, category := range appsettings.AllCategories {
		children := s.categoryChildren(category, query)
		if query != "" && len(children) == 0 {
			continue
		}
		open := s.categoryOpen(category)
		target := float32(0)
		if open {
			target = 1
		}
		group := ui.Column(c).Key("settings-tree-" + category.Title()).WidthPercent(100)
		reveal := target
		if animate {
			reveal = components.Follow(group, "reveal", target, components.MotionSettle)
		}
		current := s.Category == category
		group.Children(func() {
			components.NavigationRow{
				Key: "settings-category-" + category.Title(), Label: category.Title(),
				Selected: current && (!open || active == ""), Track: track,
				Animate: animate,
				Role:    ui.RoleTreeItem, FullWidth: true, Expanded: &open,
				Leading: func() {
					ui.Icon(c, components.IconChevronRight.SVG()).
						FontSize(14 * scale / theme.DefaultUIFontSize).
						TextColor(semantic.Icon.Muted).Rotate(90 * reveal)
				},
				Action: func() { s.chooseCategory(category) },
			}.View(c, semantic)
			if reveal <= 0.001 || len(children) == 0 {
				return
			}
			natural := float32(len(children)) * (metrics.Height + gap)
			ui.Column(c).WidthPercent(100).Height(natural * reveal).Clip().
				Opacity(reveal).Children(func() {
				ui.Column(c).WidthPercent(100).Padding(gap, 0, 0, 0).Gap(gap).Children(func() {
					for _, label := range children {
						label := label
						selected := current && open && active == label
						row := components.NavigationRow{
							Key:   "settings-section-" + category.Title() + "-" + label,
							Label: label, Selected: selected, Track: track,
							Animate: animate,
							Role:    ui.RoleTreeItem, FullWidth: true,
							Leading: func() {
								ui.Box(c).Width(16*scale/theme.DefaultUIFontSize).
									HeightPercent(100).Shrink(0).
									BorderWidth(0, 1, 0, 0).
									BorderColor(semantic.Border.Default.Alpha(0.5))
							},
							Action: func() { s.chooseSection(category, label) },
						}.View(c, semantic)
						if selected && activeChanged {
							row.ScrollIntoView()
						}
					}
				})
			})
		})
	}
}

func (s *State) categoryChildren(category appsettings.Category, query string) []string {
	labels := make([]string, 0)
	switch category {
	case appsettings.Editor:
		for _, section := range s.Data.Sections {
			if query == "" || s.sectionMatches(section, query) {
				labels = append(labels, section.Title)
			}
		}
	case appsettings.Keymap:
		seen := map[string]bool{}
		for _, shortcut := range s.Data.Shortcuts {
			if query != "" && !strings.Contains(strings.ToLower(shortcutWords(shortcut)), query) {
				continue
			}
			if !seen[shortcut.Group] {
				seen[shortcut.Group] = true
				labels = append(labels, shortcut.Group)
			}
		}
	case appsettings.Appearance:
		for _, label := range []string{"Theme", "Writing Environment"} {
			if query == "" || strings.Contains(strings.ToLower(label), query) {
				labels = append(labels, label)
			}
		}
	}
	return labels
}

func (s *State) page(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
) {
	scale := s.Theme.UIFontSize
	ui.Column(c).Grow(1).MinWidth(0).HeightPercent(100).
		Background(semantic.Surface.Background).Children(func() {
		// No title bar band: the page starts where the sidebar's search does
		// and scrolls up beneath the window's top edge.
		scroll := ui.Scroll(c).Grow(1).MinHeight(0).TrackScroll(&s.contentScroll)
		scroll = components.BindOverscroll(
			scroll,
			&s.contentScroll,
			&s.contentOver,
			scrollfx.Vertical,
			components.OverscrollPanel,
			scroll.LayoutBounds().H,
			scale,
		)
		scroll.Children(func() {
			row := ui.Row(c).WidthPercent(100).Justify(ui.Center).
				Padding(theme.DefaultLayout.Titlebar*scale/theme.DefaultUIFontSize,
					theme.Space32.DIPs(scale), 0, theme.Space32.DIPs(scale))
			s.contentOver.Apply(c, row, s.motion(c))
			row.Children(func() {
				content := ui.Column(c).Key("settings-content").WidthPercent(100).
					MaxWidth(contentWidth * scale / theme.DefaultUIFontSize).
					Gap(28 * scale / theme.DefaultUIFontSize)
				if bounds := content.Bounds(); bounds.H > 0 {
					s.contentTop = bounds.Y
				}
				s.building = s.building[:0]
				content.Children(func() {
					s.pageHeader(c, semantic)
					switch s.Category {
					case appsettings.Editor:
						s.editorPage(c, semantic, metrics)
					case appsettings.Keymap:
						s.keymapPage(c, semantic, metrics)
					case appsettings.Appearance:
						s.appearancePage(c, semantic, metrics)
					}
					if len(s.building) == 0 {
						components.Label(c, "No settings match your search.",
							theme.TextSmall, semantic.Text.Muted)
					}
					ui.Box(c).Height(theme.Space24.DIPs(scale))
				})
				s.anchors = append(s.anchors[:0], s.building...)
			})
		})
		// Until the page scrolls up into it, the band above it is the
		// title bar's, and drags the window.
		if band := theme.DefaultLayout.Titlebar*scale/theme.DefaultUIFontSize - s.contentScroll.Y; band > 0.5 {
			ui.Box(c).Key("title-band").Absolute().Top(0).Left(0).Right(0).Height(band).DragWindow()
		}
	})
}

func (s *State) pageHeader(c *ui.Context, semantic theme.Theme) {
	scale := s.Theme.UIFontSize
	ui.Column(c).Padding(
		theme.Space08.DIPs(scale), theme.Space04.DIPs(scale),
		0, theme.Space04.DIPs(scale),
	).Gap(theme.Space04.DIPs(scale)).Children(func() {
		components.Headline(c, s.Category.Title(), theme.HeadlineXLarge,
			semantic.Text.Heading)
		components.Label(c, s.Category.Description(), theme.TextSmall,
			semantic.Text.Muted)
	})
}

// section draws one anchored section: its title over a card of rows. The
// anchor records where the section sits so the navigation can follow it.
func (s *State) section(
	c *ui.Context,
	semantic theme.Theme,
	label string,
	description string,
	rows func(),
) {
	s.anchored(c, label, func() {
		components.SectionHeader{Label: label, Description: description}.View(c, semantic)
		components.SettingGroup{
			Key: "settings-group-" + label, Label: label, Children: rows,
		}.View(c, semantic)
	})
}

// anchored draws content as one navigable block: the navigation scrolls to
// it and highlights label while it is the block being read.
func (s *State) anchored(c *ui.Context, label string, content func()) {
	scale := s.Theme.UIFontSize
	block := ui.Column(c).Key("settings-anchor-" + label).WidthPercent(100).
		Gap(theme.Space08.DIPs(scale))
	top := float32(-1)
	if bounds := block.Bounds(); bounds.H > 0 {
		top = bounds.Y - s.contentTop
	}
	s.building = append(s.building, anchor{Label: label, Top: top})
	block.Children(content)
}

// activeSection is the section the page is showing: the one a navigation
// scroll is gliding to, else the last whose title has reached the top.
func (s *State) activeSection() string {
	if s.target != "" {
		if s.contentScroll.Scrolling() {
			return s.target
		}
		s.target = ""
	}
	if len(s.anchors) == 0 {
		return ""
	}
	scale := s.Theme.UIFontSize
	if s.contentScroll.MaxY > 0 && s.contentScroll.Y >= s.contentScroll.MaxY-1 {
		return s.anchors[len(s.anchors)-1].Label
	}
	active := s.anchors[0].Label
	for _, section := range s.anchors {
		if section.Top < 0 {
			continue
		}
		if section.Top-s.contentScroll.Y <= spyOffset*scale/theme.DefaultUIFontSize {
			active = section.Label
		}
	}
	return active
}

// followPending scrolls to a section the navigation chose once the page
// that holds it has been laid out.
func (s *State) followPending(c *ui.Context) {
	if s.pending == "" {
		return
	}
	for _, section := range s.anchors {
		if section.Label != s.pending || section.Top < 0 {
			continue
		}
		animation := ui.Animation{}
		if s.motion(c) {
			animation = components.Spring(components.MotionScene)
			// A scroll is in DIPs: at rest within a quarter of one.
			animation.Spring.Precision = 0.25
		}
		s.contentScroll.ScrollTo(0, max(0, section.Top), animation)
		s.target, s.pending = s.pending, ""
		return
	}
}

func (s *State) chooseCategory(category appsettings.Category) {
	if s.Category == category && strings.TrimSpace(s.Query) == "" {
		s.expanded[category] = !s.expanded[category]
		return
	}
	s.selectCategory(category)
}

func (s *State) chooseSection(category appsettings.Category, label string) {
	s.selectCategory(category)
	s.pending = label
}

func (s *State) selectCategory(category appsettings.Category) {
	if s.Category != category {
		s.contentScroll.Y = 0
		s.contentOver.Reset()
		s.anchors = s.anchors[:0]
		s.pending, s.target = "", ""
	}
	s.Category = category
	s.expanded[category] = true
}

func (s *State) categoryOpen(category appsettings.Category) bool {
	if strings.TrimSpace(s.Query) != "" {
		return true
	}
	return s.expanded[category]
}

func (s *State) statusText() string {
	page := strings.TrimSpace(s.Data.Page)
	if page == "" {
		return "No open page"
	}
	return page + " · changes stay in memory"
}

func (s *State) sectionMatches(section appsettings.Section, query string) bool {
	if strings.Contains(strings.ToLower(section.Title+" "+s.spellWords(section.Description)), query) {
		return true
	}
	for _, setting := range section.Settings {
		if strings.Contains(strings.ToLower(s.spellWords(setting.Label+" "+setting.Description)), query) {
			return true
		}
	}
	for _, shortcut := range section.Shortcuts {
		if strings.Contains(strings.ToLower(shortcut.Label), query) {
			return true
		}
	}
	return false
}
