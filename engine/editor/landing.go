package editor

import "github.com/caelyreth/neverland/engine/model"

// Landing is where the caret goes once a rule's edit is written: at Head in
// Node (the block the input targeted when zero), read in the draft as it
// stood after its first Through text edits. Whatever is written after those
// carries it.
type Landing struct {
	Node    model.NodeID
	Head    model.UTF16Offset
	Through int
}

// current reports whether the landing reads the draft as it stands after
// count edits, so its head is an offset in the text there.
func (l Landing) current(count int) bool { return l.Through == count }

// anchor is the landing as a caret anchor, its block defaulting to node,
// with the edits that still carry it.
func (l Landing) anchor(node model.NodeID, revision uint64, edits int) (model.TextAnchor, int) {
	if l.Node != 0 {
		node = l.Node
	}
	return model.NewTextAnchor(node, l.Head, model.AffinityRight, revision), min(l.Through, edits)
}

// Proposal is what the rules of one dispatch propose besides their edits.
// The session reads it once the dispatch succeeds; nothing else carries
// what a rule decided out of it.
type Proposal struct {
	landing Landing
	lands   bool
}

// Landing returns where the caret was proposed to go, if anywhere.
func (p *Proposal) Landing() (Landing, bool) { return p.landing, p.lands }

func (p *Proposal) land(landing Landing) { p.landing, p.lands = landing, true }

func (p *Proposal) reset() { *p = Proposal{} }
