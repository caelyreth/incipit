package workspace

import "testing"

func TestPagesKeepIndependentMountedState(t *testing.T) {
	t.Parallel()
	initial := "first"
	workspace, err := New(&initial, func(template *string) (*string, error) {
		value := ""
		return &value, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	mounted := &initial
	second, err := workspace.NewPage(mounted)
	if err != nil {
		t.Fatal(err)
	}
	if err := workspace.Open(second, &mounted, ScrollPosition{}); err != nil {
		t.Fatal(err)
	}
	*mounted = "second"
	if err := workspace.Close(second, &mounted, ScrollPosition{Y: 40}); err != nil {
		t.Fatal(err)
	}
	if *mounted != "first" {
		t.Fatalf("mounted = %q", *mounted)
	}
	if err := workspace.Open(second, &mounted, ScrollPosition{}); err != nil {
		t.Fatal(err)
	}
	if *mounted != "second" {
		t.Fatalf("reopened = %q", *mounted)
	}
	scroll, ok := workspace.ActiveScroll()
	if !ok || scroll.Y != 40 {
		t.Fatalf("scroll = %#v,%v", scroll, ok)
	}
}

func TestAdjacentTabsCycleAndClosingLastKeepsPages(t *testing.T) {
	t.Parallel()
	initial := "a"
	workspace, err := New(&initial, func(template *string) (*string, error) {
		value := "b"
		return &value, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	mounted := &initial
	second, _ := workspace.NewPage(mounted)
	if err := workspace.Open(second, &mounted, ScrollPosition{}); err != nil {
		t.Fatal(err)
	}
	next, ok := workspace.Adjacent(true)
	if !ok || next != 0 {
		t.Fatalf("cycle = %d,%v", next, ok)
	}
	if err := workspace.Close(second, &mounted, ScrollPosition{}); err != nil {
		t.Fatal(err)
	}
	if err := workspace.Close(0, &mounted, ScrollPosition{}); err != nil {
		t.Fatal(err)
	}
	if _, ok := workspace.Active(); ok {
		t.Fatal("last tab stayed active")
	}
	if len(workspace.Pages()) != 2 {
		t.Fatalf("pages = %d", len(workspace.Pages()))
	}
}
