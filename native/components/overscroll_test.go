package components

import "testing"

func TestOverscrollStylesScaleBySurface(t *testing.T) {
	if got := OverscrollEditor.Limit(800, 16); got < 279.9 || got > 280.1 {
		t.Fatalf("editor band = %v, want a third of its view", got)
	}
	if got := OverscrollList.Limit(800, 16); got != 200 {
		t.Fatalf("list band = %v", got)
	}
	if got := OverscrollCompact.Limit(30, 32); got != 80 {
		t.Fatalf("compact band = %v, want its least at twice the UI size", got)
	}
	styles := []OverscrollStyle{OverscrollEditor, OverscrollPanel, OverscrollList, OverscrollCompact}
	for index := 1; index < len(styles); index++ {
		larger, smaller := styles[index-1], styles[index]
		if smaller.Share >= larger.Share {
			t.Fatalf("style %d's band is not shorter than the one above it", index)
		}
		if smaller.Response >= larger.Response {
			t.Fatalf("style %d does not return quicker than the one above it", index)
		}
		if smaller.Resistance >= larger.Resistance {
			t.Fatalf("style %d is not firmer than the one above it", index)
		}
	}
}
