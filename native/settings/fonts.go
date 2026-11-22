package settings

import (
	"slices"
	"strconv"

	"github.com/caelyreth/neverland/app/fonts"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/locales"
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

// typefaceKey names the section.
const typefaceKey = "typeface"

// typefaceInfo is the section as it is found.
func (s *State) typefaceInfo() sectionInfo {
	words := locales.Typeface() + " " + locales.TypefaceKeywords() + " " + locales.SystemFont()
	for _, f := range fonts.Languages() {
		words += " " + formsLanguage(f.Forms)
	}
	return sectionInfo{typefaceKey, locales.Typeface(), locales.TypefaceDescription(), words}
}

// formsLanguage names the language whose glyph forms forms names.
func formsLanguage(forms string) string {
	switch forms {
	case "sc":
		return locales.FormsSc()
	case "tc":
		return locales.FormsTc()
	case "hk":
		return locales.FormsHk()
	case "jp":
		return locales.FormsJp()
	}
	return forms
}

// styleName names style as the settings offer it.
func styleName(style fonts.Style) string {
	if style == fonts.Sans {
		return locales.StyleSans()
	}
	return locales.StyleSerif()
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
			Label:       locales.ChineseAndJapanese(),
			Description: locales.ChineseAndJapaneseDescription(),
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
			components.SettingRow{Label: locales.AlsoOnThisComputer(), Description: locales.AlsoOnThisComputerDescription()}.View(c, semantic)
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
		Label:       locales.Style(),
		Description: locales.StyleDescription(),
		Control: func() {
			var options []components.SegmentOption
			for _, style := range []fonts.Style{fonts.Serif, fonts.Sans} {
				options = append(options, components.SegmentOption{
					Key: "style-" + style.String(), Label: styleName(style), Selected: values.Typeface == style,
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
			components.Segmented{Key: "style-choice", Label: locales.Style(), Options: options, Animate: s.motion(c)}.View(c, semantic)
		},
	}.View(c, semantic)
}

// formsOption is one option of the glyph forms: the system's fonts for a
// nil row, else a language and its face in the style chosen, how far its
// download has come, and what can be done with it. Pressed, it is chosen.
func (s *State) formsOption(c *ui.Context, semantic theme.Theme, forms string, row *FontRow) {
	values := s.Data.AppearanceValues
	// The face chosen while not kept stands for the choice until it is.
	wanting := slices.ContainsFunc(s.Fonts, func(r FontRow) bool { return r.Wanted })
	chosen := !wanting && values.CJKForms == forms
	if row == nil {
		// The system's fonts draw while the forms chosen have no face kept.
		drawn := s.row(values.Typeface, values.CJKForms)
		chosen = !wanting && (drawn == nil || !drawn.Kept)
	}
	label, note := locales.SystemFont(), locales.SystemFontNote()
	if row != nil {
		f := row.Face
		label = formsLanguage(f.Forms)
		switch {
		case row.Coming:
			note = locales.FontNoteComing(f.Family, megabytes(row.Done), megabytes(row.Total))
		case row.Kept:
			note = locales.FontNoteKept(f.Family, megabytes(f.Size()))
		case row.Partial > 0:
			note = locales.FontNotePartial(f.Family, megabytes(row.Partial), megabytes(f.Size()))
		default:
			note = locales.FontNote(f.Family, megabytes(f.Size()))
		}
		chosen = chosen && row.Kept || row.Wanted
	}
	o := choiceOption{key: "option-" + forms, label: label, note: note, chosen: chosen,
		choose: func() { s.choose(values.Typeface, forms) }}
	if row != nil {
		row := *row
		if row.Coming {
			o.mark = func() { s.progress(c, semantic, row) }
		}
		o.actions = func() { s.optionActions(c, semantic, row) }
		if row.Failed != "" {
			o.footer = func() { s.failure(c, semantic, row.Failed) }
		}
	}
	s.choiceView(c, semantic, o)
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
		components.Button{Key: "stop", Label: locales.Stop(), Style: components.SubtleButton, Size: components.CompactButton,
			Animate: s.motion(c), Action: act(FontCancel)}.View(c, semantic)
	case row.Kept || row.Partial > 0:
		components.Button{Key: "remove", Label: locales.Remove(), Style: components.SubtleButton, Size: components.CompactButton,
			Animate: s.motion(c), Action: act(FontRemove)}.View(c, semantic)
	}
}

// heldRow is a face of the style not chosen this computer holds, whole or
// in part: its language, how much it takes, and what can be done with it.
func (s *State) heldRow(c *ui.Context, semantic theme.Theme, row FontRow, last bool) {
	f := row.Face
	scale := s.Theme.UIFontSize
	language := formsLanguage(f.Forms)
	note := locales.FontNote(language, megabytes(f.Size()))
	switch {
	case row.Coming:
		note = locales.FontNoteComing(language, megabytes(row.Done), megabytes(row.Total))
	case !row.Kept:
		note = locales.FontNotePartial(language, megabytes(row.Partial), megabytes(f.Size()))
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
		button("stop", locales.Stop(), components.SubtleButton, FontCancel)
	case !row.Kept:
		button("resume", locales.Resume(), components.OutlinedButton, FontChoose)
		button("remove", locales.Remove(), components.SubtleButton, FontRemove)
	default:
		button("remove", locales.Remove(), components.SubtleButton, FontRemove)
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
		locales.Downloading(row.Face.Family, strconv.Itoa(int(done*100))))
}

// failure is why a download did not finish, under its row.
func (s *State) failure(c *ui.Context, semantic theme.Theme, why string) {
	scale := s.Theme.UIFontSize
	components.Label(c, why, theme.TextSmall, semantic.Status.Danger.Foreground).
		Padding(0, theme.Space16.DIPs(scale), theme.Space12.DIPs(scale), theme.Space16.DIPs(scale))
}

// megabytes is n bytes in megabytes, to a tenth.
func megabytes(n int64) string {
	return locales.Megabytes(strconv.FormatFloat(float64(n)/1e6, 'f', 1, 64))
}
