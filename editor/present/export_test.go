package present

import "fmt"

// AccessMismatch reports the first stop whose accessible offset the
// index places other than the scan does.
func AccessMismatch(p *Presentation) error {
	parts, _, u := p.accessParts()
	x := newAccessIndex(parts, u)
	for surface, s := range x.surfaces {
		if !s.ordered {
			return fmt.Errorf("surface %d's parts are out of order", surface)
		}
	}
	for i, st := range p.Stops {
		if got, want := x.offset(int(st.Surface), st.Offset()), accessibleOffset(parts, int(st.Surface), st.Offset(), u); got != want {
			return fmt.Errorf("stop %d (surface %d, offset %d): index %d, scan %d", i, st.Surface, st.Offset(), got, want)
		}
	}
	return nil
}
