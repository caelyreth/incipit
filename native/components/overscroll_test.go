package components

import "testing"

func TestOverscrollStylesScaleBySurface(t *testing.T) {
	if got := OverscrollEditor.Limit(800, 16); got != 80 {
		t.Fatalf("editor limit = %v", got)
	}
	if got := OverscrollList.Limit(800, 16); got != 52 {
		t.Fatalf("list limit = %v", got)
	}
	if got := OverscrollCompact.Limit(30, 16); got != 12 {
		t.Fatalf("compact limit = %v", got)
	}
	styles := []OverscrollStyle{OverscrollEditor, OverscrollPanel, OverscrollList, OverscrollCompact}
	for index := 1; index < len(styles); index++ {
		larger, smaller := styles[index-1], styles[index]
		if smaller.Response >= larger.Response {
			t.Fatalf("style %d does not return quicker than the one above it", index)
		}
		if smaller.Resistance >= larger.Resistance {
			t.Fatalf("style %d is not firmer than the one above it", index)
		}
	}
}
