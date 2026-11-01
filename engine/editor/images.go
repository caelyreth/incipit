package editor

import (
	"errors"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

// An image block shows a picture above its line, and its text is the
// picture's caption: the caret, typing, marks and history all work in a
// caption as in any block's text. Its notation is the one character the
// picture stands on, surface.FigureObject, which shows as nothing. A
// picture is never taken away a character at a time: Backspace at its
// caption's start, or Delete at the end of the block above it, selects it
// (selectFigure), and only deleting it then, or an edit across its whole
// line, takes it away. The picture itself is named by its media, kept by
// the notebook, never by the page.
const (
	// ImageNode is an image block; its text is its caption.
	ImageNode model.NodeTypeID = "image"

	// MediaAttribute names the picture an image block shows by its
	// media's hash, as the notebook keeps it.
	MediaAttribute = "media"

	imageWidthAttribute  = "width"
	imageHeightAttribute = "height"
	// imageSpanAttribute is the share of the column the picture takes, in
	// thousandths; none sets it at its own size.
	imageSpanAttribute  = "span"
	imageAlignAttribute = "align"
	imageAltAttribute   = "alt"

	imageAlignStart = "start"
	// captionScale sets a caption a step smaller than the text around it.
	captionScale = 0.88

	figureTrigger = "writing:figure"
)

// imageAttributes are the attributes only an image block has.
var imageAttributes = []string{
	MediaAttribute, imageWidthAttribute, imageHeightAttribute,
	imageSpanAttribute, imageAlignAttribute, imageAltAttribute,
}

// ImageShape is what an image block shows apart from its caption.
type ImageShape struct {
	// Media is the picture's hash, and Width and Height its size in
	// pixels.
	Media         string
	Width, Height int
	// Span is the share of the column it takes, from 0 (its own size) to 1.
	Span  float32
	Align surface.FigureAlign
	Alt   string
}

// ErrNoImage reports an image command on a block that is not an image.
var ErrNoImage = errors.New("editor: the block is not an image")

// ImageShapeOf reads an image block's shape.
func ImageShapeOf(node *model.Node) (ImageShape, bool) {
	if node == nil || node.Kind() != ImageNode {
		return ImageShape{}, false
	}
	text := func(key string) string {
		value, _ := node.Attribute(key)
		text, _ := value.Text()
		return text
	}
	integer := func(key string) int64 {
		value, _ := node.Attribute(key)
		integer, _ := value.Integer()
		return integer
	}
	shape := ImageShape{
		Media:  text(MediaAttribute),
		Width:  int(integer(imageWidthAttribute)),
		Height: int(integer(imageHeightAttribute)),
		Span:   float32(max(0, min(1000, integer(imageSpanAttribute)))) / 1000,
		Alt:    text(imageAltAttribute),
	}
	if text(imageAlignAttribute) == imageAlignStart {
		shape.Align = surface.FigureStart
	}
	return shape, shape.Media != "" && shape.Width > 0 && shape.Height > 0
}

// writeImageShape gives node shape's attributes, leaving out the defaults.
func writeImageShape(document *model.DocumentDraft, node model.NodeID, shape ImageShape) error {
	if shape.Media == "" || shape.Width <= 0 || shape.Height <= 0 {
		return ErrNoImage
	}
	current, ok := document.Node(node)
	if !ok {
		return &model.DocumentError{Kind: model.DocumentErrorMissingNode, Node: node}
	}
	set := func(key string, keep bool, value model.AttributeValue) error {
		_, has := current.Attribute(key)
		switch {
		case keep:
			return document.SetAttribute(node, key, value)
		case has:
			return document.RemoveAttribute(node, key)
		}
		return nil
	}
	span := int64(max(0, min(1, shape.Span)) * 1000)
	return errors.Join(
		set(MediaAttribute, true, model.TextValue(shape.Media)),
		set(imageWidthAttribute, true, model.IntegerValue(int64(shape.Width))),
		set(imageHeightAttribute, true, model.IntegerValue(int64(shape.Height))),
		set(imageSpanAttribute, span > 0, model.IntegerValue(span)),
		set(imageAlignAttribute, shape.Align == surface.FigureStart, model.TextValue(imageAlignStart)),
		set(imageAltAttribute, shape.Alt != "", model.TextValue(shape.Alt)),
	)
}

// pageFlow is the surface flow of a projection: its text, its style runs,
// and the pictures its image blocks show above their lines.
func pageFlow(p *projection.Projection, document model.DocumentSnapshot) (*surface.Flow, error) {
	runs, err := InlineStyleRuns(p, document)
	if err != nil {
		return nil, err
	}
	flow, err := surface.NewFlow(p.Version(), p.Text(), runs)
	if err != nil {
		return nil, err
	}
	var figures []surface.Figure
	p.EachSegment(func(_ int, segment projection.Segment) bool {
		target, ok := segment.AttributeTarget()
		if !ok || target.Key() != blockNotationKey {
			return true
		}
		node, ok := document.Node(target.Node())
		shape, image := ImageShapeOf(node)
		if !ok || !image {
			return true
		}
		figures = append(figures, surface.Figure{
			At:     segment.ProjectedRange().Start,
			Node:   node.ID(),
			Media:  shape.Media,
			Width:  shape.Width,
			Height: shape.Height,
			Span:   shape.Span,
			Align:  shape.Align,
			Alt:    shape.Alt,
		})
		if blockLength(node) > 0 {
			figures[len(figures)-1].Caption = 1
		}
		return true
	})
	if len(figures) == 0 {
		return flow, nil
	}
	return flow.WithFigures(figures)
}

// InsertImage puts a picture on the page: in place of the empty paragraph
// the caret is in, or after the caret's block, with a paragraph after it
// to go on writing in, where the caret goes.
func (s *Session) InsertImage(shape ImageShape) (kernel.Outcome, error) {
	at := s.presentation.Selection.Node()
	var resume model.NodeID
	outcome, err := s.dispatchFigure(func(document *model.DocumentDraft) (bool, error) {
		current, ok := document.Node(at)
		if !ok {
			return false, nil
		}
		parent, _ := current.Parent()
		container, _ := document.Node(parent)
		index := 0
		for position, child := range container.Children() {
			if child == at {
				index = position
			}
		}
		image := at
		if current.Kind() != ParagraphNode || blockLength(current) > 0 {
			index++
			created, err := document.CreateNode(parent, index, ImageNode)
			if err == nil {
				err = document.SetTextValue(created, "")
			}
			if err != nil {
				return false, err
			}
			image = created
		} else if err := document.SetNodeKind(at, ImageNode); err != nil {
			return false, err
		}
		if err := writeImageShape(document, image, shape); err != nil {
			return false, err
		}
		container, _ = document.Node(parent)
		children := container.Children()
		if index+1 < len(children) {
			resume = children[index+1]
			return true, nil
		}
		created, err := document.CreateNode(parent, index+1, ParagraphNode)
		if err == nil {
			err = document.SetTextValue(created, "")
		}
		resume = created
		return true, err
	})
	if err != nil || outcome != kernel.OutcomeHandled {
		return outcome, err
	}
	return outcome, s.Select(model.NewTextSelection(
		model.NewTextAnchor(resume, 0, model.AffinityRight, s.runtime.Revision()),
		model.NewTextAnchor(resume, 0, model.AffinityRight, s.runtime.Revision()),
	))
}

// ShapeImage changes what an image block shows, as change has it, in one
// step of history.
func (s *Session) ShapeImage(node model.NodeID, change func(*ImageShape)) (kernel.Outcome, error) {
	return s.dispatchFigure(func(document *model.DocumentDraft) (bool, error) {
		current, ok := document.Node(node)
		shape, image := ImageShapeOf(current)
		if !ok || !image {
			return false, ErrNoImage
		}
		next := shape
		change(&next)
		if next == shape {
			return false, nil
		}
		return true, writeImageShape(document, node, next)
	})
}

// guardFigure is Backspace at the start of an image's caption, or Delete
// at the end of the block above an image: either selects the picture, for
// a second press to delete, rather than taking it away with the character
// it stands on.
func (s *Session) guardFigure(backward bool) (kernel.Outcome, bool, error) {
	selection := s.presentation.Selection
	if !selection.IsCollapsed() {
		return 0, false, nil
	}
	document := s.runtime.Snapshot().Document()
	node, ok := document.Node(selection.Head().Node())
	if !ok {
		return 0, false, nil
	}
	image := node
	if backward {
		ok = selection.Head().Offset() == 0
	} else if selection.Head().Offset() == blockLength(node) {
		image, ok = neighborBlock(document, node.ID(), 1)
	} else {
		ok = false
	}
	if !ok || image.Kind() != ImageNode {
		return 0, false, nil
	}
	return kernel.OutcomeHandled, true, s.selectFigure(image.ID())
}

// selectFigure selects an image block's whole line, the character its
// picture stands on and its caption, as a click on the picture does.
func (s *Session) selectFigure(node model.NodeID) error {
	start, end, found := model.UTF16Offset(0), model.UTF16Offset(0), false
	s.presentation.Projection.EachSegment(func(_ int, segment projection.Segment) bool {
		if owner, ok := segmentNode(segment); !ok || owner != node {
			return true
		}
		span := segment.ProjectedRange()
		if !found {
			start, found = span.Start, true
		}
		end = span.End
		return true
	})
	if !found {
		return nil
	}
	return s.SelectProjected(start, end)
}

// segmentNode is the block a segment of the projection shows, its text or
// its notation.
func segmentNode(segment projection.Segment) (model.NodeID, bool) {
	if target, ok := segment.AttributeTarget(); ok {
		return target.Node(), true
	}
	if target, ok := segment.SemanticTarget(); ok {
		return target.Node(), true
	}
	return 0, false
}

// dispatchFigure runs an edit of image blocks as the writing policy's
// figure rule, keeping the selection where it was.
func (s *Session) dispatchFigure(edit func(*model.DocumentDraft) (bool, error)) (kernel.Outcome, error) {
	if !s.runtime.Configuration().IsEnabled(WritingPolicyID) {
		return kernel.OutcomePass, nil
	}
	if _, active := s.composition.Preview(); active {
		return 0, ErrCompositionActive
	}
	return s.dispatchSelectionCommand(
		figureTrigger,
		&figureInvoker{session: s, edit: edit},
		func(*model.Node) *model.MarkID { return activeMark(s.inline) },
	)
}

type figureInvoker struct {
	session *Session
	edit    func(*model.DocumentDraft) (bool, error)
}

func (i *figureInvoker) Invoke(
	rule *kernel.LinkedRule,
	leaf kernel.LeafID,
	context *kernel.RuleContext,
) (kernel.Outcome, error) {
	if rule == nil || rule.Key.Owner != WritingPolicyID || rule.Key.ID != "figure" {
		return i.session.invoker.Invoke(rule, leaf, context)
	}
	document, err := context.DocumentMut()
	if err != nil {
		return 0, err
	}
	changed, err := i.edit(document)
	if err != nil {
		return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
	}
	if !changed {
		return kernel.OutcomePass, nil
	}
	return kernel.OutcomeHandled, nil
}
