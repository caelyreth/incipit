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
// All positions and the returned offset use the same convention: content
// coordinates from the top and a non-negative scroll offset. It returns
// changed=false when the caret already satisfies the reveal rule.
func RevealOffset(options RevealOptions) (float32, bool) {
	margin := float32(8)
	top := options.CurrentOffset
	bottom := top + options.ViewportHeight
	var target float32
	changed := true
	switch {
	case options.Typewriter && !options.ManualOverride:
		// The caret's line sits at the exact vertical center.
		target = options.CaretY + options.LineHeight/2 - options.ViewportHeight/2
	case options.CaretY < top+margin:
		target = options.CaretY - margin
	case options.CaretY+options.LineHeight > bottom-margin:
		target = options.CaretY + options.LineHeight - options.ViewportHeight + margin
	default:
		target = options.CurrentOffset
		changed = false
	}
	if target == options.CurrentOffset {
		return options.CurrentOffset, false
	}
	return target, changed
}
