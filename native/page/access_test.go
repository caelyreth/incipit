package page

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/egoist/mygo/ui"
)

// Assistive technology sees the page's units with their semantic roles,
// names and values, while the text field remains the editing surface.
func TestAccessibilityExposesPageUnits(t *testing.T) {
	el := newElement(t)
	typeText(el, "## Title")
	el.Editor.Do(vocab.Break{}, time.Unix(100, 0))
	typeText(el, "Body 😀")
	tester := ui.NewTester(func(c *ui.Context) { el.View(c) }, 900, 600)
	tester.Frame()

	field, ok := tester.AccessByRole(ui.RoleTextField, "Page")
	if !ok || field.Value != "Body 😀" {
		t.Fatalf("page field = %+v, %v", field, ok)
	}
	heading, ok := tester.AccessByRole(ui.RoleHeading, "Title")
	if !ok || heading.Level != 2 || heading.Value != "Title" {
		t.Fatalf("heading = %+v, %v", heading, ok)
	}
	body, ok := tester.AccessByRole(ui.RoleText, "Body 😀")
	if !ok || body.Value != "Body 😀" {
		t.Fatalf("body = %+v, %v", body, ok)
	}
}

// A feedback pulse sweeps forward from the starts of its lines or
// backward from their ends; reduced motion shows it whole.
