// Package figures is the figures policy: pictures placed as figures,
// resized and aligned. A figure is an object, its caption its own slot:
// how the keys act on it and in its caption is core's, as for every
// object.
package figures

import (
	"time"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

// ID is the policy's id.
const ID = "figures"

// Policy is the figures policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	p.Command("figures.insert", "commands.insertPicture", profile.PickPictures{}, policy.Offer(profile.InsertOffers, 0, ""))
	p.Command("figures.alignStart", "commands.alignPictureStart", profile.AlignFigure{To: profile.AlignStart},
		policy.Offer(profile.PictureAlignOffers, 0, ""))
	p.Command("figures.alignCenter", "commands.alignPictureCenter", profile.AlignFigure{To: profile.AlignCenter},
		policy.Offer(profile.PictureAlignOffers, 1, ""))
	p.Command("figures.copy", "commands.copyPicture", profile.UsePicture{Use: profile.PictureCopy}, policy.Offer(profile.PictureOffers, 0, ""))
	p.Command("figures.save", "commands.savePicture", profile.UsePicture{Use: profile.PictureSave}, policy.Offer(profile.PictureOffers, 1, ""))
	p.Command("figures.preview", "commands.previewPicture", profile.UsePicture{Use: profile.PicturePreview},
		policy.Offer(profile.PictureOffers, 2, ""))
	p.Command("figures.replace", "commands.replacePicture", profile.UsePicture{Use: profile.PictureReplace},
		policy.Offer(profile.PictureReplaceOffers, 0, ""))

	in := policy.Slot.Inside()
	policy.On(p, "pick", policy.Intent[profile.PickPictures](), in, pick).Any()
	policy.On(p, "place", policy.Intent[profile.PlaceFigures](), in, place).Any()
	policy.On(p, "resize", policy.Intent[profile.ResizeFigure](), in, resize).Any()

	figure := policy.Node(profile.Figure)
	held := policy.Or(figure.Selected(), figure.Own())
	policy.On(p, "align", policy.Intent[profile.AlignFigure](), held, align).Any()
	policy.On(p, "use", policy.Intent[profile.UsePicture](), held, use).Any()
	policy.On(p, "replace", policy.Intent[profile.ReplacePicture](), in, replace).Any()

	// The card's demonstration, played by the engine itself: a picture
	// still on its way shows its place.
	p.Showcase(2600*time.Millisecond,
		policy.Types("Low tide"),
		policy.Waits(400*time.Millisecond),
		policy.Presses(profile.PlaceFigures{Pictures: []profile.Picture{{Width: 1600, Height: 900}}}),
		policy.Waits(400*time.Millisecond),
		policy.Types("The bay at dawn"),
	)
}, policy.Label("policies.figures"), policy.Description("policies.figures.description"))

// pick asks the host for pictures; it answers with PlaceFigures.
func pick(*policy.Ctx, profile.PickPictures, policy.At[doc.SlotRef]) policy.Result {
	return policy.Result{Effects: []policy.Effect{{Kind: policy.EffectMedia, Data: profile.PictureAsk{Use: profile.PictureInsert}}}}
}

// place puts pictures on the page as figures, in order, replacing what
// the selection covers, the caret going on in the last one's caption.
func place(c *policy.Ctx, in profile.PlaceFigures, _ policy.At[doc.SlotRef]) policy.Result {
	if len(in.Pictures) == 0 {
		return policy.Swallow
	}
	var clear vocab.Edit
	at := c.Head().Pos
	if in.At != nil {
		if _, ok := c.Doc().Node(in.At.Pos.Node); !ok {
			return policy.Swallow
		}
		at = in.At.Pos
	} else {
		clear, at, _ = c.Replacing()
	}
	// The last figure's id, which the edit learns as it places them.
	var last doc.ID
	res := policy.Do(func(tx *doc.Tx) error {
		if clear != nil {
			if err := clear(tx); err != nil {
				return err
			}
			at, _ = doc.Steps(tx.Steps()).Map(at, doc.BiasBefore)
		}
		gap, err := vocab.PlaceGap(tx, at, profile.Figure)
		if err != nil {
			return err
		}
		nodes := make([]*doc.Node, len(in.Pictures))
		for i, pic := range in.Pictures {
			nodes[i] = profile.NewFigure(tx.Schema(), pic, "")
		}
		if err := tx.Apply(doc.Insert{Parent: gap.Node, Slot: gap.Slot, Index: gap.Offset, Nodes: nodes}); err != nil {
			return err
		}
		pn, _ := tx.Node(gap.Node)
		last = pn.Slot(gap.Slot).Child(gap.Offset + len(nodes) - 1).ID()
		return nil
	})
	res.Then = func(doc.Reader) doc.Selection {
		return doc.Collapsed(doc.CaretAt(doc.Pos{Node: last}))
	}
	res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
	return res
}

// resize sets a figure's span.
func resize(c *policy.Ctx, in profile.ResizeFigure, _ policy.At[doc.SlotRef]) policy.Result {
	n, ok := c.Doc().Node(in.ID)
	if !ok || n.Kind() != profile.Figure {
		return policy.Swallow
	}
	shape := profile.ShapeOf(n)
	if shape.Span == in.Span {
		return policy.Swallow
	}
	shape.Span = in.Span
	return policy.Do(profile.SetShape(in.ID, shape))
}

// align aligns the figure selected or captioned.
func align(c *policy.Ctx, in profile.AlignFigure, at policy.At[*doc.Node]) policy.Result {
	shape := profile.ShapeOf(at.Data)
	if shape.Align == in.To {
		return policy.Swallow
	}
	shape.Align = in.To
	return policy.Do(profile.SetShape(at.Data.ID(), shape))
}

// use asks the host to use the picture of the figure selected or
// captioned. A figure without one, as a showcase's, has nothing to give.
func use(_ *policy.Ctx, in profile.UsePicture, at policy.At[*doc.Node]) policy.Result {
	media := profile.ShapeOf(at.Data).Media
	if media == "" || in.Use == profile.PictureInsert {
		return policy.Swallow
	}
	ask := profile.PictureAsk{Use: in.Use, Figure: at.Data.ID(), Media: media}
	return policy.Result{Effects: []policy.Effect{{Kind: policy.EffectMedia, Data: ask}}}
}

// replace shows another picture in a figure.
func replace(c *policy.Ctx, in profile.ReplacePicture, _ policy.At[doc.SlotRef]) policy.Result {
	n, ok := c.Doc().Node(in.ID)
	if !ok || n.Kind() != profile.Figure {
		return policy.Swallow
	}
	shape := profile.ShapeOf(n)
	if shape.Picture == in.Picture {
		return policy.Swallow
	}
	shape.Picture = in.Picture
	return policy.Do(profile.SetShape(in.ID, shape))
}
