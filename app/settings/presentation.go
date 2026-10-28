package settings

type PresentationState struct {
	Motion     bool
	Typewriter bool
	Zen        bool
}

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

func (s *PresentationState) Set(mode Presentation, value bool) {
	switch mode {
	case PresentationMotion:
		s.Motion = value
	case PresentationTypewriter:
		s.Typewriter = value
	case PresentationZen:
		s.Zen = value
	}
}
