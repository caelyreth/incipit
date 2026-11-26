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

// keepLog directs application logs and crash reports to neverland.log inside dir
// (mygo.PathLogs in production) as well as stderr. It returns the log path, or
// "" if dir is empty or could not be opened.
func keepLog(dir string) string {
	if dir == "" {
		return ""
	}
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
