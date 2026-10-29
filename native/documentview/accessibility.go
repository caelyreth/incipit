package documentview

// MyGo v0.7.1 exposes two accessibility paths for a custom text surface:
//
//   - Element.HandleTextInput contributes the native text-input client, while
//     Role, Label, Value, and Focusable contribute the basic accessibility
//     node. AccessSelectText is delivered to HandleInput as InputSelect.
//   - Element.Access is the richer hook for selection ranges, TextRuns, and
//     editing actions, but its callback parameter is
//     *github.com/egoist/mygo/internal/platform.AccessNode. Go's internal
//     package rule prevents Neverland from naming that type.
//
// For a custom TextInputClient, MyGo also does not synthesize AccessNode
// selection or the AccessSetValue and AccessInsertText actions; those paths
// require its private built-in editor state. Neverland therefore supports
// labeled, focusable text with native IME/text services and screen-reader
// selection requests. Announced selection ranges, rich text runs, and
// accessibility-driven value replacement require a public description API in
// a future MyGo release.

// utf16OffsetForRune converts the rune offset used by MyGo accessibility input
// to the UTF-16 offset owned by the controller and native text services.
func utf16OffsetForRune(text string, offset int) int {
	if offset <= 0 {
		return 0
	}
	units := 0
	runes := 0
	for _, r := range text {
		if runes >= offset {
			return units
		}
		if r > 0xffff {
			units += 2
		} else {
			units++
		}
		runes++
	}
	return units
}
