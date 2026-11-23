package policy

import (
	"slices"

	"github.com/caelyreth/neverland/internal/editor/vocab"
)

// A command a policy declares may be offered (vocab.OfferGroup): put
// forward in a group, where its conditions hold, as what suits the
// selection; Where reads the selection's span (span.go), so a word is not
// offered what turns blocks into another kind. Whether it can run is its
// preview, and whether it is on is a state a policy declares for its
// intent (State). A host asks the engine for both (Engine.Offers) and
// draws them, knowing nothing of what they do.

// Offering is how a command is offered: its group, its place in it, the
// key that runs it where its surface reads keys, and where it suits.
type Offering struct {
	Group vocab.OfferGroup
	Order int
	Key   string
	Where []Cond
}

// Offer offers the command in group, at order within it, run by key where
// its surface reads keys, wherever every condition of where holds.
func Offer(group vocab.OfferGroup, order int, key string, where ...Cond) CommandSetting {
	return func(c *Command) { c.Offer = &Offering{Group: group, Order: order, Key: key, Where: where} }
}

// Holds reports whether the offering suits where c stands.
func (o *Offering) Holds(c *Ctx) bool {
	for _, cond := range o.Where {
		if !cond.test(c) {
			return false
		}
	}
	return true
}

// stateDecl is a policy's answer to whether intents of one pattern are on
// where the context stands.
type stateDecl struct {
	policy string
	match  matcher
	on     func(c *Ctx, in vocab.Intent) bool
}

// State declares whether an intent pat matches is on where the context
// stands: a mark the selection has all of, a kind its blocks are. It is
// what a host checks or lights, read once for every command resolving to
// such an intent; the first enabled policy declaring one answers.
func State[I vocab.Intent](p *P, pat Pattern[I], on func(c *Ctx, in I) bool) {
	p.states = append(p.states, stateDecl{policy: p.policy.id, match: pat.m,
		on: func(c *Ctx, in vocab.Intent) bool { return on(c, in.(I)) }})
}

// On reports whether in is on where c stands, by the state declared for
// it; false where none is.
func (p *Program) On(c *Ctx, in vocab.Intent) bool {
	for _, s := range p.states {
		if s.match.match(in) {
			return s.on(c, in)
		}
	}
	return false
}

// OfferedGroup is a group's offered commands, in their order.
type OfferedGroup struct {
	Group    vocab.OfferGroup
	Commands []Command
}

// Offered are the offered commands of the enabled policies, by group in
// the groups' order.
func (p *Program) Offered() []OfferedGroup { return p.offered }

// compileOffers groups the offered commands, a group declared alike
// wherever it is named.
func (p *Program) compileOffers(rep *Report) {
	byID := map[string]int{}
	for _, pol := range p.policies {
		for _, c := range pol.commands {
			if c.Offer == nil {
				continue
			}
			g := c.Offer.Group
			i, ok := byID[g.ID]
			if !ok {
				i = len(p.offered)
				byID[g.ID] = i
				p.offered = append(p.offered, OfferedGroup{Group: g})
			} else if p.offered[i].Group != g {
				rep.errorf("offer group %s declared two ways", g.ID)
			}
			p.offered[i].Commands = append(p.offered[i].Commands, c)
		}
		p.states = append(p.states, pol.states...)
	}
	slices.SortStableFunc(p.offered, func(a, b OfferedGroup) int { return a.Group.Order - b.Group.Order })
	for _, g := range p.offered {
		slices.SortStableFunc(g.Commands, func(a, b Command) int { return a.Offer.Order - b.Offer.Order })
	}
}
