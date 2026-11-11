package navigation

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/engine"
	"github.com/caelyreth/neverland/editor/policies/core"
	"github.com/caelyreth/neverland/editor/policies/formulas"
	"github.com/caelyreth/neverland/editor/policies/lists"
	"github.com/caelyreth/neverland/editor/policies/markdown"
	"github.com/caelyreth/neverland/editor/policies/notation"
	"github.com/caelyreth/neverland/editor/policies/pairs"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

func open(t *testing.T, values policy.Values) *engine.Engine {
	t.Helper()
	prog, err := policy.Compile(policy.Manifest{Profile: profile.Profile,
		Enabled: []*policy.Policy{core.Policy, notation.Policy, pairs.Policy, markdown.Policy, formulas.Policy, lists.Policy, Policy},
		Resolve: []policy.Order{policy.Over(formulas.Policy, Policy, vocab.RoleCloser)},
		Strict:  true,
	}, values)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := profile.EmptyPage(1, 5, doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	e := engine.New(snap, prog, engine.Options{Trace: true})
	e.OpenView(1)
	return e
}

func typeText(t *testing.T, e *engine.Engine, s string) {
	t.Helper()
	now := time.Unix(10, 0)
	for _, r := range s {
		now = now.Add(time.Second)
		if _, err := e.Do(1, vocab.Input{Intent: vocab.Type{Text: string(r)}, At: now}); err != nil {
			t.Fatal(err)
		}
	}
}
