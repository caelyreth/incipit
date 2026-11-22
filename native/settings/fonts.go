package settings

import (
	"slices"
	"strconv"

	"github.com/caelyreth/neverland/app/fonts"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// FontRow is a face for Chinese and Japanese as the Typeface section
// offers it: whether it is kept, or coming and how far, how much a
// download stopped or failed left to go on from, why its last download
// failed, and whether it was chosen while not kept, to be drawn once it
// is.
type FontRow struct {
	Face        fonts.Face
	Kept        bool
	Coming      bool
	Done, Total int64
	Partial     int64
	Failed      string
	Wanted      bool
}

// FontAct is what the section asks of a face.
type FontAct uint8

const (
	// FontChoose has the face draw its glyph forms, downloading it first
	// when it is not kept; for no face, the system's fonts draw them.
	FontChoose FontAct = iota + 1
	FontCancel
	FontRemove
)

// typefaceTitle is the section's title.
const typefaceTitle = "Typeface"

// systemFont is what the language picker calls the system's fonts.
const systemFont = "System Font"

// typefaceInfo is the section as it is found.
func (s *State) typefaceInfo() sectionInfo {
	words := typefaceTitle + " Fonts Faces Style Serif Sans Writing Systems Download Google Fonts Chinese Japanese Han Kanji Noto " + systemFont
	for _, f := range fonts.Languages() {
		words += " " + f.Language
	}
	return sectionInfo{typefaceTitle,
		"The style the prose is set in, and the glyph forms of Chinese and Japanese. A face for them downloads once from Google Fonts and stays on this computer.",
		words}
}

// typefaceSection draws the section: the style, the glyph forms of
// Chinese and Japanese as a list of options, each with its face's state in
// the style chosen, then the faces held of the other style.
func (s *State) typefaceSection(c *ui.Context, semantic theme.Theme, sec Section) {
	scale := s.Theme.UIFontSize
	s.section(c, semantic, sec, func() {
		style := s.Data.AppearanceValues.Typeface
		var other []FontRow
		for _, row := range s.Fonts {
			if row.Face.Style != style && (row.Kept || row.Coming || row.Partial > 0) {
				other = append(other, row)
			}
		}
		s.styleRow(c, semantic)
		components.SettingRow{
			Label:       "Chinese and Japanese",
			Description: "The glyph forms they are drawn in. A face downloads when chosen.",
			Last:        true,
		}.View(c, semantic)
		ui.Column(c).Key("forms").WidthPercent(100).Padding(0, 0, theme.Space08.DIPs(scale), 0).Children(func() {
			s.formsOption(c, semantic, "", nil)
			for _, lang := range fonts.Languages() {
				if row := s.row(style, lang.Forms); row != nil {
					s.formsOption(c, semantic, lang.Forms, row)
				}
			}
		})
		if len(other) > 0 {
			components.SettingRow{Label: "Also on This Computer", Description: "Faces of the other style."}.View(c, semantic)
		}
		for i, row := range other {
			s.heldRow(c, semantic, row, i == len(other)-1)
		}
	})
}

// row is the row of the face of forms in style.
func (s *State) row(style fonts.Style, forms string) *FontRow {
	for i := range s.Fonts {
		if f := s.Fonts[i].Face; f.Style == style && f.Forms == forms {
			return &s.Fonts[i]
		}
	}
	return nil
}

// choose asks for the face of forms in style, the system's fonts for none.
func (s *State) choose(style fonts.Style, forms string) {
	if s.Font == nil {
		return
	}
	id := ""
	if row := s.row(style, forms); row != nil {
		id = row.Face.ID
	}
	s.Font(id, FontChoose)
}

// styleRow chooses Serif or Sans, for the Latin face at once and for the
// face of the glyph forms chosen, downloaded first if not kept.
func (s *State) styleRow(c *ui.Context, semantic theme.Theme) {
	values := s.Data.AppearanceValues
	components.SettingRow{
		Label:       "Style",
		Description: "Latin text is set in Noto Serif or Noto Sans, and Chinese and Japanese in its match.",
		Control: func() {
			var options []components.SegmentOption
			for _, style := range []fonts.Style{fonts.Serif, fonts.Sans} {
				options = append(options, components.SegmentOption{
					Key: "style-" + style.String(), Label: style.String(), Selected: values.Typeface == style,
					Action: func() {
						if style == values.Typeface {
							return
						}
						s.apply(appsettings.NewAppearanceChange(appsettings.AppearanceChange{
							Kind: appsettings.SetTypeface, Typeface: style,
						}))
						if values.CJKForms != "" {
							s.choose(style, values.CJKForms)
						}
					},
				})
			}
			components.Segmented{Key: "style-choice", Label: "Style", Options: options, Animate: s.motion(c)}.View(c, semantic)
		},
	}.View(c, semantic)
}

// formsOption is one option of the glyph forms: the system's fonts for a
// nil row, else a language and its face in the style chosen, how far its
// download has come, and what can be done with it. Pressed, it is chosen.
func (s *State) formsOption(c *ui.Context, semantic theme.Theme, forms string, row *FontRow) {
	scale := s.Theme.UIFontSize
	k := scale / theme.DefaultUIFontSize
	values := s.Data.AppearanceValues
	// The face chosen while not kept stands for the choice until it is.
	wanting := slices.ContainsFunc(s.Fonts, func(r FontRow) bool { return r.Wanted })
	chosen := !wanting && values.CJKForms == forms
	if row == nil {
		// The system's fonts draw while the forms chosen have no face kept.
		drawn := s.row(values.Typeface, values.CJKForms)
		chosen = !wanting && (drawn == nil || !drawn.Kept)
	}
	label, note := systemFont, "The system's own, nothing to download"
	if row != nil {
		f := row.Face
		label = f.Language
		switch {
		case row.Coming:
			note = f.Family + " · " + megabytes(row.Done) + " of " + megabytes(row.Total)
		case row.Kept:
			note = f.Family + " · " + megabytes(f.Size()) + " · Downloaded"
		case row.Partial > 0:
			note = f.Family + " · " + megabytes(row.Partial) + " of " + megabytes(f.Size()) + " downloaded"
		default:
			note = f.Family + " · " + megabytes(f.Size())
		}
		chosen = chosen && row.Kept || row.Wanted
	}
	pad := theme.Space16.DIPs(scale)
	ui.Column(c.Key("option-" + forms)).WidthPercent(100).Children(func() {
		ui.Row(c).WidthPercent(100).AlignItems(ui.Center).Gap(theme.Space08.DIPs(scale)).
			Padding(0, pad-theme.Space08.DIPs(scale), 0, pad-theme.Space08.DIPs(scale)).Children(func() {
			option := ui.ButtonBase(c.Key("choose")).Role(ui.RoleRadio).Label(label).Checked(chosen).
				Grow(1).MinWidth(0).AlignItems(ui.Center).Gap(theme.Space12.DIPs(scale)).
				Padding(theme.Space08.DIPs(scale), theme.Space08.DIPs(scale), theme.Space08.DIPs(scale), theme.Space08.DIPs(scale)).
				Radius(theme.DefaultControls.Scaled(scale).Radius).FocusRing(true)
			if option.Hovered() && !chosen {
				option.Background(semantic.GhostElement.Hover)
			}
			option.Children(func() {
				if row != nil && row.Coming {
					s.progress(c, semantic, *row)
				} else {
					radio(c, semantic, chosen, k)
				}
				ui.Column(c).Grow(1).MinWidth(0).Gap(theme.Space02.DIPs(scale)).Children(func() {
					components.Label(c, label, theme.TextDefault, semantic.Text.Default).SingleLine()
					components.Label(c, note, theme.TextSmall, semantic.Text.Muted).SingleLine().FontFeatures("tnum")
				})
			})
			if option.Clicked() && !chosen {
				s.choose(values.Typeface, forms)
			}
			if row != nil {
				ui.Row(c).Key("actions").Gap(theme.Space08.DIPs(scale)).AlignItems(ui.Center).Shrink(0).Children(func() {
					s.optionActions(c, semantic, *row)
				})
			}
		})
		if row != nil && row.Failed != "" {
			s.failure(c, semantic, row.Failed)
		}
	})
}

// optionActions are what can be done with an option's face beside
// choosing it: Stop its download, or Remove what is kept of it.
func (s *State) optionActions(c *ui.Context, semantic theme.Theme, row FontRow) {
	act := func(a FontAct) func() {
		return func() {
			if s.Font != nil {
				s.Font(row.Face.ID, a)
			}
		}
	}
	switch {
	case row.Coming:
		components.Button{Key: "stop", Label: "Stop", Style: components.SubtleButton, Size: components.CompactButton,
			Animate: s.motion(c), Action: act(FontCancel)}.View(c, semantic)
	case row.Kept || row.Partial > 0:
		components.Button{Key: "remove", Label: "Remove", Style: components.SubtleButton, Size: components.CompactButton,
			Animate: s.motion(c), Action: act(FontRemove)}.View(c, semantic)
	}
}

// radio is an option's mark: a ring, filled with the accent's dot when
// chosen.
func radio(c *ui.Context, semantic theme.Theme, on bool, k float32) {
	size := 16 * k
	ring := ui.Box(c.Key("radio")).Size(size, size).Shrink(0).Radius(size / 2).AlignItems(ui.Center).Justify(ui.Center)
	if !on {
		ring.Border(1.5*k, semantic.Border.Variant)
		return
	}
	ring.Background(semantic.Accent.Foreground).Children(func() {
		ui.Box(c.Key("dot")).Size(6*k, 6*k).Radius(3 * k).Background(semantic.Accent.OnAccent)
	})
}

// heldRow is a face of the style not chosen this computer holds, whole or
// in part: its language, how much it takes, and what can be done with it.
func (s *State) heldRow(c *ui.Context, semantic theme.Theme, row FontRow, last bool) {
	f := row.Face
	scale := s.Theme.UIFontSize
	note := f.Language + " · " + megabytes(f.Size())
	switch {
	case row.Coming:
		note = f.Language + " · " + megabytes(row.Done) + " of " + megabytes(row.Total)
	case !row.Kept:
		note = f.Language + " · " + megabytes(row.Partial) + " of " + megabytes(f.Size()) + " downloaded"
	}
	components.SettingRow{
		Label:       f.Family,
		Description: note,
		Indent:      theme.Space16.DIPs(scale),
		Last:        last,
		Control: func() {
			ui.Row(c).Key("held-" + f.ID).Gap(theme.Space08.DIPs(scale)).AlignItems(ui.Center).Children(func() {
				s.heldControl(c, semantic, row)
			})
		},
		Footer: func() {
			if row.Failed != "" && !row.Wanted {
				s.failure(c, semantic, row.Failed)
			}
		},
	}.View(c, semantic)
}

// heldControl is a held face's control: the download coming and Stop, or
// Resume and Remove for one left part way, or Remove.
func (s *State) heldControl(c *ui.Context, semantic theme.Theme, row FontRow) {
	act := func(a FontAct) func() {
		return func() {
			if s.Font != nil {
				s.Font(row.Face.ID, a)
			}
		}
	}
	button := func(key, label string, style components.ButtonStyle, a FontAct) {
		components.Button{Key: key, Label: label, Style: style, Size: components.CompactButton,
			Animate: s.motion(c), Action: act(a)}.View(c, semantic)
	}
	switch {
	case row.Coming:
		if !row.Wanted {
			s.progress(c, semantic, row)
		}
		button("stop", "Stop", components.SubtleButton, FontCancel)
	case !row.Kept:
		button("resume", "Resume", components.OutlinedButton, FontChoose)
		button("remove", "Remove", components.SubtleButton, FontRemove)
	default:
		button("remove", "Remove", components.SubtleButton, FontRemove)
	}
}

// progress is how far a face's download has come.
func (s *State) progress(c *ui.Context, semantic theme.Theme, row FontRow) {
	scale := s.Theme.UIFontSize
	done := float32(0)
	if row.Total > 0 {
		done = float32(row.Done) / float32(row.Total)
	}
	k := scale / theme.DefaultUIFontSize
	components.ProgressRing(c, "progress-"+row.Face.ID, 16*k, 2*k, done, semantic.Border.Variant, semantic.Text.Muted,
		"Downloading "+row.Face.Family+", "+strconv.Itoa(int(done*100))+"%")
}

// failure is why a download did not finish, under its row.
func (s *State) failure(c *ui.Context, semantic theme.Theme, why string) {
	scale := s.Theme.UIFontSize
	components.Label(c, why, theme.TextSmall, semantic.Status.Danger.Foreground).
		Padding(0, theme.Space16.DIPs(scale), theme.Space12.DIPs(scale), theme.Space16.DIPs(scale))
}

// megabytes is n bytes in megabytes, to a tenth.
func megabytes(n int64) string {
	return strconv.FormatFloat(float64(n)/1e6, 'f', 1, 64) + " MB"
}
