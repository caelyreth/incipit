// Package guides is the guides policy: key hints in the text, from
// previews (§12.1). Where Leave would exit a scope, its key shows where
// the caret would land; where Return would go back, at the caret.
//
// TODO(guides): SelectScope's growth and an armed deletion, once the
// scopes and deletion policies exist.
package guides

import "github.com/caelyreth/neverland/editor/policy"

// Policy is the guides policy.
var Policy = policy.Define("guides", func(p *policy.P) {
	p.Guide("core.leave", policy.GuideAtLanding)
	p.Guide("core.return", policy.GuideAtCaret)
})
