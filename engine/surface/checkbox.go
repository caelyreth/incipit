package surface

import (
	"fmt"
	"slices"
	"sort"
	"strings"

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

// WithCheckboxes attaches ordered controls, at most one on each line, to a flow.
func (f *Flow) WithCheckboxes(checkboxes []Checkbox) (*Flow, error) {
	previous := model.UTF16Offset(0)
	previousByte := 0
	nodes := make(map[model.NodeID]bool, len(checkboxes))
	for index, checkbox := range checkboxes {
		if checkbox.Node == 0 || checkbox.Marker.Empty() || checkbox.Marker.Start < previous {
			return nil, fmt.Errorf("invalid checkbox marker")
		}
		bytes, err := f.byteRange(checkbox.Marker)
		if err != nil {
			return nil, err
		}
		text := f.text[bytes.Start:bytes.End]
		if len(text) != checkbox.Marker.Len() || strings.ContainsAny(text, "\r\n") {
			return nil, fmt.Errorf("checkbox marker must be ASCII on one line")
		}
		if nodes[checkbox.Node] || index > 0 && !strings.Contains(f.text[previousByte:bytes.Start], "\n") {
			return nil, fmt.Errorf("checkboxes must have distinct nodes and lines")
		}
		nodes[checkbox.Node] = true
		previous = checkbox.Marker.End
		previousByte = int(bytes.End)
	}
	next := *f
	next.checkboxes = slices.Clone(checkboxes)
	return &next, nil
}

// Checkboxes are the controls in projected text order.
func (f *Flow) Checkboxes() []Checkbox { return slices.Clone(f.checkboxes) }

// HasCheckboxes reports whether the flow contains any controls.
func (f *Flow) HasCheckboxes() bool { return len(f.checkboxes) > 0 }

// CheckboxAfter finds the first marker ending after offset.
func (f *Flow) CheckboxAfter(offset model.UTF16Offset) (Checkbox, bool) {
	index := sort.Search(len(f.checkboxes), func(index int) bool {
		return f.checkboxes[index].Marker.End > offset
	})
	if index == len(f.checkboxes) {
		return Checkbox{}, false
	}
	return f.checkboxes[index], true
}
