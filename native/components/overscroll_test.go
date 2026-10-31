package components

import "testing"

func TestOverscrollStylesScaleBySurface(t *testing.T) {
	if got := OverscrollEditor.Limit(800, 16); got != 80 {
		t.Fatalf("editor limit = %v", got)
	}
	if got := OverscrollList.Limit(800, 16); got != 44 {
		t.Fatalf("list limit = %v", got)
	}
	if got := OverscrollCompact.Limit(30, 16); got != 12 {
		t.Fatalf("compact limit = %v", got)
	}
	if OverscrollList.Stiffness <= OverscrollEditor.Stiffness {
		t.Fatal("the shorter list pull does not return more firmly")
	}
	if OverscrollCompact.Resistance >= OverscrollList.Resistance {
		t.Fatal("the compact pull is not more resistant")
	}
}
