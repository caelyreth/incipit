package main

// The icon of Windows, Linux and the development app is drawn from the
// Icon Composer document that macOS shows (mygo.json); Git keeps only the
// document.
//go:generate go tool icon ../../assets/profile.icon ../../assets/icon.png

import (
	"fmt"
	"os"

	"github.com/caelyreth/neverland/internal/app/manifest"
	"github.com/caelyreth/neverland/internal/ui/window"
	"github.com/egoist/mygo"
)

const version = "0.1.0-dev"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "version" {
		fmt.Println(version)
		return
	}
	// A development build checks what the editor's rewrites declare.
	manifest.Checked = mygo.IsDev()
	window.MustRun()
}
