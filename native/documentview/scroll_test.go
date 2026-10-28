package documentview

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/native/theme"
)

func TestScrollYRestoresWithoutCarryoverMotion(t *testing.T) {
	element := NewElement(nil, theme.DefaultSettings().Semantic(), theme.NewProseStyle(19), time.Unix(0, 0))
	element.SetScrollY(120)
	if element.ScrollY() != 120 {
		t.Fatalf("scroll = %v", element.ScrollY())
	}
	element.SetScrollY(-20)
	if element.ScrollY() != 0 {
		t.Fatalf("negative scroll = %v", element.ScrollY())
	}
}
