package editor

import (
	"github.com/caelyreth/neverland/engine/kernel"
	"slices"
	"time"

	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

// The caret stands in a stack of places (Places). Leave pops exactly one
// of those inside its block, never entering one; Return goes back into the
// one just left; Dismiss leaves one as Leave does, or drops a selection;
// Select All selects the next place of blocks out, each press the next one
// that holds more than the selection does.

// widenedFrom is what the run of Select All that made the selection grew
// from, while it stands.
func (s *Session) widenedFrom() (model.TextSelection, bool) {
	if s.prior.widen == nil {
		return model.TextSelection{}, false
	}
	return *s.prior.widen, true
}

// PlaceKind is what kind of place the caret stands in.
type PlaceKind uint8

const (
	// PlacePair is inside a typed pair, between its opener and closer.
	PlacePair PlaceKind = iota + 1
	// PlaceEmphasis is inside a mark's text.
	PlaceEmphasis
	// PlaceFormula is in a formula's source, inline or a block's.
	PlaceFormula
	// PlaceBlock is the block the selection's head is in.
	PlaceBlock
	// PlaceGroup is a run of blocks the block nests in: a list, a quote.
	PlaceGroup
	// PlaceTable is the table the block is a cell of.
	PlaceTable
	// PlacePage is the whole page.
	PlacePage
)

// Place is one place around the selection: what it is and the projected
// range it covers. A place inside a block (pair, emphasis, formula) can be
// left, at Exit: Exit is where the caret goes, read semantically at exitAt
// when that is set. A place of blocks can be selected whole; its block
// interval owns the selection semantically, never by projected containment,
// since generated notation can put a caret outside the range its head's
// block shows.
type Place struct {
	Kind   PlaceKind
	Range  model.UTF16Range
	Exit   model.UTF16Offset
	exitAt *model.TextAnchor
	leaves bool

	firstBlock, lastBlock int
	ownsSelection         bool
}

// Leaves reports whether the caret can leave the place.
func (p Place) Leaves() bool { return p.leaves }

func (p Place) owns(first, last int) bool {
	return p.firstBlock <= first && last <= p.lastBlock
}

// Places are the places around the selection, innermost first: what the
// caret can leave inside its block, narrowest first, then the blocks it
// stands in out to the page.
func (s *Session) Places() []Place {
	return append(s.innerPlaces(), s.blockPlaces()...)
}

// innerPlaces are the places inside the caret's block that it can leave,
// narrowest first. A selection has none: it is left by dismissing it.
func (s *Session) innerPlaces() []Place {
	settings := s.navigationSettings()
	if !settings.Enabled || !s.presentation.VisualCollapsed() {
		return nil
	}
	selection := s.presentation.Selection
	node, ok := s.runtime.Snapshot().Document().Node(selection.Node())
	if !ok {
		return nil
	}
	head := selection.Head().OffsetUTF16()
	p := s.presentation.Projection
	project := func(offset model.UTF16Offset) (model.UTF16Offset, model.TextAnchor, bool) {
		anchor := model.NewTextAnchor(node.ID(), offset, model.AffinityRight, selection.Revision())
		target, err := p.ProjectAnchor(anchor)
		return target.RangeUTF16().Start, anchor, err == nil
	}
	places := make([]Place, 0, 4)
	if settings.Pairs && settings.PairsPolicyEnabled {
		for _, pair := range s.Pairs() {
			end := pair.Close + model.UTF16Offset(model.StringLenUTF16(pair.Closer))
			if !(pair.Open < head && head < end) {
				continue
			}
			open, _, okOpen := project(pair.Open)
			exit, anchor, okExit := project(end)
			if okOpen && okExit {
				places = append(places, Place{Kind: PlacePair, Range: model.NewUTF16Range(open, exit), Exit: exit, exitAt: &anchor, leaves: true})
			}
		}
	}
	if settings.Emphasis {
		active := activeMark(s.inline)
		for _, mark := range node.Marks() {
			// A formula is a place of its own, read from its source.
			text := mark.Range()
			if mark.Kind() == MathMark || !(active != nil && *active == mark.ID() || text.Start < head && head < text.End) {
				continue
			}
			start, _, okStart := project(text.Start)
			exit, anchor, okExit := project(text.End)
			if !okStart || !okExit {
				continue
			}
			// Leaving a mark lands after its generated closer, not at
			// the edge of its text.
			key := MarkerKeyForMark(mark.ID())
			segments := p.Segments()
			for index := len(segments) - 1; index >= 0; index-- {
				if attribute, found := segments[index].AttributeTarget(); found && attribute.Key() == key {
					exit = segments[index].ProjectedRange().End
					break
				}
			}
			places = append(places, Place{Kind: PlaceEmphasis, Range: model.NewUTF16Range(start, exit), Exit: exit, exitAt: &anchor, leaves: true})
		}
	}
	if flow, err := s.SurfaceFlow(); err == nil {
		if f, ok := sourceAt(flow, s.presentation.Caret.RangeUTF16().Start); ok {
			// A block has no place beside it of its own: it is left for
			// the line after it, or the one before it at the page's end.
			place := Place{Kind: PlaceFormula, Range: f.Range, Exit: f.Range.End, leaves: true}
			if f.Display {
				switch {
				case f.Range.End < flow.UTF16Len():
					place.Exit = f.Range.End + 1
				case f.Range.Start > 0:
					place.Exit = f.Range.Start - 1
				default:
					place.leaves = false
				}
			}
			if place.leaves {
				places = append(places, place)
			}
		}
	}
	slices.SortStableFunc(places, func(a, b Place) int { return int(a.Range.Len()) - int(b.Range.Len()) })
	return places
}

// blockPlaces are the places of blocks around the selection, narrowest
// first: its head's block, each group that block nests in from the deepest
// out, its table, and the page.
func (s *Session) blockPlaces() []Place {
	p := s.presentation.Projection
	page := Place{Kind: PlacePage, Range: model.NewUTF16Range(0, model.UTF16Offset(p.UTF16Len())), ownsSelection: true}
	blocks := DocumentBlocks(s.runtime.Snapshot().Document())
	selection := s.presentation.Selection
	at := func(id model.NodeID) int {
		return slices.IndexFunc(blocks, func(block *model.Node) bool { return block.ID() == id })
	}
	index, firstSelected := at(selection.Head().Node()), at(selection.Anchor().Node())
	if index < 0 || firstSelected < 0 {
		return []Place{page}
	}
	lastSelected := index
	if lastSelected < firstSelected {
		firstSelected, lastSelected = lastSelected, firstSelected
	}
	places := make([]Place, 0, 4)
	add := func(kind PlaceKind, first, last int, extent blockExtent) {
		from, fromOK := blockProjectedRange(p, blocks[first], extent)
		to, toOK := blockProjectedRange(p, blocks[last], extent)
		if !fromOK || !toOK {
			return
		}
		place := Place{Kind: kind, Range: model.NewUTF16Range(from.Start, to.End), firstBlock: first, lastBlock: last}
		place.ownsSelection = place.owns(firstSelected, lastSelected)
		places = append(places, place)
	}
	add(PlaceBlock, index, index, blockExtentInline)
	for _, run := range groupRuns(blocks, index) {
		add(PlaceGroup, run[0], run[1], blockExtentAll)
	}
	if table, ok := TableOf(s.runtime.Snapshot().Document(), blocks[index].ID()); ok {
		add(PlaceTable, at(table.First()), at(table.Last()), blockExtentAll)
	}
	page.firstBlock, page.lastBlock = 0, len(blocks)-1
	return append(places, page)
}

// blockExtent is how much of one block a selection scope includes.
type blockExtent uint8

const (
	// blockExtentInline keeps inline notation with its text, but leaves the
	// block's structural notation to the enclosing group or page.
	blockExtentInline blockExtent = iota
	// blockExtentAll includes structural notation and objects as well.
	blockExtentAll
)

// blockProjectedRange is a block's projected extent at one scope level.
// Inline notation belongs to the block's local scope; structural notation
// belongs only once selection widens to its group or page.
func blockProjectedRange(
	p *projection.Projection,
	block *model.Node,
	extent blockExtent,
) (model.UTF16Range, bool) {
	return projectedSegments(p, block, extent)
}

func projectedSegments(
	p *projection.Projection,
	block *model.Node,
	extent blockExtent,
) (model.UTF16Range, bool) {
	start, end, found := model.UTF16Offset(0), model.UTF16Offset(0), false
	for _, segment := range p.Segments() {
		owner, ok := segmentNode(segment)
		if !ok || owner != block.ID() {
			continue
		}
		if extent == blockExtentInline {
			if target, ok := segment.AttributeTarget(); ok {
				if !MarkerKeyIsInline(target.Key()) {
					continue
				}
			} else if _, ok := segment.SemanticTarget(); !ok {
				continue
			}
		}
		span := segment.ProjectedRange()
		if !found {
			start, found = span.Start, true
		}
		end = max(end, span.End)
	}
	return model.NewUTF16Range(start, end), found
}

// nextScope is the narrowest scope holding more than the selection.
func (s *Session) nextScope() (model.UTF16Range, bool) {
	selected, err := s.VisualSelection()
	if err != nil {
		return model.UTF16Range{}, false
	}
	scopes := s.blockPlaces()
	// An exact projected match is the selection already made by this
	// sequence, even when its semantic anchors have normalized into one
	// block. Step past it rather than treating that anchor as a new start.
	current := false
	for _, scope := range scopes {
		if scope.Range == selected {
			current = true
			continue
		}
		if current {
			return scope.Range, true
		}
	}
	if current {
		return model.UTF16Range{}, false
	}
	// A selection not matching a scope, as a caret in generated notation,
	// starts at the narrowest scope that owns its semantic endpoints, never
	// one inside it: a selected picture widens to the page, not narrowing
	// to its caption.
	for _, scope := range scopes {
		inside := !selected.Empty() && selected.Start <= scope.Range.Start && scope.Range.End <= selected.End
		holds := scope.Range.Start <= selected.Start && selected.End <= scope.Range.End
		if scope.ownsSelection && scope.Range != selected && !inside && holds {
			return scope.Range, true
		}
	}
	return model.UTF16Range{}, false
}

// scopeGuides mark the next scope while a scope is selected: pressing
// Select All again would select the area they mark.
func (s *Session) scopeGuides(time.Time) []Guide {
	selected, err := s.VisualSelection()
	if err != nil || selected.Empty() {
		return nil
	}
	for _, scope := range s.blockPlaces() {
		if scope.Range != selected {
			continue
		}
		next, ok := s.nextScope()
		if !ok {
			return nil
		}
		return []Guide{{
			Producer: surface.ProducerID(TextPolicyID),
			Command:  SelectAllCommandID,
			Area:     []model.UTF16Range{next},
		}}
	}
	return nil
}

// step is where a responder takes the caret: to a projected offset, read
// semantically at anchor when that is set.
type step struct {
	to        model.UTF16Offset
	anchor    *model.TextAnchor
	returning bool
	expires   *time.Time
	remember  bool
}

// leaveStep is where Leave goes: out of the innermost place inside the
// caret's block, else back to the last edit while that is offered. A guide
// shows this same step.
func (s *Session) leaveStep(now time.Time) (step, bool) {
	if places := s.innerPlaces(); len(places) > 0 {
		return step{to: places[0].Exit, anchor: places[0].exitAt, remember: true}, true
	}
	settings := s.navigationSettings()
	target, live := s.navigation.Returning()
	if !settings.Enabled || !settings.ReturnToEdit || !live || !now.Before(target.ExpiresAt) ||
		!s.presentation.VisualCollapsed() {
		return step{}, false
	}
	projected, err := s.presentation.Projection.ProjectAnchor(target.Anchor)
	if err != nil {
		return step{}, false
	}
	expires := target.ExpiresAt
	return step{to: projected.RangeUTF16().Start, anchor: &target.Anchor, returning: true, expires: &expires}, true
}

// returnStep is where Return goes: back into the place just left, else into
// the source of the inline formula just before the caret, at its end.
func (s *Session) returnStep() (step, bool) {
	caret := s.presentation.Caret.RangeUTF16()
	if inside := s.prior.inside; inside != nil {
		return step{to: *inside}, true
	}
	if !s.navigationSettings().Enabled || !caret.Empty() {
		return step{}, false
	}
	flow, err := s.SurfaceFlow()
	if err != nil {
		return step{}, false
	}
	if f, ok := formulaEndingAt(flow, caret.Start); ok && !f.Display {
		return step{to: f.Text.End}, true
	}
	return step{}, false
}

// take moves the caret by a step, remembering the place it left when the
// step leaves one.
func (s *Session) take(next step) error {
	inside := s.presentation.Caret.RangeUTF16().Start
	if next.anchor == nil {
		if err := s.SelectProjected(next.to, next.to); err != nil {
			return err
		}
	} else {
		if err := s.Select(model.NewTextSelection(*next.anchor, *next.anchor)); err != nil {
			return err
		}
		target, err := s.presentation.Projection.Target(model.EmptyUTF16Range(next.to))
		if err != nil {
			return err
		}
		s.presentation.Caret = target
		s.refreshInline()
	}
	if next.returning {
		s.navigation.ConsumeReturn()
	}
	if next.remember {
		s.follow(followUp{inside: &inside})
	}
	return nil
}

// leave is Leave: in a table, the next cell; else one place out.
func (s *Session) leave(now time.Time) (kernel.Outcome, error) {
	if outcome, inTable, err := s.stepCell(1); inTable || err != nil {
		return outcome, err
	}
	next, ok := s.leaveStep(now)
	if !ok {
		return kernel.OutcomePass, nil
	}
	return kernel.OutcomeHandled, s.take(next)
}

// enter is Return: in a table, the previous cell; else back into a place.
func (s *Session) enter() (kernel.Outcome, error) {
	if outcome, inTable, err := s.stepCell(-1); inTable || err != nil {
		return outcome, err
	}
	next, ok := s.returnStep()
	if !ok {
		return kernel.OutcomePass, nil
	}
	return kernel.OutcomeHandled, s.take(next)
}

// dismiss is Dismiss at a caret: one place out, as Leave goes, but never
// back to the last edit, which only Leave offers.
func (s *Session) dismissPlace() (kernel.Outcome, error) {
	if places := s.innerPlaces(); len(places) > 0 {
		return kernel.OutcomeHandled, s.take(step{to: places[0].Exit, anchor: places[0].exitAt, remember: true})
	}
	return kernel.OutcomeHandled, nil
}
