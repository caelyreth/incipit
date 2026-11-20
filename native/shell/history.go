package shell

// canGo reports whether the place history goes by step: -1 back, 1
// forward.
func (s *State) canGo(step int) bool { return false }

// goPlace goes along the place history by step.
func (s *State) goPlace(step int) {}
