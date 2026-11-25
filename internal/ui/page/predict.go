package page

import (
	"slices"
	"strings"
	"time"

	"github.com/caelyreth/neverland/internal/app/keymap"
	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/egoist/mygo/ui"
)

const (
	suggEnterDuration = 140 * time.Millisecond
	suggExitDuration  = 105 * time.Millisecond
)

type suggSnap struct {
	valid   bool
	groups  []vocab.Group
	shaped  *ui.TextLayout
	textX   float32
	textY   float32
	tabX    float32
	tabMidY float32
}

type leavingSugg struct {
	shaped    *ui.TextLayout
	groups    []vocab.Group
	textX     float32
	textY     float32
	tabX      float32
	tabMidY   float32
	start     time.Time
	fromAlpha float32
	alpha     float32
}

type suggMotion struct {
	active  *policy.Suggestion
	start   time.Time
	alpha   float32
	last    suggSnap
	leaving *leavingSugg
}

func (el *Element) stepSuggestion(pl *pageLayout, focused bool, now time.Time, animate bool) bool {
	ed := el.Editor
	m := &el.sugg
	var cur *policy.Suggestion
	if focused && ed != nil {
		cur = ed.deco.Suggestion
	}
	same := cur != nil && m.active != nil && *cur == *m.active
	trimmed := cur != nil && m.active != nil &&
		cur.At.Ref() == m.active.At.Ref() &&
		cur.At.Offset > m.active.At.Offset &&
		strings.HasSuffix(m.active.Text, cur.Text)

	if cur != nil && m.active == nil {
		m.start = now
	} else if m.active != nil && !same && !trimmed {
		accepted := cur == nil && ed != nil && suggestionAccepted(ed.Engine.Snapshot(), *m.active)
		if animate && m.last.valid && m.last.shaped != nil && m.alpha > 0.01 && !accepted {
			m.leaving = &leavingSugg{
				shaped:    m.last.shaped,
				groups:    m.last.groups,
				textX:     m.last.textX,
				textY:     m.last.textY,
				tabX:      m.last.tabX,
				tabMidY:   m.last.tabMidY,
				start:     now,
				fromAlpha: m.alpha,
				alpha:     m.alpha,
			}
		}
		if cur != nil {
			m.start = now
		} else {
			m.start = time.Time{}
			m.alpha = 0
		}
	}

	if cur != nil {
		c := *cur
		m.active = &c
		if !animate {
			m.alpha = 1
		} else {
			elapsed := now.Sub(m.start)
			if elapsed < 0 {
				elapsed = 0
			}
			t := min(float32(elapsed)/float32(suggEnterDuration), 1)
			m.alpha = components.Smoothstep(t)
		}
		m.last = el.snapshotSuggestion(pl)
	} else {
		m.active = nil
		m.alpha = 0
		m.last.valid = false
	}

	if m.leaving != nil {
		if !animate {
			m.leaving = nil
		} else {
			elapsed := now.Sub(m.leaving.start)
			if elapsed < 0 {
				elapsed = 0
			}
			t := min(float32(elapsed)/float32(suggExitDuration), 1)
			m.leaving.alpha = m.leaving.fromAlpha * (1 - components.Smoothstep(t))
			if t >= 1 || m.leaving.alpha <= 0.001 {
				m.leaving = nil
			}
		}
	}
	return (cur != nil && animate && m.alpha < 1) || m.leaving != nil
}

func suggestionAccepted(snap *doc.Snapshot, prev policy.Suggestion) bool {
	if snap == nil || prev.Text == "" {
		return false
	}
	n, ok := snap.Node(prev.At.Node)
	if !ok || prev.At.Slot >= n.NumSlots() || !n.Slot(prev.At.Slot).IsInline() {
		return false
	}
	in := n.Slot(prev.At.Slot).Content()
	end := prev.At.Offset + len(prev.Text)
	return prev.At.Offset >= 0 && end <= in.Len() && in.FlatRange(prev.At.Offset, end) == prev.Text
}

func (el *Element) snapshotSuggestion(pl *pageLayout) suggSnap {
	if pl == nil {
		return suggSnap{}
	}
	for _, u := range pl.units {
		for _, s := range u.surfaces {
			if !s.shown || !s.hasSugg {
				continue
			}
			for _, sg := range s.segs {
				if sg.kind != segSuggestion || sg.segFace == nil || sg.shaped == nil {
					continue
				}
				line, _, _ := s.lineOf(sg.u0)
				r0 := s.caretRect(sg.u0)
				r1 := s.caretRect(s.suggEnd)
				gb := float32(0)
				if gl := sg.shaped.Lines(); len(gl) > 0 {
					gb = gl[0].Baseline
				}
				return suggSnap{
					valid:   true,
					groups:  slices.Clone(sg.groups),
					shaped:  sg.shaped,
					textX:   u.x + s.x + r0.X,
					textY:   u.y + s.y + line.Baseline - gb,
					tabX:    u.x + s.x + r1.X + 4,
					tabMidY: u.y + s.y + r1.Y + r1.H/2,
				}
			}
		}
	}
	return suggSnap{}
}

// acceptKeys are the caps of the key that takes a suggestion: the key of
// the command leaving a scope (vocab.Leave), which the spelling policy
// answers at a suggestion, as the keymap holds it; none where it has no
// key. They are worked out again only as the keymap changes.
func (el *Element) acceptKeys() []string {
	km := el.keys()
	if el.accept.keys == km {
		return el.accept.caps
	}
	el.accept.keys, el.accept.caps = km, nil
	leave := vocab.Leave{}.Kind()
	for _, c := range el.Editor.Engine.Program().Catalog().Commands {
		if c.Intent == nil || c.Intent.Kind() != leave {
			continue
		}
		if chord, ok := km.ChordFor(keymap.CommandID(c.ID)); ok {
			el.accept.caps = keymap.Keycaps(chord, keymap.PlatformCurrent)
			break
		}
	}
	return el.accept.caps
}

type pendingSuggestion struct {
	seq     uint64
	version uint64
	at      doc.Pos
	text    string
}

func (ed *Editor) setWake(fn func()) {
	ed.mu.Lock()
	ed.Wake = fn
	ed.mu.Unlock()
}

func (ed *Editor) wake() func() {
	ed.mu.Lock()
	fn := ed.Wake
	ed.mu.Unlock()
	return fn
}

// cancelPredict lets an answer still to come go: the writer did
// something since it was asked for.
func (ed *Editor) cancelPredict() {
	ed.mu.Lock()
	ed.predictSeq++
	ed.pendingSugg = nil
	ed.mu.Unlock()
}

// predict asks the host for the prediction the spelling policy asked for,
// once typing rested (its deadline); the answer waits for the next frame,
// which drops it if anything happened meanwhile.
func (ed *Editor) predict(ask profile.PredictAsk, version uint64) {
	predict := ed.Predict
	if predict == nil || ask.Text == "" {
		return
	}
	ed.mu.Lock()
	ed.predictSeq++
	seq := ed.predictSeq
	ed.pendingSugg = nil
	ed.mu.Unlock()
	predict(ask.Text, len(ask.Text), func(sugg string) {
		if sugg == "" {
			return
		}
		ed.mu.Lock()
		if ed.predictSeq != seq {
			ed.mu.Unlock()
			return
		}
		ed.pendingSugg = &pendingSuggestion{seq: seq, version: version, at: ask.At, text: sugg}
		wake := ed.Wake
		ed.mu.Unlock()
		if wake != nil {
			wake()
		}
	})
}

func (ed *Editor) flushSuggestion(now time.Time) {
	ed.mu.Lock()
	p := ed.pendingSugg
	seq := ed.predictSeq
	ed.pendingSugg = nil
	ed.mu.Unlock()
	if p == nil || p.seq != seq || ed.Engine.Version() != p.version {
		return
	}
	v := ed.view()
	if v.Composition != nil || v.Selection.Kind != doc.SelectText || !v.Selection.IsCollapsed() || v.Selection.Head.Pos != p.at {
		return
	}
	ed.Glance(vocab.Input{Intent: profile.SetSuggestion{At: p.at, Text: p.text}, At: now})
}
