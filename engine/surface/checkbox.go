package surface

import (
	"fmt"
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

func (c Checkbox) Run() model.UTF16Range { return c.Marker }

func (c Checkbox) node() model.NodeID { return c.Node }

func (c Checkbox) validate(f *Flow, _ Block) error {
	if c.Node == 0 {
		return fmt.Errorf("a checkbox must name its block")
	}
	bytes, err := f.byteRange(c.Marker)
	if err != nil {
		return err
	}
	text := f.text[bytes.Start:bytes.End]
	if len(text) != c.Marker.Len() || strings.ContainsAny(text, "\r\n") {
		return fmt.Errorf("checkbox marker must be ASCII on one line")
	}
	return nil
}

// shifted moves a checkbox whole past an edit beside its marker; one
// within its marker takes it away.
func (c Checkbox) moved(units model.UTF16Offset) Object {
	c.Marker = by(c.Marker, units)
	return c
}

func (c Checkbox) shifted(e rangeEdit) (Object, bool) {
	marker, ok := e.whole(c.Marker)
	c.Marker = marker
	return c, ok
}
