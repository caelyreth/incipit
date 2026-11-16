package shell

import (
	"math"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/stage"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

type pillState struct {
	query        string
	input        queryInput
	focus        bool
	typing       bool
	scopeGroup   string
	fidelityStep int
	pan          stage.Point
}

// PillQuery returns the text currently written in the field pill.
func (s *State) PillQuery() string {
	if s == nil {
		return ""
	}
	return s.pill.query
}

// SetPillQuery sets the pill's query text.
func (s *State) SetPillQuery(q string) {
	if s != nil {
		s.pill.query = q
	}
}

// pillView draws the viewport-anchored Neverland pill inside the field room:
// the three-strip mark and wordmark, quiet posture chips, and the live query
// field.
func (s *State) pillView(c *ui.Context, semantic theme.Theme, scene stage.Scene, animate bool) {
	if scene.Pill.Opacity <= 0.001 {
		return
	}
	scale := s.Settings.UIFontSize
	k := scale / theme.DefaultUIFontSize
	roomW := scene.FieldRoom.W
	if roomW <= 0 {
		roomW = scene.Field.Rect.W
	}
	width := min(380*k, max(240*k, roomW-48*k))

	pill := ui.Column(c).Key("pill").Absolute().
		Left(scene.Pill.Rect.X).Top(scene.Pill.Rect.Y).
		TranslatePercent(-50, -50).
		Scale(scene.Pill.Scale, scene.Pill.Scale).
		Opacity(scene.Pill.Opacity).
		Width(width).
		Padding(10*k, 12*k, 10*k, 12*k).Gap(theme.Space08.DIPs(scale)).
		Role(ui.RoleGroup).Label("Neverland pill")
	components.ElevatedSurface(pill, semantic, scale, components.PopoverElevation)
	pill.Radius(16 * k)
	if !scene.Pill.Interactive {
		pill.IgnoreInput(true)
	}

	pill.Children(func() {
		ui.Row(c).Key("pill-head").WidthPercent(100).AlignItems(ui.Center).Gap(theme.Space08.DIPs(scale)).Children(func() {
			s.threeStripMark(c, semantic, k)
			ui.Row(c).Key("pill-wordmark").AlignItems(ui.End).Children(func() {
				size := 13 * k
				ui.Text(c, "Neverland").Font(theme.UIFontFamily).FontSize(size).FontWeight(700).
					LetterSpacing(-0.02 * size).TextColor(semantic.Text.Heading).SingleLine()
				ui.Text(c, ".").Font(theme.UIFontFamily).FontSize(size).FontWeight(700).
					TextColor(semantic.Accent.Foreground).SingleLine()
			})
			ui.Box(c).Grow(1)
			s.pillPostureChips(c, semantic, k, animate)
		})

		_, input := queryField{
			Key:         "pill-query",
			Label:       "Search field",
			Placeholder: "Search sheets, #group, $ for keys",
			Value:       &s.pill.query,
			In:          &s.pill.input,
			Scope:       query.InNotebook,
			Focus:       s.pill.focus && scene.Pill.Interactive,
			Animate:     animate,
			Keys: func(key ui.TextKey) bool {
				if key.Key != ui.KeyEscape {
					return false
				}
				if s.pill.query != "" {
					s.pill.query = ""
					s.pill.input.edit = &queryEdit{from: 0, to: math.MaxInt, text: ""}
					return true
				}
				s.ReturnToDesk()
				return true
			},
		}.View(s, c, semantic)
		s.pill.typing = input.Focused()
		if scene.Pill.Interactive {
			s.pill.focus = false
		}
	})
}

// threeStripMark draws the rounded three-strip Neverland emblem.
func (s *State) threeStripMark(c *ui.Context, semantic theme.Theme, k float32) {
	ui.Column(c).Key("pill-mark").Gap(2 * k).AlignItems(ui.Start).Children(func() {
		widths := [3]float32{14 * k, 10 * k, 6 * k}
		for i, w := range widths {
			color := semantic.Text.Muted
			if i == 2 {
				color = semantic.Accent.Foreground
			}
			ui.Box(c).Width(w).Height(2.5 * k).Radius(2 * k).Background(color)
		}
	})
}

func (s *State) pillPostureChips(c *ui.Context, semantic theme.Theme, k float32, animate bool) {
	chip := func(key, label string, active bool, action func()) {
		bg := semantic.GhostElement.Background
		fg := semantic.Text.Muted
		if active {
			bg = semantic.Surface.Chosen
			fg = semantic.Text.Default
		}
		btn := ui.ButtonBase(c.Key(key)).Height(22 * k).PaddingX(8 * k).
			AlignItems(ui.Center).Justify(ui.Center).
			Radius(7 * k).Background(bg).Label(label).
			FocusRing(false).KeepFocus()
		if btn.Hovered() && !active {
			btn.Background(semantic.Element.Hover)
		}
		if btn.Clicked() {
			action()
		}
		btn.Children(func() {
			components.Label(c, label, theme.TextXSmall, fg).SingleLine()
		})
	}
	ui.Row(c).Key("pill-postures").AlignItems(ui.Center).Gap(2 * k).Children(func() {
		if len(s.Open) > 0 {
			chip("pill-glance", "Glance", s.posture == stage.PostureGlance, func() {
				s.SetPosture(stage.PostureGlance)
			})
		}
		chip("pill-arrive", "Explore", s.posture == stage.PostureArrive, func() {
			s.SetPosture(stage.PostureArrive)
		})
		if len(s.Open) > 0 {
			chip("pill-desk", "Desk", false, s.ReturnToDesk)
		}
	})
}
