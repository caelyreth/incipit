package vocab

import (
	"fmt"
	"sync"

	"github.com/caelyreth/neverland/editor/doc"
)

var groups struct {
	sync.Mutex
	names []string
}

// Group is a highlight group: how text, notation, objects and decorations
// are styled. A group may link to a fallback.
type Group uint32

var groupFallback = struct {
	sync.Mutex
	of map[Group]Group
}{of: map[Group]Group{}}

// NewGroup declares a group linked to fallback (zero for none).
func NewGroup(name string, fallback Group) Group {
	groups.Lock()
	groups.names = append(groups.names, name)
	g := Group(len(groups.names))
	groups.Unlock()
	groupFallback.Lock()
	groupFallback.of[g] = fallback
	groupFallback.Unlock()
	return g
}

func (g Group) String() string {
	groups.Lock()
	defer groups.Unlock()
	if g == 0 || int(g) > len(groups.names) {
		return fmt.Sprintf("group(%d)", g)
	}
	return groups.names[g-1]
}

// Fallback is the group g links to, or zero.
func (g Group) Fallback() Group {
	groupFallback.Lock()
	defer groupFallback.Unlock()
	return groupFallback.of[g]
}

// The groups of generic editor states.
var (
	GroupText        = NewGroup("text", 0)
	GroupGhost       = NewGroup("notation.ghost", 0)
	GroupComposition = NewGroup("composition", 0)
	// GroupArmed styles what the next delete takes, armed by the first.
	GroupArmed = NewGroup("armed", 0)
	// GroupPairMatch styles the pair or mark a caret is inside.
	GroupPairMatch = NewGroup("pair.match", 0)
	// GroupScope styles the innermost textual scope at the caret.
	GroupScope = NewGroup("scope", 0)
	// GroupPulse styles text a rewrite or history changed.
	GroupPulse = NewGroup("feedback.pulse", 0)
)

// Phases and origin classes are the document's.
type (
	Phase       = doc.Phase
	OriginClass = doc.OriginClass
)
