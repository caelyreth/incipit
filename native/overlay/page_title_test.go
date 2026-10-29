package overlay

import (
	"testing"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func TestPageTitleCommitsNormalizedIntent(t *testing.T) {
	var confirmedID, confirmedTitle, cancelledID string
	title := &PageTitle{
		PageID: "page-1",
		Confirm: func(pageID, value string) {
			confirmedID, confirmedTitle = pageID, value
		},
		Cancel: func(pageID string) {
			cancelledID = pageID
		},
	}
	title.Begin("page-1", "")
	tester := ui.NewTester(func(c *ui.Context) {
		title.View(c, theme.DefaultSettings().Semantic(), true)
	}, 720, 520)
	tester.Frame()
	if active, _ := tester.TextInputActive(); !active {
		t.Fatal("page title did not autofocus")
	}
	tester.Type("  Chapter one  ")
	if err := tester.Click("Rename page"); err != nil {
		t.Fatal(err)
	}
	if title.Open {
		t.Fatal("dialog remained open after confirmation")
	}
	if confirmedID != "page-1" || confirmedTitle != "  Chapter one  " {
		t.Fatalf("confirmed = %q, %q", confirmedID, confirmedTitle)
	}
	if cancelledID != "" {
		t.Fatalf("cancel fired on commit: %q", cancelledID)
	}
}

func TestPageTitleCancellationClosesWithoutCommit(t *testing.T) {
	var confirmed, cancelled bool
	title := &PageTitle{
		Confirm: func(string, string) { confirmed = true },
		Cancel:  func(string) { cancelled = true },
	}
	title.Begin("page-1", "Draft")
	tester := ui.NewTester(func(c *ui.Context) {
		title.View(c, theme.DefaultSettings().Semantic(), true)
	}, 720, 520)
	tester.Frame()
	if err := tester.Click("Cancel"); err != nil {
		t.Fatal(err)
	}
	if confirmed || !cancelled || title.Open {
		t.Fatalf("cancel state = confirmed:%v cancelled:%v open:%v", confirmed, cancelled, title.Open)
	}
}

func TestPageTitleSelectsExistingNameOnOpen(t *testing.T) {
	title := &PageTitle{}
	title.Begin("page-1", "Draft")
	tester := ui.NewTester(func(c *ui.Context) {
		title.View(c, theme.DefaultSettings().Semantic(), false)
	}, 720, 520)
	tester.Frame()
	tester.Frame()
	tester.Type("Renamed")
	if title.Value != "Renamed" {
		t.Fatalf("typed replacement = %q", title.Value)
	}
}
