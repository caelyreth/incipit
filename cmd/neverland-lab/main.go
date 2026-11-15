// Command neverland-lab opens the design lab: the app on a sample
// notebook of its own, beside a window that sets candidates for its look
// on it. It never opens the writer's notebook.
package main

import "github.com/caelyreth/neverland/native/window"

func main() { window.MustRunLab() }
