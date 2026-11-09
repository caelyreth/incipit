// Package manifest is the composition root: the only code that names more
// than one policy, and orders them.
package manifest

import (
	"github.com/caelyreth/neverland/editor/policies/core"
	"github.com/caelyreth/neverland/editor/policies/guides"
	"github.com/caelyreth/neverland/editor/policies/history"
	"github.com/caelyreth/neverland/editor/policies/lists"
	"github.com/caelyreth/neverland/editor/policies/markdown"
	"github.com/caelyreth/neverland/editor/policies/navigation"
	"github.com/caelyreth/neverland/editor/policies/notation"
	"github.com/caelyreth/neverland/editor/policies/pairs"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/present"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Manifest lists the product's policies.
//
// TODO(policies): wave one adds typography, cjk, scopes, deletion,
// words, feedback and statistics, with their precedence (markdown over
// pairs for a typed '*' once pairs spell it); wave two tables and figures.
var Manifest = policy.Manifest{
	Enabled: []*policy.Policy{core.Policy, notation.Policy, pairs.Policy, markdown.Policy, lists.Policy,
		navigation.Policy, history.Policy, guides.Policy},
	Resolve: []policy.Order{
		// Pair behaviour refines ghost notation where both are pairs.
		policy.Over(pairs.Policy, notation.Policy, vocab.RoleScope(vocab.RolePair)),
	},
}

// PresentOptions are a view's presentation inputs as settings values
// choose them: the notation policy's reveal mode and ghost layout.
func PresentOptions(values policy.Values) present.Options {
	reveal, layout := notation.View(values)
	var o present.Options
	if reveal == notation.RevealHidden {
		o.Reveal = present.RevealHidden
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
