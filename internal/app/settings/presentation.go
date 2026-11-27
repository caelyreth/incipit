package settings

// PresentationState contains Incipit's app-wide writing modes.
type PresentationState struct {
	Motion     bool
	Typewriter bool
}

// DefaultPresentation matches Incipit: motion on, typewriter off.
func DefaultPresentation() PresentationState {
	return PresentationState{Motion: true}
}

func (s PresentationState) Get(mode Presentation) bool {
	switch mode {
	case PresentationMotion:
		return s.Motion
	case PresentationTypewriter:
		return s.Typewriter
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
	default:
		return false
	}
	return true
}
