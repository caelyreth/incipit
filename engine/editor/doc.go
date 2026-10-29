// Package editor owns concrete editing policies, composition, history,
// navigation, refinement, and session orchestration.
//
// Segmentation adapters use Unicode UAX #29 rules with an ICU-tailored word
// scanner and Chinese/Japanese dictionary across Latin and CJK text. The
// dependencies are deliberately confined to this package's boundary helpers;
// editor plans and commands use model.ByteRange and model.UTF16Range instead.
package editor
