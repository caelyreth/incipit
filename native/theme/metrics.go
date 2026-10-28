package theme

type Layout struct {
	Sidebar           float32
	Titlebar          float32
	Status            float32
	Narrow            float32
	ReadingTop        float32
	ReadingBottom     float32
	TitleBottom       float32
	HeaderInset       float32
	ContentGutter     float32
	TrafficLightInset float32
}

var DefaultLayout = Layout{
	Sidebar:           256,
	Titlebar:          48,
	Status:            40,
	Narrow:            920,
	ReadingTop:        84,
	ReadingBottom:     150,
	TitleBottom:       24,
	HeaderInset:       12,
	ContentGutter:     16,
	TrafficLightInset: 96,
}

type ControlMetrics struct {
	Height       float32
	InlineHeight float32
	Icon         float32
	Radius       float32
	Row          float32
	PanelRadius  float32
	DockRadius   float32
}

var DefaultControls = ControlMetrics{
	Height:       32,
	InlineHeight: 24,
	Icon:         18,
	Radius:       8,
	Row:          36,
	PanelRadius:  11,
	DockRadius:   13,
}

// Scaled converts a reference-at-16px chrome measure to the app's UI size.
func (m ControlMetrics) Scaled(uiFontSize float32) ControlMetrics {
	scale := uiFontSize / DefaultUIFontSize
	return ControlMetrics{
		Height:       m.Height * scale,
		InlineHeight: m.InlineHeight * scale,
		Icon:         m.Icon * scale,
		Radius:       m.Radius * scale,
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
