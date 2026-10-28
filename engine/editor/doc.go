// Package editor owns concrete editing policies, composition, history,
// navigation, refinement, and session orchestration.
//
// Segmentation adapters use UAX #29 through github.com/clipperhouse/uax29/v2.
// The dependency is deliberately confined to this package's boundary helpers;
// editor plans and commands use model.ByteRange and model.UTF16Range instead.
package editor
