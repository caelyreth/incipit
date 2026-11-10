package motion

import "time"

// FadeDuration is how long a fade takes from hidden to shown.
const FadeDuration = 160 * time.Millisecond

// Fade eases what shows and hides in and out: a cubic ease-out toward
// shown or hidden, turned around from wherever it is when it changes.
// What first steps shown fades in.
type Fade struct {
	value, from, target float32
	start               time.Time
}

// Step moves the fade toward visible or not at now and returns its
// opacity; without animation it is there at once.
func (f *Fade) Step(visible bool, now time.Time, animate bool) float32 {
	target := float32(0)
	if visible {
		target = 1
	}
	if target != f.target {
		f.from, f.target, f.start = f.value, target, now
	}
	if !animate {
		f.value = target
		return f.value
	}
	progress := min(float32(now.Sub(f.start))/float32(FadeDuration), 1)
	rest := 1 - progress
	f.value = f.from + (f.target-f.from)*(1-rest*rest*rest)
	return f.value
}

// Moving reports a fade not yet at its target.
func (f *Fade) Moving() bool { return f.value != f.target }

// Gone reports a fade that has finished hiding.
func (f *Fade) Gone() bool { return f.target == 0 && f.value == 0 }
