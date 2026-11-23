package clipboard

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/caelyreth/neverland/internal/editor/profile"
)

// A Markdown picture alone on its line names a file on this computer; one
// on the web, one by a relative path, one amid text or not a picture the
// notebook keeps stays text.
func TestPictureTextReadsLocalMarkdownPictures(t *testing.T) {
	home, _ := os.UserHomeDir()
	cases := []struct {
		line, path, alt string
	}{
		{"![Bay](/photos/bay.png)", "/photos/bay.png", "Bay"},
		{"  ![](/photos/bay.jpg \"At dawn\")  ", "/photos/bay.jpg", ""},
		{"![Two words](</my photos/bay.png>)", "/my photos/bay.png", "Two words"},
		{"![Bay](file:///photos/b%20ay.webp)", "/photos/b ay.webp", "Bay"},
		{"![Bay](~/bay.gif)", filepath.Join(home, "bay.gif"), "Bay"},
		{"![Bay](https://example.com/bay.png)", "", ""},
		{"![Bay](photos/bay.png)", "", ""},
		{"See ![Bay](/photos/bay.png)", "", ""},
		{"![Bay](/photos/bay.txt)", "", ""},
		{"![Bay](/photos/bay.png) after", "", ""},
		{"![Bay](file://server/bay.png)", "", ""},
	}
	for _, c := range cases {
		lines, ok := PictureText(c.line)
		if ok != (c.path != "") || len(lines) != 1 || lines[0].Path != c.path || lines[0].Alt != c.alt || lines[0].Text != c.line {
			t.Errorf("%q: %+v %v, want path %q alt %q", c.line, lines, ok, c.path, c.alt)
		}
	}
}

// A picture kept is a figure captioned by its alt text; the edges open
// only on text.
func TestPictureSliceCaptionsWhatWasKept(t *testing.T) {
	lines, _ := PictureText("![Bay](/bay.png)\nLow tide")
	slice := PictureSlice(profile.Schema(), lines, []profile.Picture{{Media: "m", Width: 4, Height: 3}})
	if len(slice.Nodes) != 2 || slice.Nodes[0].Kind() != profile.Figure || slice.Nodes[0].Slot(0).Content().Text() != "Bay" ||
		profile.ShapeOf(slice.Nodes[0]).Media != "m" || slice.OpenStart != 0 || slice.OpenEnd != 1 {
		t.Fatalf("slice %+v", slice)
	}
}
