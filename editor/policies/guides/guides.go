// Package guides is the guides policy: key hints in the text, from
// previews (§12.1). Where Leave, Return or SelectScope would move the
// caret, its key shows where the caret would land.
package guides

import (
	"time"

	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Policy is the guides policy.
var Policy = policy.Define("guides", func(p *policy.P) {
	p.Guide("core.leave")
	p.Guide("core.return")
	p.Guide("core.selectAll")
	p.Guide("lists.nest")
	p.Guide("lists.outdent")

	// The card's demonstration, played by the engine itself.
	p.Showcase(2000*time.Millisecond,
		policy.Types("Keys show [where they act"),
		policy.Waits(1400*time.Millisecond),
		policy.Presses(vocab.Leave{}),
		policy.Waits(1400*time.Millisecond),
		policy.Presses(vocab.Return{}),
		policy.Waits(600*time.Millisecond),
	)
}, policy.Label("policies.guides"), policy.Description("policies.guides.description"))
