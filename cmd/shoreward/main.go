package main

import (
	"fmt"
	"os"

	"github.com/caelyreth/neverland/native/window"
)

const version = "0.1.0-dev"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "version" {
		fmt.Println(version)
		return
	}
	window.MustRun()
}
