// Package manifest is the composition root: the only code that names more
// than one policy, and orders them.
package manifest

import (
	"github.com/caelyreth/neverland/editor/policies/core"
	"github.com/caelyreth/neverland/editor/policies/history"
	"github.com/caelyreth/neverland/editor/policies/markdown"
	"github.com/caelyreth/neverland/editor/policies/navigation"
	"github.com/caelyreth/neverland/editor/policy"
)

// Manifest lists the product's policies.
//
// TODO(policies): wave one adds notation, pairs, typography, cjk, scopes,
// deletion, lists, words, feedback, guides and statistics, with their
// precedence (pairs over notation for Pair; markdown over pairs for a
// typed '*'); wave two tables and figures.
var Manifest = policy.Manifest{
	Enabled: []*policy.Policy{core.Policy, markdown.Policy, navigation.Policy, history.Policy},
}

// Compile builds the program for settings values.
func Compile(values policy.Values, strict bool) (*policy.Program, error) {
	m := Manifest
	m.Strict = strict
	return policy.Compile(m, values)
}
