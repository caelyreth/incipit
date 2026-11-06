package projection

import (
	"fmt"
	"slices"

	"github.com/caelyreth/neverland/engine/model"
)

// Role names what a run of generated notation stands for, in the terms of
// the profile that generated it: a list's marker, a formula's opening
// fence. The projection carries roles and does not read them, so its
// consumers find what notation stands for by asking, not by measuring its
// spelling. Zero is no role.
type Role uint16

// RoleSpan is a run of a projection's text a role names, in projected
// UTF-16 offsets, and the node and attribute key whose notation it is.
type RoleSpan struct {
	Range model.UTF16Range
	Role  Role
	Node  model.NodeID
	Key   string
}

// Part is a run of notation a builder appends with the role it plays.
type Part struct {
	Text string
	Role Role
}

// Roles are the role spans that overlap range16, in order.
func (p *Projection) Roles(range16 model.UTF16Range) []RoleSpan {
	start := searchRoles(p.roles, range16.Start)
	end := start
	for end < len(p.roles) && p.roles[end].Range.Start < range16.End {
		end++
	}
	return slices.Clone(p.roles[start:end])
}

// EachRole visits the role spans in order, without copying them, until fn
// returns false.
func (p *Projection) EachRole(fn func(span RoleSpan) bool) {
	for _, span := range p.roles {
		if !fn(span) {
			return
		}
	}
}

// RoleAt is the role span holding offset, its start included and its end
// not.
func (p *Projection) RoleAt(offset model.UTF16Offset) (RoleSpan, bool) {
	index := searchRoles(p.roles, offset)
	if index < len(p.roles) && p.roles[index].Range.Start <= offset && offset < p.roles[index].Range.End {
		return p.roles[index], true
	}
	return RoleSpan{}, false
}

// searchRoles is the first span ending past offset.
func searchRoles(roles []RoleSpan, offset model.UTF16Offset) int {
	low, high := 0, len(roles)
	for low < high {
		middle := (low + high) / 2
		if roles[middle].Range.End > offset {
			high = middle
		} else {
			low = middle + 1
		}
	}
	return low
}

// validateRoles checks that role spans are non-empty, in order, apart, and
// within the text.
func validateRoles(roles []RoleSpan, length int) error {
	previous := model.UTF16Offset(0)
	for _, span := range roles {
		if span.Role == 0 || span.Range.Empty() || span.Range.Start < previous || int(span.Range.End) > length {
			return fmt.Errorf("projection: role spans must be named, non-empty, ordered and apart")
		}
		previous = span.Range.End
	}
	return nil
}
