// Package showcase replays a policy's showcase (policy.Showcase, declared
// with the policy) against a real engine, so a policy is shown by the
// engine itself rather than by a recording: typing with a human cadence,
// intents and pauses. The player owns the page and the timeline; a host types through
// a page of its own over the player's engine, which it draws. Nothing here
// depends on MyGo.
package settings

import (
	"maps"
	"math/rand/v2"
	"time"
	"unicode/utf8"

	"github.com/caelyreth/incipit/internal/app/manifest"
	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/engine"
	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

const (
	// showcaseFadeOut and showcaseFadeIn frame each loop, so a restart reads as a cut
	// rather than text vanishing.
	showcaseFadeOut = 420 * time.Millisecond
	showcaseFadeIn  = 360 * time.Millisecond
	// gap is how long a player may go unadvanced, off screen or in a
	// background window, before it resumes where it left off instead of
	// rushing through the beats it missed.
	gap = time.Second
)

// Page is what the player types through: the host's page over the
// player's engine, so the host sees every input as its own.
type ShowcasePage interface {
	Do(in vocab.Intent, at time.Time) bool
}

// View is the view of the player's engine the host's page opens.
const ShowcaseView uint64 = 1

// Player replays one script, looping, against a page of its own.
type ShowcasePlayer struct {
	script policy.Showcase
	policy string
	values policy.Values
	open   func(*engine.Engine) ShowcasePage

	prog    *policy.Program
	engine  *engine.Engine
	page    ShowcasePage
	loop    int
	index   int
	typed   int
	due     time.Time
	started time.Time
	ended   time.Time
	wake    time.Time
	err     error
}

// NewPlayer prepares the showcase of policyID under the person's values,
// starting at now. open gives the host's page over each loop's engine.
func NewShowcasePlayer(script policy.Showcase, policyID string, values policy.Values, open func(*engine.Engine) ShowcasePage, now time.Time) *ShowcasePlayer {
	p := &ShowcasePlayer{script: script, policy: policyID, open: open}
	p.configure(values)
	p.restart(now)
	return p
}

// Engine is the current loop's engine.
func (p *ShowcasePlayer) Engine() *engine.Engine { return p.engine }

// Program is the program the story plays under.
func (p *ShowcasePlayer) Program() *policy.Program { return p.prog }

// Loop counts restarts, so a host can drop motion that belongs to the
// previous loop's page.
func (p *ShowcasePlayer) Loop() int { return p.loop }

// Err is the first failure of the current loop.
func (p *ShowcasePlayer) Err() error { return p.err }

// Configure replays the script under values when they differ, so changing
// an option shows its effect from the start of the story.
func (p *ShowcasePlayer) Configure(values policy.Values, now time.Time) {
	if maps.Equal(values, p.values) {
		return
	}
	p.configure(values)
	p.restart(now)
}

// configure compiles values with the showcased policy on: a showcase
// always shows its feature working, under the options the person chose.
func (p *ShowcasePlayer) configure(values policy.Values) {
	p.values = maps.Clone(values)
	on := maps.Clone(values)
	if on == nil {
		on = policy.Values{}
	}
	on[policy.EnabledKey(p.policy)] = true
	prog, err := manifest.Compile(on, false)
	if err != nil {
		p.err = err
		return
	}
	p.prog = prog
}

// Opacity is the page's fade at now: rising after a restart, falling
// before the next.
func (p *ShowcasePlayer) Opacity(now time.Time) float32 {
	opacity := float32(1)
	if since := now.Sub(p.started); since < showcaseFadeIn {
		opacity = min(opacity, float32(since)/float32(showcaseFadeIn))
	}
	if !p.ended.IsZero() {
		restart := p.ended.Add(p.script.Hold)
		if left := restart.Sub(now); left < showcaseFadeOut {
			opacity = min(opacity, float32(max(0, left))/float32(showcaseFadeOut))
		}
	}
	return max(0, opacity)
}

// Advance plays every beat due by now and returns when the next one is.
func (p *ShowcasePlayer) Advance(now time.Time) time.Time {
	if !p.wake.IsZero() && now.Sub(p.wake) > gap {
		// Nobody advanced the player when it asked to be: it was off
		// screen or in a background window. Resume where it left off
		// rather than replay what passed while nobody watched.
		shift := now.Sub(p.wake)
		p.due = p.due.Add(shift)
		p.started = p.started.Add(shift)
		if !p.ended.IsZero() {
			p.ended = p.ended.Add(shift)
		}
	}
	p.wake = p.next(now)
	return p.wake
}

func (p *ShowcasePlayer) next(now time.Time) time.Time {
	for {
		if !p.ended.IsZero() {
			restart := p.ended.Add(p.script.Hold)
			if fade := restart.Add(-showcaseFadeOut); now.Before(fade) {
				// Wake for the fade out, not only for the restart.
				return fade
			}
			if now.Before(restart) {
				return restart
			}
			p.restart(restart)
			continue
		}
		if now.Before(p.due) {
			return p.due
		}
		p.step()
	}
}

// Finish plays the rest of the current loop at once and holds its last
// page, for a stage that must not move.
func (p *ShowcasePlayer) Finish() {
	for p.ended.IsZero() {
		p.step()
	}
}

func (p *ShowcasePlayer) restart(now time.Time) {
	p.loop++
	p.index, p.typed = 0, 0
	p.started, p.due, p.ended = now, now.Add(showcaseFadeIn), time.Time{}
	if p.prog == nil {
		p.ended = now
		return
	}
	snap, err := profile.EmptyPage(1, rand.Uint64(), doc.NewInterner())
	if err != nil {
		p.err, p.ended = err, now
		return
	}
	p.err = nil
	p.engine = engine.New(snap, p.prog, engine.Options{})
	p.page = p.open(p.engine)
}

// step plays the current beat, or the next character of a typing beat,
// and schedules the one after it.
func (p *ShowcasePlayer) step() {
	if p.index >= len(p.script.Beats) {
		p.ended = p.due
		return
	}
	beat := p.script.Beats[p.index]
	if beat.Kind == policy.BeatType {
		rest := beat.Text[p.typed:]
		if rest == "" {
			p.index, p.typed = p.index+1, 0
			return
		}
		r, size := utf8.DecodeRuneInString(rest)
		p.do(typed(r), p.due)
		p.typed += size
		p.due = p.due.Add(keystroke(r, p.loop*7919+p.index*131+p.typed))
		return
	}
	if beat.Kind == policy.BeatPress {
		p.do(beat.Intent, p.due)
	}
	p.index++
	p.due = p.due.Add(beatPause(beat))
}

func (p *ShowcasePlayer) do(in vocab.Intent, at time.Time) {
	if !p.page.Do(in, at) && p.err == nil {
		p.err = &ShowcaseRejectedError{Intent: in}
	}
}

func intents(beat policy.Beat) []vocab.Intent {
	switch beat.Kind {
	case policy.BeatType:
		var out []vocab.Intent
		for _, r := range beat.Text {
			out = append(out, typed(r))
		}
		return out
	case policy.BeatPress:
		return []vocab.Intent{beat.Intent}
	}
	return nil
}

func typed(r rune) vocab.Intent {
	if r == '\n' {
		return vocab.Break{}
	}
	return vocab.Type{Text: string(r)}
}

// keystroke is the pause after typing r: a steady hand with a little
// deterministic unevenness, a breath after spaces and a longer one after
// the end of a clause.
func keystroke(r rune, seed int) time.Duration {
	jitter := time.Duration(int(uint32(seed)*2654435761%60)-25) * time.Millisecond
	pause := 82*time.Millisecond + jitter
	switch r {
	case ' ':
		pause += 34 * time.Millisecond
	case ',', ';', ':':
		pause += 160 * time.Millisecond
	case '.', '!', '?', '。', '，', '…', '\n':
		pause += 260 * time.Millisecond
	}
	return pause
}

func beatPause(beat policy.Beat) time.Duration {
	switch beat.Kind {
	case policy.BeatWait:
		return beat.Wait
	case policy.BeatPress:
		return 260 * time.Millisecond
	default:
		return 380 * time.Millisecond
	}
}

// RejectedError reports an intent of a script the engine did not answer:
// the script and the policies disagree.
type ShowcaseRejectedError struct{ Intent vocab.Intent }

func (e *ShowcaseRejectedError) Error() string { return "showcase: the engine rejected a beat" }
