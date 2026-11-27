package keymap

import "github.com/caelyreth/incipit/internal/app/manifest"

// declared is the product's commands.
func declared() []Command {
	prog, err := manifest.Compile(nil, false)
	if err != nil {
		panic(err)
	}
	return Declared(prog.Catalog())
}

// defaulted are the declared commands with a default key.
func defaulted() []Command {
	var out []Command
	for _, c := range declared() {
		if c.Default.Valid() {
			out = append(out, c)
		}
	}
	return out
}
