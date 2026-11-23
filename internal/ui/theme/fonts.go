package theme

import (
	_ "embed"
	"errors"
	"sync"

	"github.com/egoist/mygo/ui"
)

//go:generate ../../../scripts/fetch-assets.sh

//go:embed fonts/NotoSerif-Variable.ttf
var proseFont []byte

//go:embed fonts/NotoSerif-Italic-Variable.ttf
var proseItalic []byte

//go:embed fonts/NotoSans-Variable.ttf
var proseSans []byte

//go:embed fonts/NotoSans-Italic-Variable.ttf
var proseSansItalic []byte

//go:embed fonts/NewCMMath-Book.otf
var mathFont []byte

// MathFont is the face formulas are set in: New Computer Modern Math, an
// OpenType MATH font the formula renderer reads as bytes, never through the
// platform's text engine.
func MathFont() []byte { return mathFont }

var (
	registerOnce sync.Once
	registerErr  error
)

// RegisterFonts installs the prose faces, Serif and Sans, upright and
// italic, under their stable family names on every target platform.
func RegisterFonts() error {
	registerOnce.Do(func() {
		registerErr = errors.Join(
			ui.RegisterFont(proseFont, ProseFontFamily), ui.RegisterFont(proseItalic, ProseFontFamily),
			ui.RegisterFont(proseSans, ProseSansFamily), ui.RegisterFont(proseSansItalic, ProseSansFamily))
	})
	return registerErr
}
