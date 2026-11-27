// Package manifest is the composition root: the only code that names more
// than one policy, and orders them.
package manifest

import (
	"github.com/caelyreth/incipit/internal/editor/policies/blocks"
	"github.com/caelyreth/incipit/internal/editor/policies/code"
	"github.com/caelyreth/incipit/internal/editor/policies/cjk"
	"github.com/caelyreth/incipit/internal/editor/policies/core"
	"github.com/caelyreth/incipit/internal/editor/policies/deletion"
	"github.com/caelyreth/incipit/internal/editor/policies/feedback"
	"github.com/caelyreth/incipit/internal/editor/policies/figures"
	"github.com/caelyreth/incipit/internal/editor/policies/find"
	"github.com/caelyreth/incipit/internal/editor/policies/formats"
	"github.com/caelyreth/incipit/internal/editor/policies/formulas"
	"github.com/caelyreth/incipit/internal/editor/policies/guides"
	"github.com/caelyreth/incipit/internal/editor/policies/history"
	"github.com/caelyreth/incipit/internal/editor/policies/lists"
	"github.com/caelyreth/incipit/internal/editor/policies/markdown"
	"github.com/caelyreth/incipit/internal/editor/policies/navigation"
	"github.com/caelyreth/incipit/internal/editor/policies/notation"
	"github.com/caelyreth/incipit/internal/editor/policies/pairs"
	"github.com/caelyreth/incipit/internal/editor/policies/scopes"
	"github.com/caelyreth/incipit/internal/editor/policies/spelling"
	"github.com/caelyreth/incipit/internal/editor/policies/statistics"
	"github.com/caelyreth/incipit/internal/editor/policies/tables"
	"github.com/caelyreth/incipit/internal/editor/policies/tags"
	"github.com/caelyreth/incipit/internal/editor/policies/typography"
	"github.com/caelyreth/incipit/internal/editor/policies/words"
	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

// Manifest lists the product's policies.
//
// TODO(policies): once pairs spell a typed '*', markdown must take
// precedence over pairs for that character.
var Manifest = policy.Manifest{
	Enabled: []*policy.Policy{core.Policy, notation.Policy, pairs.Policy, markdown.Policy, formulas.Policy,
		typography.Policy, cjk.Policy, navigation.Policy, scopes.Policy, deletion.Policy, formats.Policy,
		blocks.Policy, code.Policy, lists.Policy, figures.Policy, tables.Policy, tags.Policy, words.Policy, spelling.Policy, history.Policy, feedback.Policy, find.Policy, guides.Policy,
		statistics.Policy},
	Profile: profile.Profile,
	Resolve: []policy.Order{
		// "$" in an empty formula is its source or a math block, not its
		// close.
		policy.Over(formulas.Policy, navigation.Policy, vocab.RoleCloser),
		// Autocorrect finishes a word before typography shapes trailing
		// punctuation.
		policy.RunsBefore(spelling.Policy, typography.Policy, policy.PhaseCharacters),
	},
}

// Checked has every program compiled check that each rewrite's edits are
// of the kinds it declares it Produces (§11.14): the app sets it in
// development builds, and tests ask for it by Compile's strict.
var Checked bool

// Compile builds the program for settings values, checked where strict
// asks it or Checked is set.
func Compile(values policy.Values, strict bool) (*policy.Program, error) {
	m := Manifest
	m.Strict = strict || Checked
	return policy.Compile(m, values)
}
