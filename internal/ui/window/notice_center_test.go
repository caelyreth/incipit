package window

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

const (
	windowWidth  = 800
	windowHeight = 600
	footer       = 32
)

// window shows center's notices as the app's window does.
func noticeTester(center *NoticeCenter) *ui.Tester {
	semantic := theme.DefaultSettings().Semantic()
	tester := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.DefaultSettings().MyGoTheme())
		center.View(c, semantic, false, footer)
	}, windowWidth, windowHeight)
	tester.Frame()
	return tester
}

func TestANoticeAboutTheSameTakesThePlaceOfTheLast(t *testing.T) {
	center := &NoticeCenter{}
	tester := noticeTester(center)
	center.Report("Couldn't move the page", errors.New("first"))
	center.Report("Couldn't move the page", errors.New("second"))
	center.Post(Notice{ID: "save", Kind: NoticeError, Title: "Saving failed"})
	center.Post(Notice{ID: "save", Kind: NoticeError, Title: "Saving failed again"})
	tester.Frame()
	for _, gone := range []string{"First", "Saving failed"} {
		if _, ok := tester.Find(gone); ok {
			t.Fatalf("%q still shows beside the notice that replaced it", gone)
		}
	}
	if !tester.HasText("Second") || !tester.HasText("Saving failed again") {
		t.Fatalf("the later notices did not show: %v", tester.Texts())
	}
}

func TestReportSaysWhatFailedAndWhy(t *testing.T) {
	center := &NoticeCenter{}
	tester := noticeTester(center)
	center.Report("Couldn't open the page", nil)
	tester.Frame()
	if tester.HasText("Couldn't open the page") {
		t.Fatal("a nil error was reported")
	}
	center.Report("Couldn't open the page", errors.New("permission denied"))
	tester.Frame()
	if !tester.HasText("Couldn't open the page") || !tester.HasText("Permission denied") {
		t.Fatalf("the failure shows as %v", tester.Texts())
	}
}

func TestDismissClosesOnlyItsNotice(t *testing.T) {
	center := &NoticeCenter{}
	tester := noticeTester(center)
	center.Post(Notice{ID: "save", Sticky: true, Title: "Saving failed"})
	center.Post(Notice{Title: "Moved"})
	center.Dismiss("")
	tester.Frame()
	center.Dismiss("save")
	tester.Frame()
	if tester.HasText("Saving failed") || !tester.HasText("Moved") {
		t.Fatalf("after dismissing save: %v", tester.Texts())
	}
}

func TestTheActionRunsAndCloses(t *testing.T) {
	center := &NoticeCenter{}
	tester := noticeTester(center)
	ran := 0
	center.Post(Notice{Kind: NoticeError, Title: "Saving failed", Sticky: true,
		Action: "Retry now", OnAction: func() { ran++ }})
	tester.Frame()
	if err := tester.Click("Retry now"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if ran != 1 || tester.HasText("Saving failed") {
		t.Fatalf("the action ran %d times, and the notice shows: %v", ran, tester.HasText("Saving failed"))
	}
}

func TestCloseClosesTheNotice(t *testing.T) {
	center := &NoticeCenter{}
	tester := noticeTester(center)
	center.Post(Notice{Title: "Moved", Sticky: true})
	tester.Frame()
	if err := tester.Click("Close"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if tester.HasText("Moved") {
		t.Fatal("the notice stayed after Close")
	}
}

func TestNoticesGoInTheirTimeAndStickyOnesStay(t *testing.T) {
	center := &NoticeCenter{}
	tester := noticeTester(center)
	center.Post(Notice{Kind: NoticeSuccess, Title: "All changes saved"})
	center.Post(Notice{Kind: NoticeError, Title: "Couldn't move the page"})
	center.Post(Notice{Kind: NoticeError, Title: "Saving failed", Sticky: true})
	tester.Frame()
	tester.Advance(5 * time.Second)
	if tester.HasText("All changes saved") || !tester.HasText("Couldn't move the page") {
		t.Fatalf("after 5s: %v", tester.Texts())
	}
	tester.Advance(time.Minute)
	if tester.HasText("Couldn't move the page") || !tester.HasText("Saving failed") {
		t.Fatalf("after a minute: %v", tester.Texts())
	}
}

func TestPostingFromAnyGoroutineWakesTheWindow(t *testing.T) {
	center := &NoticeCenter{}
	var woken atomic.Int32
	center.SetWake(func() { woken.Add(1) })
	tester := noticeTester(center)
	var group sync.WaitGroup
	for range 8 {
		group.Go(func() { center.Post(Notice{Title: "Moved"}) })
	}
	group.Wait()
	tester.Frame()
	if woken.Load() != 8 || !tester.HasText("Moved") {
		t.Fatalf("woken %d times; shows %v", woken.Load(), tester.Texts())
	}
}

func TestEveryKindHasItsLook(t *testing.T) {
	semantic := theme.DefaultSettings().Semantic()
	icons, colors := map[string]bool{}, map[ui.Color]bool{}
	for _, kind := range []NoticeKind{NoticeInfo, NoticeSuccess, NoticeWarning, NoticeError} {
		if kind.icon().SVG() == nil {
			t.Fatalf("%v has no icon", kind)
		}
		icons[string(kind.icon())] = true
		colors[kind.color(semantic)] = true
	}
	if len(icons) != 4 || len(colors) != 4 {
		t.Fatalf("kinds share a look: %v icons, %v colors", len(icons), len(colors))
	}
}
