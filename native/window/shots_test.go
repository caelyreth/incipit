package window

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// shot saves the tester's frame as name.png in the NEVERLAND_SHOTS
// directory, when it is set, so a run can be looked at:
//
//	NEVERLAND_SHOTS=/tmp/shots go test ./native/window/
func shot(t *testing.T, tester *ui.Tester, name string) {
	t.Helper()
	dir := os.Getenv("NEVERLAND_SHOTS")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("shots: %v", err)
	}
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatalf("shots: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, tester.Image()); err != nil {
		t.Fatalf("shots: %v", err)
	}
}
