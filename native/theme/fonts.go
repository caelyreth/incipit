package theme

import (
	_ "embed"
	"sync"

	"github.com/egoist/mygo/ui"
)

//go:generate ../../scripts/fetch-assets.sh

//go:embed fonts/NotoSerifSC-Variable.ttf
var proseFont []byte

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

// RegisterFonts installs the product prose font under Neverland's stable
// family name on every target platform.
func RegisterFonts() error {
	registerOnce.Do(func() {
		registerErr = ui.RegisterFont(proseFont, ProseFontFamily)
	})
	return registerErr
}
