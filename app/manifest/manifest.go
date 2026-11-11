// Package manifest is the composition root: the only code that names more
// than one policy, and orders them.
package manifest

import (
	"github.com/caelyreth/neverland/editor/policies/blocks"
	"github.com/caelyreth/neverland/editor/policies/cjk"
	"github.com/caelyreth/neverland/editor/policies/core"
	"github.com/caelyreth/neverland/editor/policies/deletion"
	"github.com/caelyreth/neverland/editor/policies/feedback"
	"github.com/caelyreth/neverland/editor/policies/figures"
	"github.com/caelyreth/neverland/editor/policies/formats"
	"github.com/caelyreth/neverland/editor/policies/formulas"
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
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Manifest lists the product's policies.
//
// TODO(policies): tables remain. Once pairs spell a typed '*',
// markdown must take precedence over pairs for that character.
var Manifest = policy.Manifest{
	Enabled: []*policy.Policy{core.Policy, notation.Policy, pairs.Policy, markdown.Policy, formulas.Policy,
		typography.Policy, cjk.Policy, navigation.Policy, scopes.Policy, deletion.Policy, formats.Policy,
		blocks.Policy, lists.Policy, figures.Policy, words.Policy, history.Policy, feedback.Policy, guides.Policy,
		statistics.Policy},
	Profile: profile.Profile,
	Resolve: []policy.Order{
		// "$" in an empty formula is its source or a math block, not its
		// close.
		policy.Over(formulas.Policy, navigation.Policy, vocab.RoleCloser),
		// Deleting toward a whole, by any unit, shows it first.
		policy.Over(deletion.Policy, words.Policy, vocab.IntentDelete),
	},
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
