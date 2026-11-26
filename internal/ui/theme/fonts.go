package theme

import _ "embed"

//go:generate go tool fetch

//go:embed fonts/NewCMMath-Book.otf
var mathFont []byte

// MathFont is the face formulas are set in: New Computer Modern Math, an
// OpenType MATH font the formula renderer reads as bytes, never through the
// platform's text engine.
func MathFont() []byte { return mathFont }
