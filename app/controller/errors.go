package controller

import "errors"

var (
	// ErrPolicyDeclined reports that the active policies did not handle an
	// input mutation.
	ErrPolicyDeclined = errors.New("controller: active policies declined the edit")

	// ErrNoComposition reports a preedit operation without preedit state.
	ErrNoComposition = errors.New("controller: no active composition")

	// ErrMarkedRange reports a native range outside the active marked text.
	ErrMarkedRange = errors.New("controller: range is outside the marked text")

	// ErrInvalidUTF16Range reports a reversed, out-of-bounds, or
	// surrogate-splitting document range.
	ErrInvalidUTF16Range = errors.New("controller: invalid UTF-16 range")

	// ErrNoGeometry reports that no layout adapter can answer a native query.
	ErrNoGeometry = errors.New("controller: text geometry is unavailable")

	// ErrUnknownCommand reports a command outside the editor policy catalog.
	ErrUnknownCommand = errors.New("controller: unknown editor command")
)
