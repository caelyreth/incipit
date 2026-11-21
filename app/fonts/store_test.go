package fonts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// served is a face of one file the test server serves as body, going on
// from a range as a server does; fail, when set, answers a request first.
func served(t *testing.T, body string, sum string, fail ...func(w http.ResponseWriter, r *http.Request) bool) (*Store, Face) {
	t.Helper()
	backoff = time.Millisecond
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/s/notoserifx/v1/x.ttf" {
			http.NotFound(w, r)
			return
		}
		requests++
		if requests <= len(fail) && fail[requests-1](w, r) {
			return
		}
		http.ServeContent(w, r, "x.ttf", time.Time{}, strings.NewReader(body))
	}))
	t.Cleanup(srv.Close)
	if sum == "" {
		s := sha256.Sum256([]byte(body))
		sum = hex.EncodeToString(s[:])
	}
	f := Face{ID: "x", Family: "Noto Serif X", Files: []File{{Path: "s/notoserifx/v1/x.ttf", Name: "NotoSerifX-400.ttf", Weight: 400, Size: int64(len(body)), SHA256: sum}}}
	return &Store{Dir: t.TempDir(), Hosts: []string{srv.URL}, Client: srv.Client()}, f
}

// TestInstallMeetsAFailingNetwork goes on from where a dropped connection
// left off, and asks again of a server that is busy.
func TestInstallMeetsAFailingNetwork(t *testing.T) {
	body := strings.Repeat("a font's bytes ", 40000)
	var ranged string
	dropped := func(w http.ResponseWriter, r *http.Request) bool {
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.Write([]byte(body[:len(body)/2]))
		return true
	}
	busy := func(w http.ResponseWriter, r *http.Request) bool {
		ranged = r.Header.Get("Range")
		w.WriteHeader(http.StatusServiceUnavailable)
		return true
	}
	s, f := served(t, body, "", dropped, busy)
	if err := s.Install(context.Background(), f, nil); err != nil {
		t.Fatal(err)
	}
	if ranged != "bytes="+strconv.Itoa(len(body)/2)+"-" {
		t.Errorf("asked again for %q, not from where it was left", ranged)
	}
	if data, err := s.Read(f); err != nil || string(data[0]) != body {
		t.Errorf("read back %d bytes, %v", len(data), err)
	}
}

// TestInstallTriesTheNextHost asks the next host when one cannot be
// reached, as the mirror for mainland China is asked.
func TestInstallTriesTheNextHost(t *testing.T) {
	s, f := served(t, "a font's bytes", "")
	down := httptest.NewServer(http.NotFoundHandler())
	down.Close()
	s.Hosts = append([]string{down.URL}, s.Hosts...)
	if err := s.Install(context.Background(), f, nil); err != nil || !s.Installed(f) {
		t.Fatalf("with the first host down: %v", err)
	}
}

// TestInstallGoesOnAfterAStop keeps what came of a stopped download and
// goes on from it, and asks nothing again of a file not there.
func TestInstallGoesOnAfterAStop(t *testing.T) {
	body := strings.Repeat("a font's bytes ", 40000)
	s, f := served(t, body, "")
	ctx, cancel := context.WithCancel(context.Background())
	err := s.Install(ctx, f, func(done, total int64) {
		if done >= total/3 {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("stopped: %v", err)
	}
	if got := s.Partial(f); got <= 0 || got >= f.Size() || s.Installed(f) {
		t.Fatalf("stopped, %d of %d kept, installed %v", got, f.Size(), s.Installed(f))
	}
	var from int64 = -1
	if err := s.Install(context.Background(), f, func(done, _ int64) {
		if from < 0 {
			from = done
		}
	}); err != nil || !s.Installed(f) {
		t.Fatalf("went on: %v", err)
	}
	if from <= 0 {
		t.Errorf("went on from %d, not from what had come", from)
	}
	missing := f
	missing.Files = []File{{Path: "s/notoserifx/v1/missing.ttf", Name: "Missing.ttf", Size: 4, SHA256: f.Files[0].SHA256}}
	if err := s.Install(context.Background(), missing, nil); err == nil {
		t.Error("a file not there installed")
	}
}

// TestInstallVerifiesAndKeeps downloads a face, telling its progress,
// keeps it, reads it back, and removes it.
func TestInstallVerifiesAndKeeps(t *testing.T) {
	s, f := served(t, "a font's bytes", "")
	var last, total int64
	if err := s.Install(context.Background(), f, func(d, n int64) { last, total = d, n }); err != nil {
		t.Fatal(err)
	}
	if last != f.Size() || total != f.Size() {
		t.Errorf("progress ended at %d of %d, want %d", last, total, f.Size())
	}
	if !s.Installed(f) {
		t.Fatal("installed, it is not kept")
	}
	data, err := s.Read(f)
	if err != nil || len(data) != 1 || string(data[0]) != "a font's bytes" {
		t.Fatalf("read back %q, %v", data, err)
	}
	if err := s.Remove(f); err != nil || s.Installed(f) {
		t.Errorf("removed (%v), it is still kept", err)
	}
}

// TestInstallRefusesWhatIsNotPinned keeps nothing whose bytes are not the
// pinned ones, and reads a kept file changed since as missing.
func TestInstallRefusesWhatIsNotPinned(t *testing.T) {
	s, f := served(t, "other bytes, as long", "00")
	if err := s.Install(context.Background(), f, nil); !errors.Is(err, ErrMismatch) {
		t.Fatalf("installing bytes not pinned: %v", err)
	}
	if s.Installed(f) {
		t.Error("bytes not pinned were kept")
	}
	entries, _ := os.ReadDir(s.Dir)
	if len(entries) != 0 {
		t.Errorf("left behind: %v", entries)
	}
	good, f2 := served(t, "a font's bytes", "")
	if err := good.Install(context.Background(), f2, nil); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(good.path(f2.Files[0]), []byte("a font's BYTES"), 0o644)
	if _, err := good.Read(f2); !errors.Is(err, ErrMismatch) || good.Installed(f2) {
		t.Errorf("a changed file read as %v, still kept %v", err, good.Installed(f2))
	}
}

// TestFamilyOrder puts the chosen Han face first among the installed, the
// rest in the catalog's order, after the Latin face.
func TestFamilyOrder(t *testing.T) {
	if got := Family("Noto Serif", nil, "jp"); got != "Noto Serif" {
		t.Errorf("none installed: %q", got)
	}
	got := Family("Noto Serif", []string{"thai", "sc", "jp"}, "jp")
	if want := "Noto Serif, Noto Serif JP, Noto Serif SC, Noto Serif Thai"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := Family("Noto Serif", []string{"sc", "jp"}, ""); got != "Noto Serif, Noto Serif SC, Noto Serif JP" {
		t.Errorf("no Han choice: %q", got)
	}
}

// TestCatalogIsPinned has every face's files pinned, and every id once.
func TestCatalogIsPinned(t *testing.T) {
	seen := map[string]bool{}
	for _, f := range Catalog {
		if seen[f.ID] || f.Family == "" || f.Sample == "" || len(f.Files) == 0 {
			t.Errorf("face %+v", f)
		}
		seen[f.ID] = true
		for _, file := range f.Files {
			if len(file.SHA256) != 64 || file.Size <= 0 {
				t.Errorf("%s: %+v is not pinned", f.ID, file)
			}
		}
	}
}
