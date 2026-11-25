package page

import (
	"strings"
	"time"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/egoist/mygo/ui"
)

const (
	defaultPredictDelay = 350 * time.Millisecond
	suggEnterDuration   = 240 * time.Millisecond
	suggExitDuration    = 180 * time.Millisecond
	suggSlideX          = float32(6)
	suggExitSlideX      = float32(3)
)

type suggSnap struct {
	valid   bool
	groups  []vocab.Group
	shaped  *ui.TextLayout
	textX   float32
	textY   float32
	lineY   float32
	lineH   float32
	clipW   float32
	tabX    float32
	tabMidY float32
}

type leavingSugg struct {
	shaped    *ui.TextLayout
	groups    []vocab.Group
	textX     float32
	textY     float32
	lineY     float32
	lineH     float32
	clipW     float32
	tabX      float32
	tabMidY   float32
	start     time.Time
	fromAlpha float32
	fromDX    float32
	alpha     float32
	dx        float32
}

type suggMotion struct {
	active  *policy.Suggestion
	start   time.Time
	alpha   float32
	dx      float32
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
				lineY:     m.last.lineY,
				lineH:     m.last.lineH,
				clipW:     m.last.clipW,
				tabX:      m.last.tabX,
				tabMidY:   m.last.tabMidY,
				start:     now,
				fromAlpha: m.alpha,
				fromDX:    m.dx,
				alpha:     m.alpha,
				dx:        m.dx,
			}
		}
		if cur != nil {
			m.start = now
		} else {
			m.start = time.Time{}
			m.alpha = 0
			m.dx = 0
		}
	}

	if cur != nil {
		c := *cur
		m.active = &c
		if !animate {
			m.alpha, m.dx = 1, 0
		} else {
			elapsed := now.Sub(m.start)
			if elapsed < 0 {
				elapsed = 0
			}
			t := min(float32(elapsed)/float32(suggEnterDuration), 1)
			m.alpha = components.Smoothstep(t)
			m.dx = -suggSlideX * (1 - components.GlideEase(t))
		}
		m.last = el.snapshotSuggestion(pl)
	} else {
		m.active = nil
		m.alpha = 0
		m.dx = 0
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
			m.leaving.dx = m.leaving.fromDX - suggExitSlideX*components.GlideEase(t)
			if t >= 1 || m.leaving.alpha <= 0.001 {
				m.leaving = nil
			}
		}
	}
	return (cur != nil && animate && (m.alpha < 1 || m.dx != 0)) || m.leaving != nil
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
					groups:  slicesClone(sg.groups),
					shaped:  sg.shaped,
					textX:   u.x + s.x + r0.X,
					textY:   u.y + s.y + line.Baseline - gb,
					lineY:   u.y + s.y + line.Rect.Y,
					lineH:   line.Rect.H,
					clipW:   max(s.w-r0.X, 0) + 24,
					tabX:    u.x + s.x + r1.X + 4,
					tabMidY: u.y + s.y + r1.Y + r1.H/2,
				}
			}
		}
	}
	return suggSnap{}
}

func slicesClone[T any](s []T) []T {
	if len(s) == 0 {
		return nil
	}
	out := make([]T, len(s))
	copy(out, s)
	return out
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

func (ed *Editor) cancelPredict() {
	ed.mu.Lock()
	ed.predictSeq++
	if ed.predictTimer != nil {
		ed.predictTimer.Stop()
		ed.predictTimer = nil
	}
	ed.pendingSugg = nil
	ed.mu.Unlock()
}

func (ed *Editor) schedulePredict(ask profile.PredictAsk, version uint64) {
	predict := ed.Predict
	if predict == nil || ask.Text == "" {
		return
	}
	delay := ed.predictDelay
	if delay <= 0 {
		delay = defaultPredictDelay
	}
	ed.mu.Lock()
	ed.predictSeq++
	seq := ed.predictSeq
	if ed.predictTimer != nil {
		ed.predictTimer.Stop()
		ed.predictTimer = nil
	}
	ed.pendingSugg = nil
	ed.predictTimer = time.AfterFunc(delay, func() {
		ed.mu.Lock()
		stale := ed.predictSeq != seq
		ed.mu.Unlock()
		if stale {
			return
		}
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
	})
	ed.mu.Unlock()
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
