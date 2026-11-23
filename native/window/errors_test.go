package window

import (
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"github.com/caelyreth/neverland/app/library"
)

// TestNoticesSayWhyInTheAppsWords says a failure the app knows in its
// own words, however wrapped, and any other as the system gives it.
func TestNoticesSayWhyInTheAppsWords(t *testing.T) {
	for err, want := range map[error]string{
		fmt.Errorf("opening: %w", fs.ErrPermission): "Neverland isn't allowed to use the file. Check its permissions.",
		fmt.Errorf("moving: %w", library.ErrCycle):  "A group can't go inside itself.",
		errors.New("the network went away"):         "The network went away",
	} {
		if got := describeNotice(err); got != want {
			t.Errorf("describeNotice(%v) = %q, want %q", err, got, want)
		}
	}
}
