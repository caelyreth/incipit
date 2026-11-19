package theme

import (
	"math"
	"reflect"
	"sync"
	"time"

	"github.com/caelyreth/neverland/native/motion"
	"github.com/egoist/mygo/ui"
)

// ThemeTransition is about how long a theme or accent change takes to
// settle from rest.
const ThemeTransition = 360 * time.Millisecond

// colorFrequency settles a channel's spring in about ThemeTransition.
const colorFrequency = float32(18)

// Transition carries the app's colors from one theme to the next. Every
// window reads the same one, so a change crosses the whole app together
// instead of each window snapping when it next happens to rebuild.
//
// Each color channel follows its target on a critically damped spring, so
// a change made while another is under way takes over at the speed the
// colors are already moving: flipping the theme back and forth eases
// through rather than jerking at each reversal.
type Transition struct {
	mu           sync.Mutex
	target       Theme
	targetValues []float32
	channels     []motion.Spring
	set          bool
	settled      bool
}

// Retarget heads every color for target from wherever it is at now.
func (t *Transition) Retarget(target Theme, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.set {
		t.place(target, now)
		return
	}
	// Bring every channel to now under the old target, then aim it at the
	// new one from there, keeping its velocity.
	t.step(now)
	t.target = target
	t.targetValues = channelsOf(target)
	t.settled = !t.step(now)
}

// Jump shows target at once, for an app that is not animating.
func (t *Transition) Jump(target Theme) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.place(target, time.Time{})
}

// At is the theme showing at now for settings, and whether it is still
// changing. A transition never told of settings yields them as they are.
func (t *Transition) At(settings Settings, now time.Time) (Theme, bool) {
	if t == nil {
		return settings.Semantic(), false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.set {
		t.place(settings.Semantic(), now)
	}
	if t.settled {
		return t.target, false
	}
	moving := t.step(now)
	if !moving {
		t.settled = true
		return t.target, false
	}
	result := t.target
	colors := colorsOf(&result)
	for index, color := range colors {
		*color = ui.RGBA(
			channel(t.channels[4*index]),
			channel(t.channels[4*index+1]),
			channel(t.channels[4*index+2]),
			float32(channel(t.channels[4*index+3]))/255,
		)
	}
	return result, true
}

func (t *Transition) place(target Theme, now time.Time) {
	t.target, t.set, t.settled = target, true, true
	values := channelsOf(target)
	t.targetValues = values
	if cap(t.channels) < len(values) {
		t.channels = make([]motion.Spring, len(values))
	} else {
		t.channels = t.channels[:len(values)]
	}
	for index, value := range values {
		t.channels[index] = motion.NewSpring(value, now)
	}
}

// step advances every channel toward the target to now and reports whether
// any is still moving.
func (t *Transition) step(now time.Time) bool {
	moving := false
	for index, value := range t.targetValues {
		t.channels[index].Step(value, now, colorFrequency)
		moving = moving || t.channels[index].Moving()
	}
	return moving
}

func channel(spring motion.Spring) uint8 {
	return uint8(max(0, min(255, math.Round(float64(spring.Value())))))
}

// channelsOf lists theme's colors as RGBA channels in field order.
func channelsOf(theme Theme) []float32 {
	colors := colorsOf(&theme)
	values := make([]float32, 0, 4*len(colors))
	for _, color := range colors {
		values = append(values, float32(color.R), float32(color.G), float32(color.B), float32(color.A))
	}
	return values
}

var colorType = reflect.TypeFor[ui.Color]()

// colorsOf points at every color in theme, in field order.
func colorsOf(theme *Theme) []*ui.Color {
	colors := make([]*ui.Color, 0, 48)
	var walk func(reflect.Value)
	walk = func(value reflect.Value) {
		switch {
		case value.Type() == colorType:
			colors = append(colors, value.Addr().Interface().(*ui.Color))
		case value.Kind() == reflect.Struct:
			for index := range value.NumField() {
				walk(value.Field(index))
			}
		case value.Kind() == reflect.Array:
			for index := range value.Len() {
				walk(value.Index(index))
			}
		}
	}
	walk(reflect.ValueOf(theme).Elem())
	return colors
}
