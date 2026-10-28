package motion

type RevealOptions struct {
	ViewportTop    float32
	ViewportHeight float32
	CaretY         float32
	LineHeight     float32
	CurrentOffset  float32
	Typewriter     bool
	ManualOverride bool
}

// RevealOffset computes the viewport target that keeps the caret visible.
// It returns changed=false when the caret already satisfies the reveal rule.
func RevealOffset(options RevealOptions) (float32, bool) {
	margin := float32(8)
	var delta float32
	switch {
	case options.Typewriter && !options.ManualOverride:
		delta = options.ViewportTop + options.ViewportHeight*0.48 - options.CaretY
	case options.CaretY < options.ViewportTop+margin:
		delta = options.ViewportTop + margin - options.CaretY
	case options.CaretY+options.LineHeight > options.ViewportTop+options.ViewportHeight-margin:
		delta = options.ViewportTop + options.ViewportHeight - margin -
			options.LineHeight - options.CaretY
	}
	if delta == 0 {
		return options.CurrentOffset, false
	}
	return options.CurrentOffset + delta, true
}
