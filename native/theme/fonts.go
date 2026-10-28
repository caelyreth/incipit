package theme

import (
	_ "embed"
	"sync"

	"github.com/egoist/mygo/ui"
)

//go:generate ../../scripts/fetch-fonts.sh

//go:embed fonts/NotoSerifSC-Variable.ttf
var proseFont []byte

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
