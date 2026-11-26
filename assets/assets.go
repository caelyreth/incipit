// Package assets holds the brand artwork the app draws itself: the
// profile of the application icon (profile.icon), as its vector.
package assets

import _ "embed"

// Profile is the icon's profile, an SVG of filled paths.
//
//go:embed profile.icon/Assets/profile.svg
var Profile []byte
