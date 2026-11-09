// Package navigation is the navigation policy: Leave exits the innermost
// Leavable scope, and Return re-enters the scope just left. After a press
// moves the caret away from the last edit, Return goes back to it for a
// while: the return-to-edit offer.
//
// TODO(navigation): the option of which roles are leavable once roles
// other than Leavable can be.
package navigation

import (
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// ReturnToEdit offers Return back to the last edit after a press moves
// the caret elsewhere.
var ReturnToEdit = policy.Bool("returnToEdit", true, policy.Label("settings.navigation.returnToEdit"),
	policy.Description("settings.navigation.returnToEdit.description"))

// OfferFor is how long the return offer stands.
const OfferFor = 8 * time.Second

// The policy's own annotations: where the last edit was, and the offer to
// go back to it, at the caret a press moved.
var (
	lastEdit = vocab.NewAnnotationKind("navigation.lastEdit", false)
	offer    = vocab.NewAnnotationKind("navigation.returnOffer", false)
)

var textEdits = []doc.RecordKind{doc.RecordTextInserted, doc.RecordTextDeleted, doc.RecordMarks, doc.RecordTransferred,
	doc.RecordNodesInserted, doc.RecordNodesRemoved, doc.RecordNodeChanged, doc.RecordNodeMoved}

// Policy is the navigation policy.
var Policy = policy.Define("navigation", func(p *policy.P) {
	exists := func(r doc.Reader, a policy.Annotation) bool {
		_, ok := r.Node(a.Anchor.From.Node)
		return ok
	}
	p.Annotation(vocab.LeftScope, exists)
	p.Handle("leave").On(vocab.On(vocab.IntentLeave)).In(vocab.RoleScope(vocab.RoleLeavable)).Do(leave)
	p.Handle("return").On(vocab.On(vocab.IntentReturn)).Do(back)

	if !policy.Get(p, ReturnToEdit) {
		return
	}
	p.Annotation(lastEdit, exists)
	p.Annotation(offer, exists)
	p.Observe("last-edit", policy.Trigger{Kinds: textEdits}, func(c *policy.Ctx, _ []doc.Record) policy.Result {
		return policy.Result{DropKinds: []vocab.AnnotationKind{lastEdit, offer}, Annotate: []policy.Annotate{{
			Kind: lastEdit, Anchor: policy.PointAnchor(c.Head().Pos), Lifetime: policy.WithView}}}
	})
	p.Observe("offer", policy.Trigger{Kinds: []doc.RecordKind{doc.RecordSelection}}, offerReturn)
})

// leave exits the innermost leavable scope, remembering where the caret
// was for Return until the next edit or the next Leave: moving away and
// back to the exit finds the way back again; writing on moves on.
func leave(c *policy.Ctx, m policy.Match) policy.Result {
	l, ok := m.Role.(vocab.Leavable)
	if !ok {
		return policy.Swallow
	}
	exit := l.Exit()
	res := policy.Select(doc.Collapsed(exit))
	res.DropKinds = []vocab.AnnotationKind{vocab.LeftScope, offer}
	res.Annotate = []policy.Annotate{{Kind: vocab.LeftScope, Anchor: policy.PointAnchor(exit.Pos),
		Data: c.Head(), Lifetime: policy.UntilEdited}}
	res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
	return res
}

// back goes where the caret came from: into the scope just left, or to
// the last edit the offer stands for. With neither, it does nothing.
func back(c *policy.Ctx, _ policy.Match) policy.Result {
	head := c.Head().Pos
	for _, kind := range []vocab.AnnotationKind{vocab.LeftScope, offer} {
		for _, a := range c.Annotations().At(kind, head) {
			to, ok := a.Data.(doc.Caret)
			if !ok {
				continue
			}
			res := policy.Select(doc.Collapsed(to))
			res.DropKinds = []vocab.AnnotationKind{vocab.LeftScope, offer}
			res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
			return res
		}
	}
	return policy.Swallow
}

// offerReturn offers the way back when a press put the caret in another
// block than the last edit's.
func offerReturn(c *policy.Ctx, _ []doc.Record) policy.Result {
	if c.Intent().Kind() != vocab.IntentPlace {
		return policy.Result{}
	}
	head := c.Head().Pos
	for _, a := range c.Annotations().Of(lastEdit) {
		if a.Anchor.From.Node == head.Node {
			return policy.Result{DropKinds: []vocab.AnnotationKind{offer}}
		}
		until := c.Now().Add(OfferFor)
		return policy.Result{DropKinds: []vocab.AnnotationKind{offer}, Annotate: []policy.Annotate{{
			Kind: offer, Anchor: policy.PointAnchor(head), Data: doc.CaretAt(a.Anchor.From),
			Lifetime: policy.Until(until)}}, Deadlines: []time.Time{until}}
	}
	return policy.Result{}
}
