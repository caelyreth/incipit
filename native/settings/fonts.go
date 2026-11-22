package settings

import (
	"strconv"

	"github.com/caelyreth/neverland/app/fonts"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// FontRow is a face for Chinese and Japanese as Writing Systems offers it:
// whether it is kept, or coming and how far, how much a download stopped
// or failed left to go on from, why its last download failed, and whether
// it is the one chosen while not kept, to be drawn once it is.
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
	// FontChoose has the face draw Chinese and Japanese, downloading it
	// first when it is not kept; for no face, the system's fonts.
	FontChoose FontAct = iota + 1
	FontCancel
	FontRemove
)

// writingSystems is the section's title.
const writingSystems = "Writing Systems"

// systemFont is what the picker calls the system's fonts.
const systemFont = "System Font"

// writingSystemsInfo is the section as it is found.
func (s *State) writingSystemsInfo() sectionInfo {
	words := writingSystems + " Fonts Faces Typeface Download Google Fonts Chinese Japanese Han Kanji Noto Serif " + systemFont
	for _, row := range s.Fonts {
		words += " " + row.Face.Language + " " + row.Face.Family
	}
	return sectionInfo{writingSystems,
		"Chinese and Japanese are drawn in the system's fonts, or in a Noto Serif face you choose, which downloads once from Google Fonts and stays on this computer.",
		words}
}

// writingSystemsSection draws the section: the face chosen for Chinese and
// Japanese, then under it each face downloaded, or downloading, or left
// part way.
func (s *State) writingSystemsSection(c *ui.Context, semantic theme.Theme, sec Section) {
	s.section(c, semantic, sec, func() {
		var held []FontRow
		for _, row := range s.Fonts {
			if row.Kept || row.Coming || row.Partial > 0 {
				held = append(held, row)
			}
		}
		s.typefaceRow(c, semantic, len(held) == 0)
		for i, row := range held {
			s.heldRow(c, semantic, row, i == len(held)-1)
		}
	})
}

// chosen is the face chosen, nil for the system's fonts, and the one
// wanted, coming to be drawn once kept.
func (s *State) chosen() (chosen, wanted *FontRow) {
	for i := range s.Fonts {
		row := &s.Fonts[i]
		if row.Face.ID == s.CJKFace && row.Kept {
			chosen = row
		}
		if row.Wanted {
			wanted = row
		}
	}
	return chosen, wanted
}

// typefaceRow is the picker of the face that draws Chinese and Japanese,
// a line drawn in it, and, while a face chosen downloads, how far it has
// come.
func (s *State) typefaceRow(c *ui.Context, semantic theme.Theme, last bool) {
	scale := s.Theme.UIFontSize
	chosen, wanted := s.chosen()
	label, sample, family := systemFont, "永和九年，岁在癸丑　いろはにほへと", theme.ProseFontFamily
	if chosen != nil {
		label, sample, family = chosen.Face.Family, chosen.Face.Sample, chosen.Face.Family
	}
	components.SettingRow{
		Label: "Chinese and Japanese",
		Last:  last,
		DescriptionText: func() {
			ui.Text(c, sample).Font(family).FontSize(theme.TextDefault.Size(scale)).
				TextColor(semantic.Text.Default).SingleLine().MinWidth(0).Key("sample")
		},
		Control: func() {
			ui.Row(c).Key("typeface").Gap(theme.Space08.DIPs(scale)).AlignItems(ui.Center).Children(func() {
				if wanted != nil && wanted.Coming {
					s.progress(c, semantic, *wanted)
				}
				components.ChoiceButton{Key: "typeface-choice", Label: label, Tooltip: "Typeface for Chinese and Japanese",
					Animate: s.motion(c)}.View(c, semantic).Menu(s.typefaceMenu)
			})
		},
		Footer: func() {
			if wanted != nil && wanted.Failed != "" {
				s.failure(c, semantic, wanted.Failed)
			}
		},
	}.View(c, semantic)
}

// typefaceMenu offers the system's fonts and each face, the size of one
// not downloaded beside it.
func (s *State) typefaceMenu(m *ui.Menu) {
	chosen, _ := s.chosen()
	act := func(id string) {
		if s.Font != nil {
			s.Font(id, FontChoose)
		}
	}
	if m.Item(systemFont).ID("typeface-system").Radio(chosen == nil).Subtitle("Nothing to download").Chosen() {
		act("")
	}
	m.Separator()
	for _, row := range s.Fonts {
		item := m.Item(row.Face.Family).ID("typeface-" + row.Face.ID).
			Radio(chosen != nil && chosen.Face.ID == row.Face.ID).Subtitle(row.Face.Language)
		if !row.Kept {
			item.Trailing(megabytes(row.Face.Size()))
		}
		if item.Chosen() {
			act(row.Face.ID)
		}
	}
}

// heldRow is a face this computer holds, whole or in part, under the
// picker: its language, how much it takes and whether it is in use, and
// what can be done with it.
func (s *State) heldRow(c *ui.Context, semantic theme.Theme, row FontRow, last bool) {
	f := row.Face
	scale := s.Theme.UIFontSize
	chosen, _ := s.chosen()
	note := f.Language + " · " + megabytes(f.Size())
	switch {
	case chosen != nil && chosen.Face.ID == f.ID:
		note += " · In use"
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
