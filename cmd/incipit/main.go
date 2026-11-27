package main

// The icon of Windows, Linux and the development app is drawn from the
// Icon Composer document that macOS shows (mygo.json); Git keeps only the
// document.
//go:generate go tool icon ../../assets/profile.icon ../../assets/icon.png

import (
	"cmp"
	"fmt"
	"os"

	"github.com/caelyreth/incipit/internal/app/manifest"
	"github.com/caelyreth/incipit/internal/ui/window"
	"github.com/egoist/mygo"
)

func main() {
	// The version is mygo.json's, which a packaged build carries; a build
	// of go run has none.
	if len(os.Args) == 2 && os.Args[1] == "version" {
		fmt.Println(cmp.Or(mygo.App.Version(), "dev"))
		return
	}
	// A development build checks what the editor's rewrites declare.
	manifest.Checked = mygo.IsDev()
	window.MustRun()
}
