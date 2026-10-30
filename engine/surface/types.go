package surface

import (
	"time"

	"github.com/caelyreth/neverland/engine/model"
)

type ColorRole uint8

const (
	ColorText ColorRole = iota
	ColorMuted
	ColorHeading
	ColorAccent
	ColorSubtle
)

type TextStyle struct {
	Bold   bool
	Italic bool
	// Heading is a heading's level, 1 to 6, or 0 for body text.
	Heading uint8
	// Code is text set in the monospace face.
	Code   bool
	Strike bool
	Color  ColorRole
}

type StyleRun struct {
	Bytes model.ByteRange
	Style TextStyle
}

type ProducerID string

type DecorationID struct {
	Producer ProducerID
	Local    uint64
}

type Visibility uint8

const (
	VisibilityAlways Visibility = iota + 1
	VisibilityFocused
)

type Envelope uint8

const (
	EnvelopeFadeOut Envelope = iota + 1
	// EnvelopePulse lands and lets go: it rises to full in about a sixth,
	// easing into its peak, rests there a beat, then falls away and thins
	// into a faint tail, so a change is marked plainly and the text is
	// clear again soon after, as a sure hand's correction would be.
	EnvelopePulse
)

type OpacityKind uint8

const (
	OpacityFixed OpacityKind = iota + 1
	OpacityTimed
)

// Opacity is either steady or a timed pulse/fade.
//
// A timed opacity plays its envelope over Duration from phase From to phase
// To. A pulse runs 0 to 1; a pulse taken back runs from where it was to 0,
// the envelope backwards, so undoing what a pulse showed rewinds it rather
// than cutting it. From and To both zero mean 0 to 1.
type Opacity struct {
	Kind     OpacityKind
	Fixed    uint8
	Started  time.Time
	Duration time.Duration
	Peak     uint8
	Envelope Envelope
	From, To float32
}

func FixedOpacity(value uint8) Opacity {
	return Opacity{Kind: OpacityFixed, Fixed: value}
}

func TimedOpacity(started time.Time, duration time.Duration, peak uint8, envelope Envelope) Opacity {
	return Opacity{
		Kind:     OpacityTimed,
		Started:  started,
		Duration: duration,
		Peak:     peak,
		Envelope: envelope,
		From:     0,
		To:       1,
	}
}

// Played returns this opacity played from phase from to phase to over
// duration, starting at started.
func (o Opacity) Played(started time.Time, duration time.Duration, from, to float32) Opacity {
	o.Started, o.Duration, o.From, o.To = started, duration, from, to
	return o
}

// Phase is where in its envelope a timed opacity is at now, from 0 to 1,
// and whether it is still playing.
func (o Opacity) Phase(now time.Time) (float32, bool) {
	if o.Kind != OpacityTimed || o.Duration <= 0 || now.Before(o.Started) {
		return 0, false
	}
	t := float32(now.Sub(o.Started).Seconds() / o.Duration.Seconds())
	if t >= 1 {
		return o.span().to, false
	}
	span := o.span()
	return span.from + (span.to-span.from)*t, true
}

type phaseSpan struct{ from, to float32 }

func (o Opacity) span() phaseSpan {
	if o.From == 0 && o.To == 0 {
		return phaseSpan{0, 1}
	}
	return phaseSpan{o.From, o.To}
}

func (o Opacity) Sample(now time.Time, animate bool) float32 {
	switch o.Kind {
	case OpacityFixed:
		return float32(o.Fixed) / 255
	case OpacityTimed:
		phase, playing := o.Phase(now)
		if !playing {
			return 0
		}
		value := float32(1)
		if animate {
			value = o.Envelope.At(phase)
		}
		return value * float32(o.Peak) / 255
	default:
		return 0
	}
}

// At is the envelope's strength at phase, from 0 to 1.
func (e Envelope) At(phase float32) float32 {
	phase = max(0, min(1, phase))
	switch e {
	case EnvelopeFadeOut:
		return 1 - phase
	case EnvelopePulse:
		const rise, hold = 0.16, 0.22
		switch {
		case phase < rise:
			return 1 - cube(1-phase/rise)
		case phase < hold:
			return 1
		default:
			fall := 1 - (phase-hold)/(1-hold)
			return fall * fall
		}
	default:
		return 1
	}
}

func cube(value float32) float32 { return value * value * value }

func (o Opacity) Deadline() (time.Time, bool) {
	if o.Kind != OpacityTimed {
		return time.Time{}, false
	}
	return o.Started.Add(o.Duration), true
}

type PrimitiveKind uint8

const (
	PrimitiveFill PrimitiveKind = iota + 1
	PrimitiveRule
	PrimitiveGuide
)

type Primitive struct {
	Kind      PrimitiveKind
	Ranges    []model.UTF16Range
	Color     ColorRole
	Radius    float32
	Thickness float32
	At        model.UTF16Offset
	// Keys are a guide's keycaps, already in the platform's notation.
	Keys []string
}

func FillPrimitive(ranges []model.UTF16Range, color ColorRole, radius float32) Primitive {
	return Primitive{Kind: PrimitiveFill, Ranges: cloneRanges(ranges), Color: color, Radius: radius}
}

func RulePrimitive(ranges []model.UTF16Range, color ColorRole, thickness float32) Primitive {
	return Primitive{Kind: PrimitiveRule, Ranges: cloneRanges(ranges), Color: color, Thickness: thickness}
}

// GuidePrimitive anchors keycaps at a landing point: the keys that would
// take the caret there.
func GuidePrimitive(at model.UTF16Offset, keys []string, color ColorRole) Primitive {
	return Primitive{Kind: PrimitiveGuide, At: at, Keys: append([]string(nil), keys...), Color: color}
}

func (p Primitive) Clone() Primitive {
	p.Ranges = cloneRanges(p.Ranges)
	p.Keys = append([]string(nil), p.Keys...)
	return p
}

type Decoration struct {
	ID         DecorationID
	Primitive  Primitive
	Opacity    Opacity
	Visibility Visibility
	ExpiresAt  *time.Time
}

func (d Decoration) Sample(now time.Time, animate bool) float32 {
	if d.ExpiresAt != nil && !now.Before(*d.ExpiresAt) {
		return 0
	}
	return d.Opacity.Sample(now, animate)
}

func (d Decoration) Deadline() (time.Time, bool) {
	deadline, ok := d.Opacity.Deadline()
	if d.ExpiresAt != nil && (!ok || d.ExpiresAt.Before(deadline)) {
		deadline, ok = *d.ExpiresAt, true
	}
	return deadline, ok
}

func (d Decoration) Clone() Decoration {
	d.Primitive = d.Primitive.Clone()
	if d.ExpiresAt != nil {
		expires := *d.ExpiresAt
		d.ExpiresAt = &expires
	}
	return d
}

func cloneRanges(ranges []model.UTF16Range) []model.UTF16Range {
	return append([]model.UTF16Range(nil), ranges...)
}
