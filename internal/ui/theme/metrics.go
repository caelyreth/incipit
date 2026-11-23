package theme

// Layout is the window's reference geometry at the 16 DIP UI size.
// ChromeGutter is the gap between the window edge and the first visible
// chrome: the macOS traffic lights and column controls.
type Layout struct {
	Titlebar      float32
	Status        float32
	ReadingTop    float32
	ChromeGutter  float32
	ContentGutter float32
	DockOffset    float32
}

var DefaultLayout = Layout{
	Titlebar:      40,
	Status:        32,
	ReadingTop:    64,
	ChromeGutter:  12,
	ContentGutter: 24,
	DockOffset:    20,
}

// DockClearance is how far the floating writing dock reaches up from the
// bottom of the document column, plus a reading gap above it, at the
// 16 DIP reference size: the room the last line needs to clear the dock.
func (l Layout) DockClearance() float32 {
	dock := DefaultControls.Height + 2*float32(Space04) + 2
	return l.DockOffset + dock + float32(Space16)
}

// ControlMetrics are the chrome's sizes by the job they do.
type ControlMetrics struct {
	Height       float32
	InlineHeight float32
	// Icon is an icon alone on a control; RowIcon one at the head of a
	// row, before its words; InlineIcon one beside small words, as in a
	// chip or a button's label.
	Icon       float32
	RowIcon    float32
	InlineIcon float32
	Radius     float32
	// InnerRadius rounds what stands inside a control or a panel: a
	// segment's thumb, a row of a list, a tile, a token in a field.
	InnerRadius float32
	// Badge is a pill badge's height, rounded to its middle.
	Badge       float32
	Row         float32
	PanelRadius float32
	DockRadius  float32
}

var DefaultControls = ControlMetrics{
	Height:       32,
	InlineHeight: 24,
	Icon:         18,
	RowIcon:      16,
	InlineIcon:   14,
	Radius:       8,
	InnerRadius:  6,
	Badge:        18,
	Row:          36,
	PanelRadius:  11,
	DockRadius:   12,
}

// Scaled converts a reference-at-16px chrome measure to the app's UI size.
func (m ControlMetrics) Scaled(uiFontSize float32) ControlMetrics {
	scale := uiFontSize / DefaultUIFontSize
	return ControlMetrics{
		Height:       m.Height * scale,
		InlineHeight: m.InlineHeight * scale,
		Icon:         m.Icon * scale,
		RowIcon:      m.RowIcon * scale,
		InlineIcon:   m.InlineIcon * scale,
		Radius:       m.Radius * scale,
		InnerRadius:  m.InnerRadius * scale,
		Badge:        m.Badge * scale,
		Row:          m.Row * scale,
		PanelRadius:  m.PanelRadius * scale,
		DockRadius:   m.DockRadius * scale,
	}
}

type Spacing float32

const (
	Space02 Spacing = 2
	Space04 Spacing = 4
	Space06 Spacing = 6
	Space08 Spacing = 8
	Space12 Spacing = 12
	Space16 Spacing = 16
	Space20 Spacing = 20
	Space24 Spacing = 24
	Space32 Spacing = 32
)

// DIPs scales the chrome spacing by the same UI font reference as controls.
func (s Spacing) DIPs(uiFontSize float32) float32 {
	return float32(s) * uiFontSize / DefaultUIFontSize
}

type TextSize uint8

const (
	TextLarge TextSize = iota + 1
	TextDefault
	TextSmall
	TextXSmall
)

// A row's words, in every list, picker, switcher and window that is not
// the field: its name, a line saying more of it, and what is counted or
// measured beside it. A settings row is the measure, its name at the
// interface's size and what follows a step down; the field's cards keep
// their own denser scale.
const (
	RowTitle = TextDefault
	RowNote  = TextSmall
	RowMeta  = TextSmall
)

func (s TextSize) Size(uiFontSize float32) float32 {
	reference := float32(14)
	switch s {
	case TextLarge:
		reference = 16
	case TextSmall:
		reference = 12
	case TextXSmall:
		reference = 11
	}
	return reference * uiFontSize / DefaultUIFontSize
}

func (s TextSize) LineHeight(uiFontSize float32) float32 {
	reference := float32(20)
	switch s {
	case TextLarge:
		reference = 24
	case TextSmall, TextXSmall:
		reference = 16
	}
	return reference * uiFontSize / DefaultUIFontSize
}

// HeadlineSize is the chrome-page heading scale. Document titles use
// ProseStyle instead.
type HeadlineSize uint8

const (
	HeadlineSmall HeadlineSize = iota + 1
	HeadlineMedium
	HeadlineXLarge
)

func (s HeadlineSize) Size(uiFontSize float32) float32 {
	scale := float32(1)
	switch s {
	case HeadlineMedium:
		scale = 1.125
	case HeadlineXLarge:
		scale = 1.43
	}
	return uiFontSize * scale
}

// HeadlineLineHeight is the source-defined relative leading for chrome
// headings.
const HeadlineLineHeight = float32(1.4)
