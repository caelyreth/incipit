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
)

// Manifest lists the product's policies.
//
// TODO(policies): wave one adds typography, cjk, scopes, deletion,
// words, feedback and statistics, with their precedence (markdown over
// pairs for a typed '*' once pairs spell it); wave two tables and figures.
var Manifest = policy.Manifest{
	Enabled: []*policy.Policy{core.Policy, notation.Policy, pairs.Policy, markdown.Policy, lists.Policy,
		navigation.Policy, history.Policy, guides.Policy},
}

// Compile builds the program for settings values.
//
// TODO(strict): the product compiles with strict=false, so declared Produces
// is checked only by tests. Add a debug-build switch (§11.14).
func Compile(values policy.Values, strict bool) (*policy.Program, error) {
	m := Manifest
	m.Strict = strict
	return policy.Compile(m, values)
}
