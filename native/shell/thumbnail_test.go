package shell

import (
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// pictureLibrary is the fake library with Chapter holding a picture.
type pictureLibrary struct{ *fakeLibrary }

func (f pictureLibrary) Sheets(place Place) []SheetItem {
	sheets := f.fakeLibrary.Sheets(place)
	for i := range sheets {
		if sheets[i].Title == "Chapter" {
			sheets[i].Picture = "cafe"
		}
	}
	return sheets
}

// A sheet with a picture shows it small on its card, asking for it by its
// media, and its row keeps the picture's room whether or not it has come.
func TestACardShowsItsSheetsFirstPicture(t *testing.T) {
	state, library := libraryState()
	state.Library = pictureLibrary{library}
	var asked []string
	state.Thumbnail = func(media string, _ time.Time) (*ui.Bitmap, bool) {
		asked = append(asked, media)
		return nil, true
	}
	state.ShowField()
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	if !slices.Contains(asked, "cafe") {
		t.Fatalf("Chapter's card asked for %q, not its picture.", asked)
	}
	m := blockMetrics{scale: 14, k: 1, col: 240}
	bare := entry{sheet: SheetItem{Title: "Chapter"}}
	pictured := bare
	pictured.sheet.Picture = "cafe"
	// Without an opening, no text is measured.
	if got, want := entryHeight(nil, pictured, m, 2), entryHeight(nil, bare, m, 2)+rowGap(14)+thumbSide(m, 2); got != want {
		t.Errorf("a row with a picture and no opening stands %v tall, want %v", got, want)
	}
}
