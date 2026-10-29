// Package showcase replays scripted writing against a real editor session,
// so a feature is shown by the engine itself rather than by a recording.
// A script is a list of beats: typing with a human cadence, commands,
// pointer selections and pauses. The player owns the document and the
// timeline; a native stage draws the document. Nothing here depends on MyGo.
package showcase

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/engine/kernel"
)

// BeatKind is what one beat does.
type BeatKind uint8

const (
	BeatType BeatKind = iota + 1
	BeatPress
	BeatSelect
	BeatPlace
	BeatWait
)

// Beat is one step of a script.
type Beat struct {
	Kind    BeatKind
	Text    string
	Command kernel.CommandID
	Wait    time.Duration
}

// Type types text one character at a time, as a person would.
func Type(text string) Beat { return Beat{Kind: BeatType, Text: text} }

// Press invokes a command through the editor, as its shortcut would.
func Press(command kernel.CommandID) Beat { return Beat{Kind: BeatPress, Command: command} }

// Select selects the first occurrence of text in the drawn document, as a
// pointer drag would.
func Select(text string) Beat { return Beat{Kind: BeatSelect, Text: text} }

// Place clicks just after the first occurrence of text.
func Place(text string) Beat { return Beat{Kind: BeatPlace, Text: text} }

// Wait holds the frame, letting the reader see what just happened.
func Wait(duration time.Duration) Beat { return Beat{Kind: BeatWait, Wait: duration} }

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

// Player replays one script, looping, against its own document.
type Player struct {
	script        Script
	configuration kernel.PolicyConfiguration
	clock         *playerClock

	document *controller.DocumentController
	loop     int
	index    int
	typed    int
	due      time.Time
	started  time.Time
	ended    time.Time
	wake     time.Time
	err      error
}

type playerClock struct{ now time.Time }

func (c *playerClock) Now() time.Time { return c.now }

// NewPlayer prepares script under configuration, starting at now.
func NewPlayer(script Script, configuration kernel.PolicyConfiguration, now time.Time) *Player {
	player := &Player{
		script:        script,
		configuration: configuration.Clone(),
		clock:         &playerClock{now: now},
	}
	player.restart(now)
	return player
}

// Document is the live document the stage draws.
func (p *Player) Document() *controller.DocumentController { return p.document }

// Loop counts restarts, so a stage can drop motion that belongs to the
// previous loop's document.
func (p *Player) Loop() int { return p.loop }

// Err is the first failure of the current loop, if a beat was rejected.
func (p *Player) Err() error { return p.err }

// Configure replays the script under configuration when it differs, so
// toggling a setting shows its effect from the start of the story.
func (p *Player) Configure(configuration kernel.PolicyConfiguration, now time.Time) {
	if p.configuration.Equal(configuration) {
		return
	}
	p.configuration = configuration.Clone()
	p.restart(now)
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
		p.clock.now = p.due
		p.step()
	}
}

// Finish plays the rest of the current loop at once and holds its last
// page, for a stage that must not move.
func (p *Player) Finish() {
	if !p.ended.IsZero() {
		return
	}
	for p.index < len(p.script.Beats) {
		beat := p.script.Beats[p.index]
		if beat.Kind == BeatType {
			for _, r := range beat.Text[p.typed:] {
				p.document.ReplaceText(nil, string(r))
			}
		} else {
			p.play(beat)
		}
		p.index, p.typed = p.index+1, 0
	}
	p.ended = p.due
}

func (p *Player) restart(now time.Time) {
	p.clock.now = now
	p.loop++
	p.index, p.typed = 0, 0
	p.started, p.due, p.ended = now, now.Add(FadeIn), time.Time{}
	p.err = nil
	document, err := controller.NewTextDocument("", p.clock)
	if err == nil {
		err = document.Adopt(p.configuration)
	}
	if err != nil {
		p.err = err
		p.ended = now
		return
	}
	p.document = document
	for _, beat := range p.script.Setup {
		if beat.Kind == BeatType {
			for _, r := range beat.Text {
				p.document.ReplaceText(nil, string(r))
			}
			continue
		}
		p.play(beat)
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
		p.document.ReplaceText(nil, string(r))
		p.record()
		p.typed += size
		p.due = p.due.Add(keystroke(r, p.loop*7919+p.index*131+p.typed))
		return
	}
	p.play(beat)
	p.index++
	p.due = p.due.Add(beatPause(beat))
}

func (p *Player) play(beat Beat) {
	switch beat.Kind {
	case BeatPress:
		_, err := p.document.Execute(controller.Command{ID: string(beat.Command)})
		if err != nil && p.err == nil {
			p.err = err
		}
	case BeatSelect, BeatPlace:
		text := p.document.Text()
		start := strings.Index(text, beat.Text)
		if start < 0 {
			if p.err == nil {
				p.err = &MissingTextError{Text: beat.Text}
			}
			return
		}
		from := utf16Len(text[:start])
		to := from + utf16Len(beat.Text)
		if beat.Kind == BeatPlace {
			from = to
		}
		if err := p.document.SelectPointer(from, to, p.clock.now); err != nil && p.err == nil {
			p.err = err
		}
	}
	p.record()
}

func (p *Player) record() {
	if err := p.document.LastError(); err != nil && p.err == nil {
		p.err = err
	}
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
	case '.', '!', '?', '。', '，', '…':
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

func utf16Len(text string) int {
	length := 0
	for _, r := range text {
		if r >= 0x10000 {
			length += 2
		} else {
			length++
		}
	}
	return length
}

// MissingTextError reports a Select or Place beat naming text the page
// does not show: the script and the policies disagree.
type MissingTextError struct{ Text string }

func (e *MissingTextError) Error() string { return "showcase: page does not show " + e.Text }
