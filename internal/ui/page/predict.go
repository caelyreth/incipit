package page

import (
	"time"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

const defaultPredictDelay = 280 * time.Millisecond

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
