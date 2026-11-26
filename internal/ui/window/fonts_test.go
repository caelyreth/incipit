package window

import (
	"testing"

	"github.com/caelyreth/neverland/internal/app/fonts"
	"github.com/caelyreth/neverland/internal/ui/settings"
)

func TestFontStateFamilyResolution(t *testing.T) {
	fs := openFonts("")
	if got := fs.family(fonts.Serif, "", ""); got != "serif" {
		t.Fatalf("default serif = %q, want %q", got, "serif")
	}
	if got := fs.family(fonts.Sans, "", ""); got != "sans-serif" {
		t.Fatalf("default sans = %q, want %q", got, "sans-serif")
	}
	// Unkept managed Noto falls back to system generic until downloaded.
	if got := fs.family(fonts.Serif, "Noto Serif", ""); got != "serif" {
		t.Fatalf("unkept Noto Serif = %q, want %q", got, "serif")
	}
	// User-installed system font works immediately without managed download.
	if got := fs.family(fonts.Serif, "Georgia", ""); got != "Georgia, serif" {
		t.Fatalf("custom system font = %q, want %q", got, "Georgia, serif")
	}
	// Once kept, managed Latin and CJK chain cleanly before system fallback.
	fs.kept["serif-latin"] = true
	fs.kept["serif-sc"] = true
	if got := fs.family(fonts.Serif, "Noto Serif", "sc"); got != "Noto Serif, Noto Serif SC, serif" {
		t.Fatalf("kept Noto Serif + SC = %q", got)
	}
	if got := fs.family(fonts.Serif, "Georgia", "sc"); got != "Georgia, Noto Serif SC, serif" {
		t.Fatalf("custom font + SC = %q", got)
	}
	if got := fs.family(fonts.Sans, "", "sc"); got != "sans-serif" {
		t.Fatalf("style mismatch sc under sans = %q, want sans-serif", got)
	}
}

func TestFontActLatinAndRemoveResetsActiveFont(t *testing.T) {
	app, err := OpenAppState(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.fonts = openFonts(t.TempDir())
	app.fonts.kept["serif-latin"] = true
	app.refreshSettingsData()

	app.fontAct("serif-latin", settings.FontChoose)
	if got := app.Product.Appearance.ProseFont; got != "Noto Serif" {
		t.Fatalf("after choose kept serif-latin, ProseFont = %q, want %q", got, "Noto Serif")
	}

	app.fontAct("serif-latin", settings.FontRemove)
	if got := app.Product.Appearance.ProseFont; got != "" {
		t.Fatalf("after remove serif-latin, ProseFont = %q, want empty", got)
	}
	if app.fonts.kept["serif-latin"] {
		t.Fatal("serif-latin still marked kept after removal")
	}
}
