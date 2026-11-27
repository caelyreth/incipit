package engine

import (
	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

// Offered is a command offered for a view's selection (policy.Offer): the
// key that runs it where its surface reads keys, whether it can run now
// (its preview), and whether it is on (policy.State).
type Offered struct {
	Command   vocab.CommandID
	Key       string
	Available bool
	On        bool
}

// OfferedGroup is a group's commands offered for a selection.
type OfferedGroup struct {
	Group  vocab.OfferGroup
	Offers []Offered
}

// offers is a view's offers as read at an engine version: each group's,
// once a surface showing it asked (nil before).
type offers struct {
	version uint64
	groups  []*OfferedGroup
}

// Offers are the commands offered for a view's selection in the groups
// shown on surface: each where its conditions hold, its group left out
// where none does. A group is read once for each version of the engine,
// when a surface showing it first asks, however many ask.
func (e *Engine) Offers(viewID uint64, surface vocab.Surface) []OfferedGroup {
	e.mu.Lock()
	defer e.mu.Unlock()
	cur, ok := e.views[viewID]
	if !ok {
		return nil
	}
	declared := e.prog.Offered()
	memo, ok := e.offers[viewID]
	if !ok || memo.version != e.version || len(memo.groups) != len(declared) {
		memo = offers{version: e.version, groups: make([]*OfferedGroup, len(declared))}
	}
	var c *policy.Ctx
	var out []OfferedGroup
	for i, g := range declared {
		if g.Group.In&surface == 0 {
			continue
		}
		if memo.groups[i] == nil {
			if c == nil {
				view := *cur
				c = policy.NewCtx(e.prog, e.store, e.snap, &view, vocab.Input{At: e.at}, e.stopsFor(viewID))
			}
			memo.groups[i] = e.offer(viewID, c, g)
		}
		if len(memo.groups[i].Offers) > 0 {
			out = append(out, *memo.groups[i])
		}
	}
	e.offers[viewID] = memo
	return out
}

// offer reads a group's offers where c stands.
func (e *Engine) offer(viewID uint64, c *policy.Ctx, g policy.OfferedGroup) *OfferedGroup {
	group := &OfferedGroup{Group: g.Group}
	for _, cmd := range g.Commands {
		if !cmd.Offer.Holds(c) {
			continue
		}
		res, err := e.process(viewID, vocab.Input{Intent: cmd.Intent, At: e.at}, false)
		group.Offers = append(group.Offers, Offered{Command: cmd.ID, Key: cmd.Offer.Key,
			Available: err == nil && res.preview.Available, On: e.prog.On(c, cmd.Intent)})
	}
	return group
}
