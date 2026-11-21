// Package navigation is the navigation policy: Leave exits the innermost
// configured Leavable scope past its end, and Return enters the scope
// that ends right before the caret. Both read where the caret is, never
// how it got there: Return mirrors Leave, so after Leave it goes back in,
// and after a closer typed by hand it goes in all the same. A character
// typed at a Closer's inner end that closes it steps over its closing
// side, whoever wrote that side. MatchPair goes between a Closer's two
// inner ends.
package navigation

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
	"time"
)

// ID is the policy's id.
const ID = "navigation"

// Policy is the navigation policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	// Leave exits the innermost scope; Return enters the nearest one a
	// caret crosses into going back.
	scopes := policy.Role(vocab.RoleLeavable)
	policy.On(p, "leave", policy.Intent[vocab.Leave](), scopes.Inside(), leave).Any()
	policy.On(p, "return", policy.Intent[vocab.Return](), scopes.Behind(), enter).Any()
	closers := policy.Role(vocab.RoleCloser)
	policy.On(p, "close", policy.Intent[vocab.Type](), policy.Or(closers.AtEnd(), closers.Empty()), close)
	policy.On(p, "match-pair", policy.Intent[vocab.MatchPair](), policy.Slot.Inside(), matchPair).Any()
	p.Command("navigation.matchPair", "commands.matchPair", vocab.MatchPair{}, policy.Key("shift+primary+\\"))

	// The card's demonstration, played by the engine itself.
	p.Showcase(2600*time.Millisecond,
		policy.Types("Leave a scope (with Tab"),
		policy.Waits(1100*time.Millisecond),
		policy.Presses(vocab.Leave{}),
		policy.Waits(300*time.Millisecond),
		policy.Types(" and keep writing."),
	)
}, policy.Label("policies.navigation"), policy.Description("policies.navigation.description"))

// close steps over the closing side one glyph, for text that closes the
// scope; other text is typed.
func close(c *policy.Ctx, in vocab.Type, at policy.At[vocab.Closer]) policy.Result {
	if !at.Data.Closes(in.Text) {
		return policy.Pass
	}
	next, ok := c.Stops().Step(c.Head(), vocab.Forward)
	if !ok {
		return policy.Pass
	}
	return policy.Select(doc.Collapsed(next))
}

// matchPair goes to the other inner end of the innermost pair or mark
// the caret stands in, any a typed character closes: to its start from
// its end, and to its end from anywhere else in it.
func matchPair(c *policy.Ctx, _ vocab.MatchPair, _ policy.At[doc.SlotRef]) policy.Result {
	head := c.Head()
	levels := c.Stack().Levels
	for i := len(levels) - 1; i >= 0; i-- {
		l := levels[i]
		if l.Class != policy.LevelFact && l.Class != policy.LevelAnnotation {
			continue
		}
		if _, ok := l.Role(c.Doc(), vocab.RoleCloser); !ok {
			continue
		}
		to := l.End
		if head.Pos == l.End.Pos {
			to = l.Start
		}
		return policy.Reveal(doc.Collapsed(to))
	}
	return policy.Swallow
}

// leave exits the scope, past its end.
func leave(_ *policy.Ctx, _ vocab.Leave, at policy.At[vocab.Leavable]) policy.Result {
	res := policy.Select(doc.Collapsed(at.Data.Exit()))
	res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
	return res
}

// enter goes into the scope ending right before the caret, where Leave
// would have left it from: its entry.
func enter(_ *policy.Ctx, _ vocab.Return, at policy.At[vocab.Leavable]) policy.Result {
	res := policy.Select(doc.Collapsed(at.Data.Entry()))
	res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
	return res
}
