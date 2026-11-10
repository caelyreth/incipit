// Package guides is the guides policy: key hints in the text, from
// previews (§12.1). Where Leave or Return would move the caret, its key
// shows where the caret would land.
//
// TODO(guides): SelectScope growth and an armed deletion once the scopes and
// deletion policies exist. Pairs can arm now, but no guide points at it.
package guides

import "github.com/caelyreth/neverland/editor/policy"

// Policy is the guides policy.
var Policy = policy.Define("guides", func(p *policy.P) {
	p.Guide("core.leave")
	p.Guide("core.return")
})
