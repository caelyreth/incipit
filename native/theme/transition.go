package theme

import (
	"reflect"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
)

// ThemeTransition is how long a theme or accent change takes to settle.
const ThemeTransition = 360 * time.Millisecond

// Transition carries the app's colors from one theme to the next. Every
// window reads the same one, so a change crosses the whole app together
// instead of each window snapping when it next happens to rebuild.
type Transition struct {
	mu      sync.Mutex
	from    Theme
	to      Theme
	started time.Time
	set     bool
}

// Retarget starts a move from the colors showing at now to target.
func (t *Transition) Retarget(target Theme, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.set {
		t.from, t.to, t.set = target, target, true
		return
	}
	t.from = t.at(now)
	t.to = target
	t.started = now
}

// Jump shows target at once, for an app that is not animating.
func (t *Transition) Jump(target Theme) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.from, t.to, t.set, t.started = target, target, true, time.Time{}
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
		t.from, t.to, t.set = settings.Semantic(), settings.Semantic(), true
	}
	return t.at(now), now.Sub(t.started) < ThemeTransition
}

func (t *Transition) at(now time.Time) Theme {
	progress := float32(now.Sub(t.started)) / float32(ThemeTransition)
	if progress >= 1 {
		return t.to
	}
	return Blend(t.from, t.to, smooth(max(0, progress)))
}

// Blend mixes every color of two themes; anything that is not a color
// comes from to.
func Blend(from, to Theme, amount float32) Theme {
	result := to
	blend(reflect.ValueOf(&result).Elem(), reflect.ValueOf(from), reflect.ValueOf(to), amount)
	return result
}

var colorType = reflect.TypeFor[ui.Color]()

func blend(out, from, to reflect.Value, amount float32) {
	switch {
	case out.Type() == colorType:
		mixed := from.Interface().(ui.Color).Mix(to.Interface().(ui.Color), amount)
		out.Set(reflect.ValueOf(mixed))
	case out.Kind() == reflect.Struct:
		for index := range out.NumField() {
			blend(out.Field(index), from.Field(index), to.Field(index), amount)
		}
	}
}

// smooth eases the change in and out, so neither end of it jolts.
func smooth(t float32) float32 {
	return t * t * (3 - 2*t)
}
