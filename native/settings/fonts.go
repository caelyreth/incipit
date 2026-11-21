package settings

import (
	"strconv"
	"strings"

	"github.com/caelyreth/neverland/app/fonts"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// FontRow is a script's downloadable face as Writing Systems lists it:
// whether it is kept, or coming and how far, how much a download stopped
// or failed left to go on from, and why its last download failed.
type FontRow struct {
	Face        fonts.Face
	Kept        bool
	Coming      bool
	Done, Total int64
	Partial     int64
	Failed      string
}

// FontAct is what a row asks of its face.
type FontAct uint8

const (
	FontDownload FontAct = iota + 1
	FontCancel
	FontRemove
)

// writingSystems is the section's title.
const writingSystems = "Writing Systems"

// writingSystemsInfo is the section as it is found.
func (s *State) writingSystemsInfo() sectionInfo {
	words := writingSystems + " Fonts Faces Scripts Download Google Fonts Han"
	for _, row := range s.Fonts {
		words += " " + row.Face.Name + " " + row.Face.Family
	}
	return sectionInfo{writingSystems,
		"Serif faces for scripts beyond Latin, downloaded from Google Fonts only when you ask, and kept on this computer. A script without one is drawn in the system's fonts.",
		words}
}

// writingSystemsSection draws the section: the Han choice, when more than
// one kept face draws Han characters, then a row for each face.
func (s *State) writingSystemsSection(c *ui.Context, semantic theme.Theme, sec Section) {
	s.section(c, semantic, sec, func() {
		var han []fonts.Face
		for _, row := range s.Fonts {
			if row.Kept && row.Face.Han {
				han = append(han, row.Face)
			}
		}
		if len(han) > 1 {
			s.hanRow(c, semantic, han)
		}
		for i, row := range s.Fonts {
			s.fontRow(c, semantic, row, i == len(s.Fonts)-1)
		}
	})
}

// hanRow chooses which kept face draws the Han characters the faces share.
func (s *State) hanRow(c *ui.Context, semantic theme.Theme, han []fonts.Face) {
	chosen := han[0].ID
	for _, f := range han {
		if f.ID == s.HanFace {
			chosen = f.ID
		}
	}
	components.SettingRow{
		Label:       "Han Characters",
		Description: "Chinese, Japanese and Korean draw many of the same characters each in their own way; text in any of them uses this one's.",
		Control: func() {
			options := make([]components.SegmentOption, 0, len(han))
			for _, f := range han {
				id := f.ID
				options = append(options, components.SegmentOption{
					Key: "han-" + id, Label: hanLabel(f), Selected: id == chosen,
					Action: func() {
						s.apply(appsettings.NewAppearanceChange(appsettings.AppearanceChange{
							Kind: appsettings.SetHanFace, Face: id,
						}))
					},
				})
			}
			components.Segmented{Key: "han-face", Label: "Han Characters", Options: options, Animate: s.motion(c)}.View(c, semantic)
		},
	}.View(c, semantic)
}

// hanLabel is a Han face's short name, its script's without the language.
func hanLabel(f fonts.Face) string {
	if _, variant, ok := strings.Cut(f.Name, ", "); ok {
		return variant
	}
	return f.Name
}

// fontRow is a face's row: its script, a line in its face and its size,
// and what can be done with it.
func (s *State) fontRow(c *ui.Context, semantic theme.Theme, row FontRow, last bool) {
	f := row.Face
	scale := s.Theme.UIFontSize
	sample := theme.ProseFontFamily
	if row.Kept {
		sample = f.Family
	}
	components.SettingRow{
		Label: f.Name,
		Last:  last,
		DescriptionText: func() {
			ui.Row(c).Key("sample").Gap(theme.Space08.DIPs(scale)).AlignItems(ui.Baseline).MinWidth(0).Children(func() {
				// The line is in the face once kept, else as the system
				// draws the script.
				ui.Text(c, f.Sample).Font(sample).FontSize(theme.TextDefault.Size(scale)).
					TextColor(semantic.Text.Default).SingleLine().MinWidth(0).Shrink(1)
				components.Label(c, megabytes(f.Size()), theme.TextXSmall, semantic.Text.Muted).
					SingleLine().Shrink(0).FontFeatures("tnum")
			})
		},
		Control: func() { s.fontControl(c, semantic, row) },
		Footer: func() {
			if row.Failed != "" {
				components.Label(c, row.Failed, theme.TextSmall, semantic.Status.Danger.Foreground).
					Padding(0, theme.Space16.DIPs(scale), theme.Space12.DIPs(scale), theme.Space16.DIPs(scale))
			}
		},
	}.View(c, semantic)
}

// fontControl is a row's control: Download, the download coming and Stop,
// or Remove.
func (s *State) fontControl(c *ui.Context, semantic theme.Theme, row FontRow) {
	f := row.Face
	act := func(a FontAct) func() {
		return func() {
			if s.Font != nil {
				s.Font(f.ID, a)
			}
		}
	}
	scale := s.Theme.UIFontSize
	switch {
	case row.Coming:
		ui.Row(c).Key("coming").Gap(theme.Space08.DIPs(scale)).AlignItems(ui.Center).Children(func() {
			done := float32(0)
			if row.Total > 0 {
				done = float32(row.Done) / float32(row.Total)
			}
			components.ProgressRing(c, "progress", 16*scale/theme.DefaultUIFontSize, 2*scale/theme.DefaultUIFontSize, done,
				semantic.Border.Variant, semantic.Text.Muted, "Downloading "+f.Name+", "+strconv.Itoa(int(done*100))+"%")
			components.Button{Key: "stop", Label: "Stop", Style: components.SubtleButton, Size: components.CompactButton,
				Animate: s.motion(c), Action: act(FontCancel)}.View(c, semantic)
		})
	case row.Kept:
		components.Button{Key: "remove", Label: "Remove", Style: components.SubtleButton, Size: components.CompactButton,
			Animate: s.motion(c), Action: act(FontRemove)}.View(c, semantic)
	default:
		label := "Download"
		if row.Partial > 0 {
			label = "Resume"
		}
		components.Button{Key: "download", Label: label, Style: components.OutlinedButton, Size: components.CompactButton,
			Animate: s.motion(c), Action: act(FontDownload)}.View(c, semantic)
	}
}

// megabytes is n bytes in megabytes, to a tenth.
func megabytes(n int64) string {
	return strconv.FormatFloat(float64(n)/1e6, 'f', 1, 64) + " MB"
}
