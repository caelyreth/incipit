package projection

import "github.com/caelyreth/neverland/engine/model"

// Patch is a replacement in previous and current UTF-16 surface coordinates.
// Patches are ordered and disjoint; flat consumers apply them in reverse.
type Patch struct {
	previous model.UTF16Range
	current  model.UTF16Range
}

func NewPatch(previous, current model.UTF16Range) Patch {
	return Patch{previous: previous, current: current}
}

func (p Patch) PreviousRange() model.UTF16Range { return p.previous }
func (p Patch) CurrentRange() model.UTF16Range  { return p.current }

type Delta struct {
	projection ProjectionID
	previous   Version
	current    Version
	patches    []Patch
}

func (d Delta) Projection() ProjectionID { return d.projection }
func (d Delta) PreviousVersion() Version { return d.previous }
func (d Delta) CurrentVersion() Version  { return d.current }
func (d Delta) Patches() []Patch         { return append([]Patch(nil), d.patches...) }

type Update struct {
	Projection *Projection
	Delta      Delta
}

func (p *Projection) PrepareAttachment(previous *Projection) (*Projection, error) {
	if p.id != previous.id {
		return nil, &ProjectionError{
			Kind:     ErrorWrongProjection,
			Expected: previous.id,
			Actual:   p.id,
		}
	}
	copy := *p
	copy.text = p.text
	copy.text.retainIfEqual(previous.text)
	return &copy, nil
}

func (p *Projection) PrepareUpdate(previous *Projection) (Update, error) {
	if p.id != previous.id {
		return Update{}, &ProjectionError{
			Kind:     ErrorWrongProjection,
			Expected: previous.id,
			Actual:   p.id,
		}
	}
	patches := p.text.changedRanges(previous.text)
	copy := *p
	if len(patches) == 0 {
		copy.text = previous.text
	}
	return Update{
		Projection: &copy,
		Delta: Delta{
			projection: copy.id,
			previous:   previous.version,
			current:    copy.version,
			patches:    patches,
		},
	}, nil
}
