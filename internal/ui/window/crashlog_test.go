package window

import (
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

// TestTheLogIsKeptInUserData writes the log to Logs/neverland.log in the
// user-data folder, keeping a log grown too large aside first.
func TestTheLogIsKeptInUserData(t *testing.T) {
	dir := t.TempDir()
	logs := filepath.Join(dir, "Logs")
	if err := os.MkdirAll(logs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logs, "neverland.log"), make([]byte, logLimit+1), 0o644); err != nil {
		t.Fatal(err)
	}
	defer func() {
		log.SetOutput(os.Stderr)
		_ = debug.SetCrashOutput(nil, debug.CrashOptions{})
	}()
	path := keepLog(dir)
	log.Print("a line to keep")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "a line to keep") || len(data) > logLimit {
		t.Fatalf("log holds %d bytes: %q", len(data), data)
	}
	if _, err := os.Stat(filepath.Join(logs, "neverland.previous.log")); err != nil {
		t.Fatal("the large log was not kept aside:", err)
	}
}
