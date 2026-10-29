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
	EnvelopePulse
)

type OpacityKind uint8

const (
	OpacityFixed OpacityKind = iota + 1
	OpacityTimed
)

// Opacity is either steady or a timed pulse/fade.
type Opacity struct {
	Kind     OpacityKind
	Fixed    uint8
	Started  time.Time
	Duration time.Duration
	Peak     uint8
	Envelope Envelope
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
	}
}

func (o Opacity) Sample(now time.Time, animate bool) float32 {
	switch o.Kind {
	case OpacityFixed:
		return float32(o.Fixed) / 255
	case OpacityTimed:
		if now.Before(o.Started) || o.Duration <= 0 {
			return 0
		}
		t := float32(now.Sub(o.Started).Seconds() / o.Duration.Seconds())
		if t > 1 {
			t = 1
		}
		if t >= 1 {
			return 0
		}
		value := float32(1)
		if animate {
			switch o.Envelope {
			case EnvelopeFadeOut:
				value = 1 - t
			case EnvelopePulse:
				enter := min(1, t*8)
				value = (1 - cube(1-enter)) * square(1-t)
			}
		}
		return value * float32(o.Peak) / 255
	default:
		return 0
	}
}

func (o Opacity) Deadline() (time.Time, bool) {
	if o.Kind != OpacityTimed {
		return time.Time{}, false
	}
	return o.Started.Add(o.Duration), true
}

func cube(value float32) float32   { return value * value * value }
func square(value float32) float32 { return value * value }

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
