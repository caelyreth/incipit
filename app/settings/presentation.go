package settings

// PresentationState contains Neverland's app-wide writing modes.
type PresentationState struct {
	Motion     bool
	Typewriter bool
	Texture    bool
}

// DefaultPresentation matches Neverland: motion and texture on, typewriter off.
func DefaultPresentation() PresentationState {
	return PresentationState{Motion: true, Texture: true}
}

func (s PresentationState) Get(mode Presentation) bool {
	switch mode {
	case PresentationMotion:
		return s.Motion
	case PresentationTypewriter:
		return s.Typewriter
	case PresentationTexture:
		return s.Texture
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
	case PresentationTexture:
		s.Texture = value
	default:
		return false
	}
	return true
}
