package editor

import (
	"strings"
	"unicode/utf8"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

// A block's opening notation is tokens and gaps. A token is its marker
// glyphs, "##", "-", "1.", "- [x]", "> >": the caret stops before each of
// them (and in a checkbox's box), never after the last, and typing there
// edits the block's shape. A gap is layout, the indent before a nested item
// and the space after a marker: the caret skips it, and it never reads as
// text.

// tokenRole reports whether role is a marker's glyphs, not a gap.
func tokenRole(role projection.Role) bool {
	switch role {
	case roleHeading, roleQuote, roleBullet, roleOrdinal, roleCheckbox:
		return true
	}
	return false
}

// openingNotation is the notation opening a block that at stands in or at
// the end of: its segment, its block and its range.
func openingNotation(p *projection.Projection, at model.UTF16Offset) (model.NodeID, model.UTF16Range, bool) {
	for _, segment := range p.Segments() {
		target, ok := segment.AttributeTarget()
		if !ok || target.Key() != blockNotationKey {
			continue
		}
		if span := segment.ProjectedRange(); span.Start <= at && at <= span.End {
			return target.Node(), span, true
		}
	}
	return 0, model.UTF16Range{}, false
}

// tokenStops are the caret stops in a block's opening notation at span:
// before each glyph of its tokens, and in a checkbox's box.
func tokenStops(p *projection.Projection, span model.UTF16Range) []model.UTF16Offset {
	text, err := p.TextRange(span)
	if err != nil {
		return nil
	}
	var stops []model.UTF16Offset
	at, box := span.Start, false
	for _, r := range text {
		if role, ok := p.RoleAt(at); ok && tokenRole(role.Role) {
			switch {
			case r == '[':
				box = true
				stops = append(stops, at)
			case r == ']':
				box = false
			case r != ' ' || box:
				stops = append(stops, at)
			}
		}
		at += model.UTF16Offset(utf16Len(r))
	}
	return stops
}

func utf16Len(r rune) int {
	if r >= 0x10000 {
		return 2
	}
	return 1
}

// tokenStop reports whether at is a caret stop in a token.
func tokenStop(p *projection.Projection, at model.UTF16Offset) bool {
	_, span, ok := openingNotation(p, at)
	if !ok || at == span.End {
		return false
	}
	for _, stop := range tokenStops(p, span) {
		if stop == at {
			return true
		}
	}
	return false
}

// tokenStep is where a horizontal step from caret goes among a block's
// token stops and its text's start, the gap after the token skipped. It
// reports false where the step leaves the notation, for motion to go on as
// it does anywhere.
func tokenStep(p *projection.Projection, caret model.UTF16Offset, backward bool) (model.UTF16Offset, bool) {
	_, span, ok := openingNotation(p, caret)
	if !ok {
		return 0, false
	}
	stops := append(tokenStops(p, span), span.End)
	for i, stop := range stops {
		if stop != caret {
			continue
		}
		switch {
		case backward && i > 0:
			return stops[i-1], true
		case !backward && i+1 < len(stops):
			return stops[i+1], true
		}
		return 0, false
	}
	return 0, false
}

// absorbing is text the next keystroke would type that the last reshape
// already wrote: the "." of a "1." typed digit by digit.
type absorbing struct {
	text     string
	caret    model.UTF16Offset
	revision uint64
}

// absorbs takes text from what an earlier step already wrote, reporting
// whether it did: typing what was written is typing nothing, one glyph for
// one, while the caret and page stay as that step left them.
func (s *Session) absorbs(text string) bool {
	a := s.absorb
	if a == nil || text == "" || !strings.HasPrefix(a.text, text) || a.revision != s.runtime.Revision() ||
		!s.presentation.VisualCollapsed() || s.presentation.Caret.RangeUTF16().Start != a.caret {
		return false
	}
	if a.text = a.text[len(text):]; a.text == "" {
		s.absorb = nil
	}
	return true
}

// expect records text the next keys absorb, from the caret as it stands.
func (s *Session) expect(text string) {
	s.absorb = nil
	if text != "" {
		s.absorb = &absorbing{text: text, caret: s.presentation.Caret.RangeUTF16().Start, revision: s.runtime.Revision()}
	}
}

// typeAtToken types text with the caret at a token stop: a block's own
// glyph steps its level up, another kind's opening glyph re-shapes it (a
// digit over "-" makes it numbered), and a space or x in a checkbox's box
// sets it. It reports false for text a token does not take, which edits
// the notation as text does anywhere.
func (s *Session) typeAtToken(text string) (EditingOutcome, bool, error) {
	p := s.presentation.Projection
	caret := s.presentation.Caret.RangeUTF16()
	r, size := utf8.DecodeRuneInString(text)
	if !s.presentation.VisualCollapsed() || size != len(text) || !tokenStop(p, caret.Start) {
		return EditingOutcome{}, false, nil
	}
	id, span, _ := openingNotation(p, caret.Start)
	node, ok := s.runtime.Snapshot().Document().Node(id)
	if !ok {
		return EditingOutcome{}, false, nil
	}
	shape := shapeOf(node)
	desc := shape.desc()
	next, land, absorb := shape, span.End, ""
	inBox := false
	if desc.Checks {
		if before, err := p.TextRange(model.NewUTF16Range(caret.Start-1, caret.Start)); err == nil && before == "[" {
			inBox = true
		}
	}
	switch opened, opens := glyphOpener(r); {
	case inBox && (r == ' ' || r == 'x' || r == 'X'):
		next.checked = r != ' ' || !shape.checked
		land = caret.Start
	case inBox:
		return EditingOutcome{}, false, nil
	case opens && opened == shape.kind:
		switch {
		case desc.Leveled:
			next.level = min(6, shape.level+1)
			land = caret.Start + 1
		case desc.Group == "quote":
			next.depth = min(maxDepth, shape.depth+1)
			land = caret.Start
		default:
			// Another of a list's own markers leaves it as it is.
			return EditingOutcome{Outcome: kernel.OutcomeHandled}, true, nil
		}
	case opens:
		next = blockShape{kind: opened, level: 1}
		if describe(opened).Group == desc.Group && desc.Group != "" {
			next.depth = shape.depth
		}
		if describe(opened).Numbered {
			absorb = ". "
		}
	default:
		return EditingOutcome{}, false, nil
	}
	notation, _ := blockNotation(next, 1)
	edit, err := FromProjectedRange(p, span, notation, kernel.EventOriginSemantic)
	if err != nil {
		return EditingOutcome{}, true, err
	}
	outcome, err := s.applyProjectedEdit(edit)
	if err != nil || outcome.Outcome != kernel.OutcomeHandled {
		return outcome, true, err
	}
	if land != span.End {
		err = s.SelectProjected(land, land)
	} else if _, after, ok := openingNotation(s.presentation.Projection, span.Start); ok {
		err = s.SelectProjected(after.End, after.End)
	}
	s.expect(absorb)
	return outcome, true, err
}

// glyphOpener is the kind a glyph typed at a token opens.
func glyphOpener(r rune) (model.NodeTypeID, bool) {
	for _, desc := range kindTable {
		for _, glyph := range desc.Glyphs {
			if glyph == r {
				return desc.Kind, true
			}
		}
	}
	return "", false
}
