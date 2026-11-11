// Package showcase replays scripted writing against a real engine, so a
// policy is shown by the engine itself rather than by a recording. A
// script is a list of beats: typing with a human cadence, intents and
// pauses. The player owns the page and the timeline; a host types through
// a page of its own over the player's engine, which it draws. Nothing here
// depends on MyGo.
package showcase

import (
	"maps"
	"math/rand/v2"
	"time"
	"unicode/utf8"

	"github.com/caelyreth/neverland/app/manifest"
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/engine"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

// BeatKind is what one beat does.
type BeatKind uint8

const (
	BeatType BeatKind = iota + 1
	BeatPress
	BeatWait
)

// Beat is one step of a script.
type Beat struct {
	Kind   BeatKind
	Text   string
	Intent vocab.Intent
	Wait   time.Duration
}

// Type types text one character at a time, as a person would; a newline
// is Enter.
func Type(text string) Beat { return Beat{Kind: BeatType, Text: text} }

// Press sends an intent, as its key would.
func Press(in vocab.Intent) Beat { return Beat{Kind: BeatPress, Intent: in} }

// Wait holds the frame, letting the reader see what just happened.
func Wait(d time.Duration) Beat { return Beat{Kind: BeatWait, Wait: d} }

// Script is a showcase: beats set up instantly, then beats played in time.
// Hold is how long the finished page stays before the loop restarts.
type Script struct {
	Setup []Beat
	Beats []Beat
	Hold  time.Duration
}

const (
	// FadeOut and FadeIn frame each loop, so a restart reads as a cut
	// rather than text vanishing.
	FadeOut = 420 * time.Millisecond
	FadeIn  = 360 * time.Millisecond
	// gap is how long a player may go unadvanced, off screen or in a
	// background window, before it resumes where it left off instead of
	// rushing through the beats it missed.
	gap = time.Second
)

// Page is what the player types through: the host's page over the
// player's engine, so the host sees every input as its own.
type Page interface {
	Do(in vocab.Intent, at time.Time) bool
}

// View is the view of the player's engine the host's page opens.
const View uint64 = 1

// Player replays one script, looping, against a page of its own.
type Player struct {
	script Script
	policy string
	values policy.Values
	open   func(*engine.Engine) Page

	prog    *policy.Program
	engine  *engine.Engine
	page    Page
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
func NewPlayer(script Script, policyID string, values policy.Values, open func(*engine.Engine) Page, now time.Time) *Player {
	p := &Player{script: script, policy: policyID, open: open}
	p.configure(values)
	p.restart(now)
	return p
}

// Engine is the current loop's engine.
func (p *Player) Engine() *engine.Engine { return p.engine }

// Program is the program the story plays under.
func (p *Player) Program() *policy.Program { return p.prog }

// Loop counts restarts, so a host can drop motion that belongs to the
// previous loop's page.
func (p *Player) Loop() int { return p.loop }

// Err is the first failure of the current loop.
func (p *Player) Err() error { return p.err }

// Configure replays the script under values when they differ, so changing
// an option shows its effect from the start of the story.
func (p *Player) Configure(values policy.Values, now time.Time) {
	if maps.Equal(values, p.values) {
		return
	}
	p.configure(values)
	p.restart(now)
}

// configure compiles values with the showcased policy on: a showcase
// always shows its feature working, under the options the person chose.
func (p *Player) configure(values policy.Values) {
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
func (p *Player) Opacity(now time.Time) float32 {
	opacity := float32(1)
	if since := now.Sub(p.started); since < FadeIn {
		opacity = min(opacity, float32(since)/float32(FadeIn))
	}
	if !p.ended.IsZero() {
		restart := p.ended.Add(p.script.Hold)
		if left := restart.Sub(now); left < FadeOut {
			opacity = min(opacity, float32(max(0, left))/float32(FadeOut))
		}
	}
	return max(0, opacity)
}

// Advance plays every beat due by now and returns when the next one is.
func (p *Player) Advance(now time.Time) time.Time {
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

func (p *Player) next(now time.Time) time.Time {
	for {
		if !p.ended.IsZero() {
			restart := p.ended.Add(p.script.Hold)
			if fade := restart.Add(-FadeOut); now.Before(fade) {
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
func (p *Player) Finish() {
	for p.ended.IsZero() {
		p.step()
	}
}

func (p *Player) restart(now time.Time) {
	p.loop++
	p.index, p.typed = 0, 0
	p.started, p.due, p.ended = now, now.Add(FadeIn), time.Time{}
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
	for _, beat := range p.script.Setup {
		for _, in := range intents(beat) {
			p.do(in, now)
		}
	}
}

// step plays the current beat, or the next character of a typing beat,
// and schedules the one after it.
func (p *Player) step() {
	if p.index >= len(p.script.Beats) {
		p.ended = p.due
		return
	}
	beat := p.script.Beats[p.index]
	if beat.Kind == BeatType {
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
	if beat.Kind == BeatPress {
		p.do(beat.Intent, p.due)
	}
	p.index++
	p.due = p.due.Add(beatPause(beat))
}

func (p *Player) do(in vocab.Intent, at time.Time) {
	if !p.page.Do(in, at) && p.err == nil {
		p.err = &RejectedError{Intent: in}
	}
}

func intents(beat Beat) []vocab.Intent {
	switch beat.Kind {
	case BeatType:
		var out []vocab.Intent
		for _, r := range beat.Text {
			out = append(out, typed(r))
		}
		return out
	case BeatPress:
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

func beatPause(beat Beat) time.Duration {
	switch beat.Kind {
	case BeatWait:
		return beat.Wait
	case BeatPress:
		return 260 * time.Millisecond
	default:
		return 380 * time.Millisecond
	}
}

// RejectedError reports an intent of a script the engine did not answer:
// the script and the policies disagree.
type RejectedError struct{ Intent vocab.Intent }

func (e *RejectedError) Error() string { return "showcase: the engine rejected a beat" }
