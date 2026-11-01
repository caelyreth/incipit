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
