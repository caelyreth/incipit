// Package manifest is the composition root: the only code that names more
// than one policy, and orders them.
package manifest

import (
	"github.com/caelyreth/neverland/editor/policies/cjk"
	"github.com/caelyreth/neverland/editor/policies/core"
	"github.com/caelyreth/neverland/editor/policies/deletion"
	"github.com/caelyreth/neverland/editor/policies/feedback"
	"github.com/caelyreth/neverland/editor/policies/guides"
	"github.com/caelyreth/neverland/editor/policies/history"
	"github.com/caelyreth/neverland/editor/policies/lists"
	"github.com/caelyreth/neverland/editor/policies/markdown"
	"github.com/caelyreth/neverland/editor/policies/navigation"
	"github.com/caelyreth/neverland/editor/policies/notation"
	"github.com/caelyreth/neverland/editor/policies/pairs"
	"github.com/caelyreth/neverland/editor/policies/scopes"
	"github.com/caelyreth/neverland/editor/policies/statistics"
	"github.com/caelyreth/neverland/editor/policies/typography"
	"github.com/caelyreth/neverland/editor/policies/words"
	"github.com/caelyreth/neverland/editor/policy"
)

// Manifest lists the product's policies.
//
// TODO(policies): tables and figures remain. Once pairs spell a typed '*',
// markdown must take precedence over pairs for that character.
var Manifest = policy.Manifest{
	Enabled: []*policy.Policy{core.Policy, notation.Policy, pairs.Policy, markdown.Policy,
		typography.Policy, cjk.Policy, navigation.Policy, scopes.Policy, deletion.Policy,
		lists.Policy, words.Policy, history.Policy, feedback.Policy, guides.Policy, statistics.Policy},
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
