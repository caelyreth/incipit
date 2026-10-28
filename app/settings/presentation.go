package settings

// PresentationState contains Shoreward's three app-wide writing modes.
type PresentationState struct {
	Motion     bool
	Typewriter bool
	Zen        bool
}

// DefaultPresentation matches Shoreward: motion on, typewriter and Zen off.
func DefaultPresentation() PresentationState {
	return PresentationState{Motion: true}
}

func (s PresentationState) Get(mode Presentation) bool {
	switch mode {
	case PresentationMotion:
		return s.Motion
	case PresentationTypewriter:
		return s.Typewriter
	case PresentationZen:
		return s.Zen
	default:
		return false
	}
}

func (s *PresentationState) Set(mode Presentation, value bool) bool {
	if s == nil {
		return false
	}
	switch mode {
	case PresentationMotion:
		s.Motion = value
	case PresentationTypewriter:
		s.Typewriter = value
	case PresentationZen:
		s.Zen = value
	default:
		return false
	}
	return true
}
