package editor

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

// InlineState retains the mark that owns the caret. It deliberately stores
// identity rather than a derived range so later edits cannot silently retarget
// marker typing to a different mark.
type InlineState struct {
	active    *model.MarkID
	hasActive bool
}

// Active returns the remembered mark identity.
func (s InlineState) Active() (model.MarkID, bool) {
	return derefMark(s.active), s.hasActive
}

// Activate replaces the remembered mark identity.
func (s *InlineState) Activate(id *model.MarkID) {
	if id == nil {
		s.active = nil
		s.hasActive = false
		return
	}
	value := *id
	s.active = &value
	s.hasActive = true
}

// SelectProjected activates the caret's membership: the innermost mark
// whose generated opener and closer bracket the projected head.
func (s *InlineState) SelectProjected(
	node *model.Node,
	p *projection.Projection,
	head model.UTF16Offset,
) {
	s.Activate(caretMembership(node, p, head))
}

// MarkerActionKind identifies one semantic marker-typing operation.
type MarkerActionKind uint8

const (
	MarkerCreate MarkerActionKind = iota + 1
	MarkerUpgrade
	MarkerExit
	MarkerFormat
	// MarkerEnclose closes a span a paragraph opened with a marker typed as
	// text (heldOpener): the opener goes, and what followed it takes the
	// mark.
	MarkerEnclose
	// MarkerFormulaBlock makes a paragraph holding only an empty formula,
	// a second dollar typed in it, a formula block.
	MarkerFormulaBlock
)

// MarkerAction is a checked, pure marker-typing plan. It is deliberately
// separate from EngineInvoker: the kernel can execute the document mutation,
// while selection/exit facts remain with the caller.
type MarkerAction struct {
	Handled  bool
	Kind     MarkerActionKind
	Mark     model.MarkID
	MarkKind model.MarkKind
	Node     model.NodeID
	Range    model.UTF16Range
	HasMark  bool
	Level    uint8
	Head     model.UTF16Offset
	Exited   bool
	NoOp     bool
	Consumed bool
}

// PlanMarkerTypes interprets one typed inline marker ("*" or "_", "`",
// "~", "$") against the active mark. Non-marker text, non-typing origins
// and text typed where it is literal pass through: inside code or a
// formula, and backticks at a paragraph's start, where they may open a
// fence. A dollar opens a formula as it is typed, closed already; $$ alone
// in a paragraph opens a formula block.
func PlanMarkerTypes(
	node *model.Node,
	input model.UTF16Range,
	text string,
	active *model.MarkID,
	triggers writingTriggers,
	origin kernel.EventOrigin,
) (MarkerAction, error) {
	action := MarkerAction{Node: nodeID(node), Range: input, Head: input.End}
	if node == nil || origin != kernel.EventOriginTyping {
		return action, nil
	}
	kind, top, ok := triggers.markerKind(text)
	if !ok || literalBlock(node) {
		return action, nil
	}
	// A dollar after a backslash is one, as TeX writes it, opening and
	// closing nothing.
	if kind == MathMark {
		content := node.Text().String()
		before := content[:model.UTF16OffsetToByteClamped(content, input.Start)]
		if slashes := len(before) - len(strings.TrimRight(before, "\\")); slashes%2 == 1 {
			return action, nil
		}
	}
	// Within code or a formula only its own marker is notation, the one
	// closing it.
	if literal, ok := literalMark(node, active); ok && kind != literal {
		return action, nil
	}
	// Backticks all a paragraph holds may be opening a fence, and stay
	// text; what follows one of them is taken as written until a second
	// closes it as code, or it opens no fence.
	if opener, ok := heldOpener(node, active, text); ok && input.Empty() {
		content := node.Text().String()
		before := content[:model.UTF16OffsetToByteClamped(content, input.Start)]
		switch {
		case strings.Trim(before, opener) == "":
			return action, nil
		case text != opener || openers(before, opener) != 1 || kind != CodeMark:
			return action, nil
		}
		return MarkerAction{
			Handled:  true,
			Kind:     MarkerEnclose,
			MarkKind: kind,
			Node:     node.ID(),
			Range:    model.NewUTF16Range(0, input.Start),
			Level:    1,
			Head:     input.End,
			Consumed: true,
		}, nil
	}
	if err := node.ValidateTextRangeUTF16(input.Start, input.End); err != nil {
		return MarkerAction{}, err
	}
	var current *model.InlineMark
	if active != nil {
		for _, mark := range node.Marks() {
			if mark.ID() == *active && mark.Kind() == kind {
				value := mark
				current = &value
				break
			}
		}
	}
	// Within code or a formula a marker short of its end is text, as TeX's
	// \$ is.
	if current != nil && (kind == CodeMark || kind == MathMark) && current.Range().End != input.Start {
		return action, nil
	}
	if current != nil && input.Empty() && current.Range().End == input.Start {
		// $$ alone in a paragraph opens a formula block.
		if kind == MathMark && current.Range().Empty() && node.Kind() == ParagraphNode && blockLength(node) == 0 {
			return MarkerAction{
				Handled:  true,
				Kind:     MarkerFormulaBlock,
				Node:     node.ID(),
				Mark:     current.ID(),
				HasMark:  true,
				Consumed: true,
			}, nil
		}
		if !current.Range().Empty() {
			return MarkerAction{
				Handled:  true,
				Kind:     MarkerExit,
				Node:     node.ID(),
				Range:    input,
				Mark:     current.ID(),
				HasMark:  true,
				Level:    current.Level(),
				Head:     input.End,
				Exited:   true,
				Consumed: true,
			}, nil
		}
		// Emphasis goes from emphasis to strong and back to none; a mark
		// at the highest level the profile allows stays as it is.
		next := current.Level() + 1
		if current.Kind() == EmphasisMark && current.Level() == 2 {
			next = 0
		}
		if next > top {
			return MarkerAction{
				Handled:  true,
				Kind:     MarkerUpgrade,
				Node:     node.ID(),
				Range:    input,
				Mark:     current.ID(),
				HasMark:  true,
				Level:    current.Level(),
				Head:     input.End,
				NoOp:     true,
				Consumed: true,
			}, nil
		}
		return MarkerAction{
			Handled:  true,
			Kind:     MarkerUpgrade,
			Node:     node.ID(),
			Range:    input,
			Mark:     current.ID(),
			HasMark:  true,
			Level:    next,
			Head:     input.End,
			Consumed: true,
		}, nil
	}
	if input.Empty() {
		return MarkerAction{
			Handled:  true,
			Kind:     MarkerCreate,
			MarkKind: kind,
			Node:     node.ID(),
			Range:    input,
			Level:    1,
			Head:     input.End,
			Consumed: true,
		}, nil
	}
	return MarkerAction{
		Handled:  true,
		Kind:     MarkerFormat,
		MarkKind: kind,
		Node:     node.ID(),
		Range:    input,
		Level:    1,
		Head:     input.End,
		Consumed: true,
	}, nil
}

// openers counts the markers in text, a dollar after a backslash not one.
func openers(text, marker string) int {
	count := 0
	for index := strings.Index(text, marker); index >= 0; {
		slashes := len(text[:index]) - len(strings.TrimRight(text[:index], "\\"))
		if marker != "$" || slashes%2 == 0 {
			count++
		}
		next := strings.Index(text[index+len(marker):], marker)
		if next < 0 {
			break
		}
		index += len(marker) + next
	}
	return count
}

// heldOpener is the backtick a paragraph opens with, typed as
// text in case it opened a fence, while no mark is in it to say otherwise;
// in an empty paragraph, typed is about to be one.
func heldOpener(node *model.Node, active *model.MarkID, typed string) (string, bool) {
	if node == nil || node.Kind() != ParagraphNode || active != nil || node.MarkCount() > 0 {
		return "", false
	}
	content := node.Text().String()
	if content == "" {
		content = typed
	}
	if strings.HasPrefix(content, "`") {
		return "`", true
	}
	return "", false
}

// Apply writes a marker action to a document draft. Caller-owned selection
// state must still consume Exited/Head after the transaction succeeds.
func (a MarkerAction) Apply(draft *model.DocumentDraft) error {
	if draft == nil || !a.Handled || a.Exited || a.NoOp {
		return nil
	}
	switch a.Kind {
	case MarkerCreate:
		_, err := draft.AddMark(a.Node, a.Range, a.MarkKind, a.Level)
		return err
	case MarkerUpgrade:
		if !a.HasMark {
			return fmt.Errorf("marker upgrade has no mark identity")
		}
		return draft.SetMarkLevel(a.Node, a.Mark, a.Level)
	case MarkerFormat:
		return ToggleMark(draft, a.Node, a.Range, a.MarkKind, a.Level)
	case MarkerFormulaBlock:
		if err := draft.SetMarkLevel(a.Node, a.Mark, 0); err != nil {
			return err
		}
		return shapeBlock(draft, a.Node, blockShape{kind: MathNode})
	case MarkerEnclose:
		// The opener is one unit of text at the range's start.
		if err := draft.ReplaceTextMarked(a.Node, a.Range.Start, a.Range.Start+1, "", nil); err != nil {
			return err
		}
		_, err := draft.AddMark(a.Node, model.NewUTF16Range(a.Range.Start, a.Range.End-1), a.MarkKind, a.Level)
		return err
	default:
		return fmt.Errorf("unknown marker action %d", a.Kind)
	}
}

// MarkerEffect resolves the active mark after a create/format action. It
// prefers the newest exact-range mark, matching Neverland's marker_effect.
func MarkerEffect(node *model.Node, range16 model.UTF16Range) (model.MarkID, bool) {
	if node == nil {
		return 0, false
	}
	var best *model.InlineMark
	for _, mark := range node.Marks() {
		if mark.Range() != range16 {
			continue
		}
		value := mark
		if best == nil || value.ID() > best.ID() {
			best = &value
		}
	}
	if best == nil {
		return 0, false
	}
	return best.ID(), true
}

// InlineInputStyle resolves the style text typed at the caret takes: its
// block's, with the active mark's.
func InlineInputStyle(
	node *model.Node,
	active *model.MarkID,
) surface.TextStyle {
	style := surface.TextStyle{Color: surface.ColorText}
	if node == nil {
		return style
	}
	style = blockStyle(node)
	if active != nil {
		for _, mark := range node.Marks() {
			if mark.ID() == *active {
				markStyle(&style, mark)
				break
			}
		}
	}
	return style
}

// InlineStyleRuns converts projection segments to contiguous byte style
// runs. Every segment takes its block's style; generated notation is muted
// within it, and editable text takes the marks enclosing it.
func InlineStyleRuns(
	p *projection.Projection,
	document model.DocumentSnapshot,
) ([]surface.StyleRun, error) {
	if p == nil {
		return nil, &projection.ProjectionError{Kind: projection.ErrorNoCorrespondence}
	}
	nodes := make(map[model.NodeID]*model.Node)
	lookup := func(id model.NodeID) (*model.Node, bool) {
		if node, ok := nodes[id]; ok {
			return node, true
		}
		node, ok := document.Node(id)
		if ok {
			nodes[id] = node
		}
		return node, ok
	}
	text := p.Text()
	byteOffset := 0
	runs := make([]surface.StyleRun, 0, 8)
	// Generated text between blocks has no block of its own; it goes as
	// the block before it does, so a line break never changes its size.
	current := surface.TextStyle{Color: surface.ColorText}
	var failure error
	p.EachSegment(func(_ int, segment projection.Segment) bool {
		var owner *model.Node
		if target, ok := segment.SemanticTarget(); ok {
			owner, ok = lookup(target.Node())
			if !ok {
				failure = &projection.ProjectionError{Kind: projection.ErrorNoCorrespondence}
				return false
			}
		} else if target, ok := segment.AttributeTarget(); ok {
			owner, _ = lookup(target.Node())
		}
		if owner != nil {
			current = blockStyle(owner)
		}
		units := segment.ProjectedRange().Len()
		if units == 0 {
			return true
		}
		end := byteOffset
		seen := 0
		for end < len(text) {
			r, size := utf8.DecodeRuneInString(text[end:])
			if size <= 0 {
				failure = &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
				return false
			}
			seen += model.RuneUTF16Len(r)
			end += size
			if seen >= units {
				break
			}
		}
		if seen != units {
			failure = &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
			return false
		}
		style := current
		if target, ok := segment.SemanticTarget(); ok {
			targetRange := target.RangeUTF16()
			for _, mark := range owner.Marks() {
				if mark.Range().Start <= targetRange.Start && targetRange.End <= mark.Range().End {
					markStyle(&style, mark)
				}
			}
		} else if !ok {
			style = notationStyle(style)
		}
		pushSurfaceRun(&runs, surface.StyleRun{
			Bytes: model.NewByteRange(model.ByteOffset(byteOffset), model.ByteOffset(end)),
			Style: style,
		})
		byteOffset = end
		return true
	})
	if failure != nil {
		return nil, failure
	}
	if byteOffset != len(text) {
		return nil, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
	}
	return runs, nil
}

// notationStyle is how notation and generated text show in text of style:
// muted, at its size and in its face, but not as its marks are.
func notationStyle(style surface.TextStyle) surface.TextStyle {
	style.Color = surface.ColorMuted
	style.Bold, style.Italic, style.Strike = false, false, false
	return style
}

// blockStyle is how a block's text shows: the Writing Profile's use of the
// surface's styles. Headings are titles by level and strong; quote lines
// lean; code, and a formula's source, are set in the mono face; a caption
// reads as its picture's.
func blockStyle(node *model.Node) surface.TextStyle {
	style := surface.TextStyle{Color: surface.ColorText}
	switch shape := shapeOf(node); shape.kind {
	case HeadingNode:
		style.Size = titleSize(shape.level)
		style.Bold = true
		style.Color = surface.ColorStrong
	case QuoteNode:
		style.Italic = true
	case CodeNode, MathNode:
		codeStyle(&style)
	case RuleNode:
		style.Color = surface.ColorSubtle
	case ImageNode:
		style.Size = surface.SizeCaption
		style.Color = surface.ColorMuted
	}
	return style
}

// markStyle adds what a mark shows to style.
func markStyle(style *surface.TextStyle, mark model.InlineMark) {
	switch mark.Kind() {
	case EmphasisMark:
		switch mark.Level() {
		case 1:
			style.Italic = !style.Italic
		case 2:
			style.Bold = true
		}
	case CodeMark, MathMark:
		codeStyle(style)
	case StrikeMark:
		style.Strike = true
	}
}

// codeStyle sets code in the mono face, at the size of the text around
// it, as a title's code is the title's.
func codeStyle(style *surface.TextStyle) {
	style.Face = surface.FaceMono
}

// titleSize is the title a heading of a level is: one per level, the last
// two as the page's text.
func titleSize(level int) surface.SizeRole {
	switch level {
	case 1:
		return surface.SizeTitle1
	case 2:
		return surface.SizeTitle2
	case 3:
		return surface.SizeTitle3
	case 4:
		return surface.SizeTitle4
	}
	return surface.SizeText
}

func pushSurfaceRun(runs *[]surface.StyleRun, next surface.StyleRun) {
	if next.Bytes.Empty() {
		return
	}
	if len(*runs) > 0 {
		last := &(*runs)[len(*runs)-1]
		if last.Style == next.Style && last.Bytes.End == next.Bytes.Start {
			last.Bytes.End = next.Bytes.End
			return
		}
	}
	*runs = append(*runs, next)
}

func derefMark(id *model.MarkID) model.MarkID {
	if id == nil {
		return 0
	}
	return *id
}

func nodeID(node *model.Node) model.NodeID {
	if node == nil {
		return 0
	}
	return node.ID()
}
