package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

func shaHex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func TestPrepareDraftFlattensMergesAndVerifiesChecksums(t *testing.T) {
	inDir := t.TempDir()
	outDir := t.TempDir()

	linuxDir := filepath.Join(inDir, "dist-linux")
	macDir := filepath.Join(inDir, "dist-darwin", "Neverland.app")
	if err := os.MkdirAll(linuxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(macDir, 0o755); err != nil {
		t.Fatal(err)
	}

	tarZst := []byte("zstd archive payload")
	dmg := []byte("dmg image payload")
	installSh := []byte("#!/bin/sh\necho install\n")
	manifest := []byte(`{"version":"1.0.0-alpha.1","target":"linux-amd64","channel":"beta"}`)
	history := []byte(`{"version":"1.0.0-alpha.1"}`)

	if err := os.WriteFile(filepath.Join(linuxDir, "neverland-1.0.0-alpha.1-linux-amd64.tar.zst"), tarZst, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(linuxDir, "install.sh"), installSh, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(linuxDir, "update-linux-amd64.json"), manifest, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(linuxDir, "history-linux-amd64.json"), history, 0o644); err != nil {
		t.Fatal(err)
	}
	// Duplicate install.sh under arm64 dir must deduplicate cleanly; .app bundle must be skipped.
	armDir := filepath.Join(inDir, "dist-linux-arm64")
	if err := os.MkdirAll(armDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(armDir, "install.sh"), installSh, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(macDir), "neverland-1.0.0-alpha.1-universal.dmg"), dmg, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(macDir, "Info.plist"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(linuxDir, checksumsName), formatChecksums(map[string]string{
		"neverland-1.0.0-alpha.1-linux-amd64.tar.zst": shaHex(tarZst),
		"install.sh": shaHex(installSh),
	}), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := prepareDraft(inDir, outDir); err != nil {
		t.Fatalf("prepareDraft: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "Info.plist")); err == nil {
		t.Fatal(".app bundle contents must not be staged")
	}
	sumsRaw, err := os.ReadFile(filepath.Join(outDir, checksumsName))
	if err != nil {
		t.Fatal(err)
	}
	sums := parseChecksums(sumsRaw)
	for _, want := range []string{"neverland-1.0.0-alpha.1-linux-amd64.tar.zst", "neverland-1.0.0-alpha.1-universal.dmg", "install.sh"} {
		if sums[want] == "" {
			t.Fatalf("missing %s in merged SHA256SUMS: %s", want, sumsRaw)
		}
	}
}

func TestDeployVerifiesChecksumsAndUploadsInStrictOrder(t *testing.T) {
	dir := t.TempDir()
	dmg := []byte("dmg bytes")
	delta := []byte("delta bytes")
	history := []byte(`{"target":"darwin-universal","channel":"beta","history":[]}`)
	payload := base64.StdEncoding.EncodeToString([]byte(`{"version":"1.0.0-alpha.1","target":"darwin-universal","channel":"beta"}`))
	manifest := []byte(fmt.Sprintf(`{"v":2,"payload":%q,"signatures":[{"kid":"k1","sig":"c2ln"},{"kid":"k2","sig":"c2lnMg=="}]}`, payload))

	for name, data := range map[string][]byte{
		"neverland-1.0.0-alpha.1-universal.dmg":               dmg,
		"neverland-1.0.0-to-1.0.0-alpha.1-darwin-universal.delta": delta,
		"history-darwin-universal.json":                         history,
		"update-darwin-universal.json":                          manifest,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, checksumsName), formatChecksums(map[string]string{
		"neverland-1.0.0-alpha.1-universal.dmg":                   "0000000000000000000000000000000000000000000000000000000000000000",
		"neverland-1.0.0-to-1.0.0-alpha.1-darwin-universal.delta": shaHex(delta),
	}), 0o644); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var ops []string
	r2Srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		ops = append(ops, "r2:"+r.Method+":"+r.URL.Path+":"+r.Header.Get("Cache-Control"))
		mu.Unlock()
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ") {
			http.Error(w, "missing sigv4", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer r2Srv.Close()

	workerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		ops = append(ops, "worker:"+r.Method+":"+r.URL.Path+":"+r.Header.Get("Authorization"))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer workerSrv.Close()

	cfg := deployConfig{
		Dir:           dir,
		ExpectedVer:   "1.0.0-alpha.1",
		R2Endpoint:    r2Srv.URL,
		R2Bucket:      "neverland-dl",
		R2Prefix:      "neverland",
		R2Region:      "auto",
		R2AccessKey:   "testkey",
		R2SecretKey:   "testsecret",
		ManifestURL:   workerSrv.URL,
		ManifestToken: "secret-token",
	}

	// Corrupted file must abort BEFORE any network upload occurs.
	var logBuf bytes.Buffer
	if err := deploy(cfg, &logBuf); err == nil || len(ops) != 0 {
		t.Fatalf("expected checksum mismatch abort before network calls; err=%v ops=%v", err, ops)
	}

	// Fix checksum and deploy cleanly.
	if err := os.WriteFile(filepath.Join(dir, checksumsName), formatChecksums(map[string]string{
		"neverland-1.0.0-alpha.1-universal.dmg":                   shaHex(dmg),
		"neverland-1.0.0-to-1.0.0-alpha.1-darwin-universal.delta": shaHex(delta),
	}), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := deploy(cfg, &logBuf); err != nil {
		t.Fatalf("deploy: %v", err)
	}

	wantOps := []string{
		"r2:PUT:/neverland-dl/neverland/neverland-1.0.0-alpha.1-universal.dmg:public, max-age=31536000, immutable",
		"r2:PUT:/neverland-dl/neverland/neverland-1.0.0-to-1.0.0-alpha.1-darwin-universal.delta:public, max-age=31536000, immutable",
		"r2:PUT:/neverland-dl/neverland/SHA256SUMS:no-cache",
		"r2:PUT:/neverland-dl/neverland/beta/history-darwin-universal.json:no-cache",
		"worker:PUT:/beta/update-darwin-universal.json:Bearer secret-token",
	}
	if !slices.Equal(ops, wantOps) {
		t.Fatalf("deploy sequence:\ngot:  %v\nwant: %v", ops, wantOps)
	}
}
