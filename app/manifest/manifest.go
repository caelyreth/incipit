// Package manifest is the composition root: the only code that names more
// than one policy, and orders them.
package manifest

import (
	"github.com/caelyreth/neverland/editor/policies/core"
	"github.com/caelyreth/neverland/editor/policies/history"
	"github.com/caelyreth/neverland/editor/policies/markdown"
	"github.com/caelyreth/neverland/editor/policies/navigation"
	"github.com/caelyreth/neverland/editor/policies/notation"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/present"
)

// Manifest lists the product's policies.
//
// TODO(policies): wave one adds pairs, typography, cjk, scopes,
// deletion, lists, words, feedback, guides and statistics, with their
// precedence (pairs over notation for Pair; markdown over pairs for a
// typed '*'); wave two tables and figures.
var Manifest = policy.Manifest{
	Enabled: []*policy.Policy{core.Policy, notation.Policy, markdown.Policy, navigation.Policy, history.Policy},
}

// PresentOptions are a view's presentation inputs as settings values
// choose them: the notation policy's reveal mode and ghost layout.
func PresentOptions(values policy.Values) present.Options {
	reveal, layout := notation.View(values)
	var o present.Options
	switch reveal {
	case notation.RevealShown:
		o.Reveal = present.RevealShown
	case notation.RevealHidden:
		o.Reveal = present.RevealHidden
	default:
		o.Reveal = present.RevealNearCaret
	}
	if layout == notation.LayoutOverlay {
		o.Ghosts = present.GhostOverlay
	}
	return o
}

// Compile builds the program for settings values.
func Compile(values policy.Values, strict bool) (*policy.Program, error) {
	m := Manifest
	m.Strict = strict
	return policy.Compile(m, values)
}
