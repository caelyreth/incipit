// Command fetchassets installs the binary assets Git does not store: the
// fonts, each from a pinned URL, verified against its SHA-256, and the
// word segmenter's CJK dictionary, built from ICU's pinned word list by
// cjkdict. An asset already present and verified is kept. It runs from
// anywhere in the repository, as go generate runs it:
//
//	go tool fetchassets
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// asset is a file installed at path, from the repository's root, from url,
// whose SHA-256 is sum.
type asset struct {
	name, path, url, sum string
}

var assets = []asset{
	// Noto Serif, the prose face, Latin, Greek and Cyrillic: other
	// scripts' faces are downloaded in the app, when asked
	// (internal/app/fonts).
	{"Noto Serif", "internal/ui/theme/fonts/NotoSerif-Variable.ttf",
		"https://raw.githubusercontent.com/google/fonts/06bfe0d62a98375cb09c942fa18e68bd32cff057/ofl/notoserif/NotoSerif%5Bwdth%2Cwght%5D.ttf",
		"4d8e6761424656867019081a1a01336f3cb086982682698714054fc33f782713"},
	{"Noto Serif Italic", "internal/ui/theme/fonts/NotoSerif-Italic-Variable.ttf",
		"https://raw.githubusercontent.com/google/fonts/06bfe0d62a98375cb09c942fa18e68bd32cff057/ofl/notoserif/NotoSerif-Italic%5Bwdth%2Cwght%5D.ttf",
		"e87acbc6c0efd0d9a20d6a8cbbda2b266c14be3a3a6f5af8ec9d7b2460570ad1"},
	// Noto Sans, the prose face set Sans, as Noto Serif.
	{"Noto Sans", "internal/ui/theme/fonts/NotoSans-Variable.ttf",
		"https://raw.githubusercontent.com/google/fonts/06bfe0d62a98375cb09c942fa18e68bd32cff057/ofl/notosans/NotoSans%5Bwdth%2Cwght%5D.ttf",
		"bfb7bb691513f12e734dc346c03a03f784912432d7e3fa8e56efcf906fe86b3d"},
	{"Noto Sans Italic", "internal/ui/theme/fonts/NotoSans-Italic-Variable.ttf",
		"https://raw.githubusercontent.com/google/fonts/06bfe0d62a98375cb09c942fa18e68bd32cff057/ofl/notosans/NotoSans-Italic%5Bwdth%2Cwght%5D.ttf",
		"58e6e0ebd1931b29a365aa2d3e2ee9a9e831a3af7cf3ad1462d4e72154f0b291"},
	{"New Computer Modern Math", "internal/ui/theme/fonts/NewCMMath-Book.otf",
		"https://raw.githubusercontent.com/typst/typst-assets/94dcb990edb36311ef85162baa26e4a8205e0bf0/files/fonts/NewCMMath-Book.otf",
		"2ea09ebc9167b1e1a66f31390dc917f2d4004ecfca72d51b28010e4ad6becd95"},
}

func main() {
	log.SetFlags(0)
	root, err := repositoryRoot()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		log.Fatal(err)
	}
	for _, a := range assets {
		if err := fetch(a); err != nil {
			log.Fatalf("%s: %v", a.name, err)
		}
	}
	// ICU release-78.3 cjdict.txt, indexed in Neverland's format for the
	// word segmenter, both pinned (tools/cjkdict). go generate names the
	// toolchain it runs in GOROOT.
	goCmd := "go"
	if r := os.Getenv("GOROOT"); r != "" {
		goCmd = filepath.Join(r, "bin", "go")
	}
	cmd := exec.Command(goCmd, "tool", "cjkdict")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("the CJK dictionary: %v", err)
	}
}

// repositoryRoot is the nearest folder up from here holding the module's
// go.mod.
func repositoryRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil && bytes.Contains(data, []byte("module github.com/caelyreth/neverland\n")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("not in the Neverland repository")
		}
		dir = parent
	}
}

// fetch installs a, unless it is there and verified: downloaded, retried a
// few times, verified, then moved into place.
func fetch(a asset) error {
	if have, err := os.ReadFile(a.path); err == nil {
		if sum(have) == a.sum {
			log.Printf("%s is already present and verified.", a.name)
			return nil
		}
		log.Printf("Replacing mismatched %s.", a.name)
	}
	log.Printf("Fetching %s...", a.name)
	data, err := download(a.url)
	if err != nil {
		return err
	}
	if got := sum(data); got != a.sum {
		return fmt.Errorf("checksum mismatch\nexpected: %s\nactual:   %s", a.sum, got)
	}
	if err := os.MkdirAll(filepath.Dir(a.path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(a.path), filepath.Base(a.path)+".tmp.*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), a.path); err != nil {
		return err
	}
	log.Printf("Installed %s at %s", a.name, a.path)
	return nil
}

// download reads url, trying again a few times as it fails.
func download(url string) ([]byte, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	var last error
	for attempt := range 5 {
		if attempt > 0 {
			time.Sleep(time.Second)
		}
		resp, err := client.Get(url)
		if err != nil {
			last = err
			continue
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err == nil && resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("%s: %s", url, resp.Status)
		}
		if err == nil {
			return data, nil
		}
		last = err
	}
	return nil, last
}

func sum(data []byte) string {
	s := sha256.Sum256(data)
	return hex.EncodeToString(s[:])
}
