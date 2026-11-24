package window

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
)

// logLimit is how large the log grows before it is kept aside, once, as
// the previous one.
const logLimit = 1 << 20

// keepLog has the app's log, and the report of a crash that ends it, go
// to Logs/neverland.log in the user-data folder as well as to stderr, so
// what happened to an app opened from the Dock or Finder can be read
// after. It never touches the notebook. It returns the log's path, ""
// where it could not be kept.
func keepLog(userData string) string {
	if userData == "" {
		return ""
	}
	dir := filepath.Join(userData, "Logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("keeping the log: %v", err)
		return ""
	}
	path := filepath.Join(dir, "neverland.log")
	if info, err := os.Stat(path); err == nil && info.Size() > logLimit {
		_ = os.Rename(path, filepath.Join(dir, "neverland.previous.log"))
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("keeping the log: %v", err)
		return ""
	}
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	if err := debug.SetCrashOutput(f, debug.CrashOptions{}); err != nil {
		log.Printf("keeping crash reports: %v", err)
	}
	return path
}
