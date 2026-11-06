package textlayout

import (
	"testing"

	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func TestRoleColorFollowsTheNativeTheme(t *testing.T) {
	for _, appearance := range []theme.Appearance{theme.Starfield, theme.Daybreak} {
		settings := theme.DefaultSettings()
		settings.Appearance = appearance
		semantic := settings.Semantic()
		for _, test := range []struct {
			role surface.ColorRole
			want ui.Color
		}{
			{surface.ColorText, semantic.Prose.Text},
			{surface.ColorMuted, semantic.Prose.Notation},
			{surface.ColorStrong, semantic.Prose.Strong},
			{surface.ColorAccent, semantic.Accent.Foreground},
			{surface.ColorSubtle, semantic.Prose.Subtle},
			{surface.ColorRole(255), semantic.Prose.Text},
		} {
			if got := RoleColor(semantic, test.role); got != test.want {
				t.Fatalf("appearance %v role %v = %v, want %v", appearance, test.role, got, test.want)
			}
		}
	}
}

// The page sets each size a style's role says: titles a step down per
// level from the largest, the page's text below them, a caption below
// that, and code a little smaller than the text it stands in, a title's
// code larger than the page's.
func TestTextSizesFollowTheirRoles(t *testing.T) {
	style := testStyle()
	size := func(text surface.TextStyle) float32 { return style.textSize(text) }
	body := size(surface.TextStyle{})
	if body != style.Prose.FontSize {
		t.Fatalf("the page's text is set at %v, not %v", body, style.Prose.FontSize)
	}
	previous := float32(0)
	for role := surface.SizeTitle4; role >= surface.SizeTitle1; role-- {
		title := size(surface.TextStyle{Size: role})
		if title <= body || title <= previous {
			t.Fatalf("title %d is %v, after %v, the text %v", role, title, previous, body)
		}
		previous = title
	}
	if caption := size(surface.TextStyle{Size: surface.SizeCaption}); caption >= body {
		t.Fatalf("a caption is %v, the text %v", caption, body)
	}
	code := size(surface.TextStyle{Face: surface.FaceMono})
	titleCode := size(surface.TextStyle{Face: surface.FaceMono, Size: surface.SizeTitle1})
	if code >= body || titleCode >= previous || titleCode <= code || style.codeSize() != code {
		t.Fatalf("code is %v in the text, %v in a title of %v", code, titleCode, previous)
	}
}
