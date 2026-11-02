package surface

import (
	"fmt"
	"slices"

	"github.com/caelyreth/neverland/engine/model"
)

// Checkbox describes a control replacing a list marker in projected text.
// Marker retains its textual spelling for copying and semantic editing.
type Checkbox struct {
	Node    model.NodeID
	Marker  model.UTF16Range
	Checked bool
	Label   string
}

// WithCheckboxes attaches ordered, non-overlapping controls to a flow.
func (f *Flow) WithCheckboxes(checkboxes []Checkbox) (*Flow, error) {
	previous := model.UTF16Offset(0)
	for _, checkbox := range checkboxes {
		if checkbox.Node == 0 || checkbox.Marker.Empty() || checkbox.Marker.Start < previous {
			return nil, fmt.Errorf("invalid checkbox marker")
		}
		if _, err := f.byteRange(checkbox.Marker); err != nil {
			return nil, err
		}
		previous = checkbox.Marker.End
	}
	next := *f
	next.checkboxes = slices.Clone(checkboxes)
	return &next, nil
}

// Checkboxes are the controls in projected text order.
func (f *Flow) Checkboxes() []Checkbox { return slices.Clone(f.checkboxes) }
