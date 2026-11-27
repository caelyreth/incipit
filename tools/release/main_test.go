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
	macDir := filepath.Join(inDir, "dist-darwin", "Incipit.app")
	if err := os.MkdirAll(linuxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(macDir, 0o755); err != nil {
		t.Fatal(err)
	}

	tarZst := []byte("zstd archive payload")
	debPkg := []byte("deb package payload")
	rpmPkg := []byte("rpm package payload")
	dmg := []byte("dmg image payload")
	installSh := []byte("#!/bin/sh\necho install\n")
	manifest := []byte(`{"version":"1.0.0-alpha.1","target":"linux-amd64","channel":"prerelease"}`)
	history := []byte(`{"version":"1.0.0-alpha.1"}`)

	if err := os.WriteFile(filepath.Join(linuxDir, "incipit-1.0.0-alpha.1-linux-amd64.tar.zst"), tarZst, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(linuxDir, "incipit_1.0.0~alpha.1_amd64.deb"), debPkg, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(linuxDir, "incipit-1.0.0~alpha.1-1.x86_64.rpm"), rpmPkg, 0o644); err != nil {
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
	if err := os.WriteFile(filepath.Join(linuxDir, "PKGBUILD"), []byte("pkgname=incipit\narch=('x86_64')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Duplicate install.sh under arm64 dir must deduplicate cleanly; per-arch PKGBUILD and .app bundle must be skipped.
	armDir := filepath.Join(inDir, "dist-linux-arm64")
	if err := os.MkdirAll(armDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(armDir, "install.sh"), installSh, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(armDir, "PKGBUILD"), []byte("pkgname=incipit\narch=('aarch64')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(macDir), "incipit-1.0.0-alpha.1-universal.dmg"), dmg, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(macDir, "Info.plist"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(linuxDir, checksumsName), formatChecksums(map[string]string{
		"incipit-1.0.0-alpha.1-linux-amd64.tar.zst": shaHex(tarZst),
		"incipit_1.0.0~alpha.1_amd64.deb":           shaHex(debPkg),
		"incipit-1.0.0~alpha.1-1.x86_64.rpm":        shaHex(rpmPkg),
		"install.sh":                                shaHex(installSh),
	}), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := prepareDraft(inDir, outDir); err != nil {
		t.Fatalf("prepareDraft: %v", err)
	}
	for _, skipped := range []string{"Info.plist", "PKGBUILD", "incipit_1.0.0~alpha.1_amd64.deb", "incipit-1.0.0~alpha.1-1.x86_64.rpm"} {
		if _, err := os.Stat(filepath.Join(outDir, skipped)); err == nil {
			t.Fatalf("%s must not be staged", skipped)
		}
	}
	sumsRaw, err := os.ReadFile(filepath.Join(outDir, checksumsName))
	if err != nil {
		t.Fatal(err)
	}
	sums := parseChecksums(sumsRaw)
	for _, want := range []string{
		"incipit-1.0.0-alpha.1-linux-amd64.tar.zst",
		"incipit-1.0.0-alpha.1-universal.dmg",
		"incipit_1.0.0.alpha.1_amd64.deb",
		"incipit-1.0.0.alpha.1-1.x86_64.rpm",
		"install.sh",
	} {
		if sums[want] == "" {
			t.Fatalf("missing %s in merged SHA256SUMS: %s", want, sumsRaw)
		}
	}
}

func TestDeployVerifiesChecksumsAndUploadsInStrictOrder(t *testing.T) {
	dir := t.TempDir()
	dmg := []byte("dmg bytes")
	delta := []byte("delta bytes")
	installSh := []byte("#!/bin/sh\n")
	history := []byte(`{"target":"darwin-arm64","channel":"prerelease","history":[]}`)
	payload := base64.StdEncoding.EncodeToString([]byte(`{"version":"1.0.0-alpha.1","target":"darwin-arm64","channel":"prerelease","artifact":{"url":"https://dl.example.com/incipit/releases/v1.0.0-alpha.1/incipit-1.0.0-alpha.1-darwin-arm64.tar.zst"}}`))
	manifest := []byte(fmt.Sprintf(`{"v":2,"payload":%q,"signatures":[{"kid":"k1","sig":"c2ln"},{"kid":"k2","sig":"c2lnMg=="}]}`, payload))

	for name, data := range map[string][]byte{
		"incipit-1.0.0-alpha.1-arm64.dmg":                       dmg,
		"incipit-1.0.0-to-1.0.0-alpha.1-darwin-arm64.delta":     delta,
		"install.sh":                                            installSh,
		"history-darwin-arm64.json":                             history,
		"update-darwin-arm64.json":                              manifest,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, checksumsName), formatChecksums(map[string]string{
		"incipit-1.0.0-alpha.1-arm64.dmg":                   "0000000000000000000000000000000000000000000000000000000000000000",
		"incipit-1.0.0-to-1.0.0-alpha.1-darwin-arm64.delta": shaHex(delta),
		"install.sh":                                        shaHex(installSh),
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

	kvSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		ops = append(ops, "kv:"+r.Method+":"+r.URL.Path+":"+r.Header.Get("Authorization"))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"errors":[]}`))
	}))
	defer kvSrv.Close()

	cfg := deployConfig{
		Dir:             dir,
		ExpectedVer:     "1.0.0-alpha.1",
		RequireKV:       true,
		R2Endpoint:      r2Srv.URL,
		R2Bucket:        "incipit-dl",
		R2Prefix:        "incipit",
		R2Region:        "auto",
		R2AccessKey:     "testkey",
		R2SecretKey:     "testsecret",
		CFAccountID:     "acct-123",
		CFKVNamespaceID: "ns-456",
		CFAPIToken:      "cf-secret-token",
		CFAPIBase:       kvSrv.URL,
	}

	// Corrupted file must abort BEFORE any network upload occurs.
	var logBuf bytes.Buffer
	if err := deploy(cfg, &logBuf); err == nil || len(ops) != 0 {
		t.Fatalf("expected checksum mismatch abort before network calls; err=%v ops=%v", err, ops)
	}

	// Fix checksum; incomplete KV credentials must also abort BEFORE R2 uploads occur.
	if err := os.WriteFile(filepath.Join(dir, checksumsName), formatChecksums(map[string]string{
		"incipit-1.0.0-alpha.1-arm64.dmg":                   shaHex(dmg),
		"incipit-1.0.0-to-1.0.0-alpha.1-darwin-arm64.delta": shaHex(delta),
		"install.sh":                                        shaHex(installSh),
	}), 0o644); err != nil {
		t.Fatal(err)
	}
	incompleteCfg := cfg
	incompleteCfg.CFAPIToken = ""
	if err := deploy(incompleteCfg, &logBuf); err == nil || len(ops) != 0 {
		t.Fatalf("expected missing KV token abort before network calls; err=%v ops=%v", err, ops)
	}
	if err := deploy(cfg, &logBuf); err != nil {
		t.Fatalf("deploy: %v", err)
	}

	wantOps := []string{
		"r2:PUT:/incipit-dl/incipit/releases/v1.0.0-alpha.1/incipit-1.0.0-alpha.1-arm64.dmg:public, max-age=31536000, immutable",
		"r2:PUT:/incipit-dl/incipit/releases/v1.0.0-alpha.1/incipit-1.0.0-to-1.0.0-alpha.1-darwin-arm64.delta:public, max-age=31536000, immutable",
		"r2:PUT:/incipit-dl/incipit/releases/v1.0.0-alpha.1/SHA256SUMS:no-cache",
		"r2:PUT:/incipit-dl/incipit/install.sh:no-cache",
		"r2:PUT:/incipit-dl/incipit/prerelease/history-darwin-arm64.json:no-cache",
		"kv:PUT:/accounts/acct-123/storage/kv/namespaces/ns-456/values/manifest:prerelease:darwin-arm64:1.0.0-alpha.1:Bearer cf-secret-token",
		"kv:PUT:/accounts/acct-123/storage/kv/namespaces/ns-456/values/latest:prerelease:darwin-arm64:Bearer cf-secret-token",
	}
	if !slices.Equal(ops, wantOps) {
		t.Fatalf("deploy sequence:\ngot:  %v\nwant: %v", ops, wantOps)
	}
}

func TestPruneProtectsLiveKVHistoryLocksAndRespectsKeepLimits(t *testing.T) {
	objects := map[string]string{
		"incipit/install.sh":                                        "#!/bin/sh\n",
		"incipit/prerelease/history-darwin-arm64.json":              `{"history":[{"version":"1.3.0-alpha.4"},{"version":"1.3.0-alpha.1"}]}`,
		"incipit/releases/v1.0.0/app-1.0.0.dmg":                     "1.0.0",
		"incipit/releases/v1.1.0/app-1.1.0.dmg":                     "1.1.0",
		"incipit/releases/v1.1.0/SHA256SUMS":                        "sum",
		"incipit/releases/v1.2.0/app-1.2.0.dmg":                     "1.2.0",
		"incipit/releases/v1.3.0-alpha.1/app-1.3.0-alpha.1.tar.zst": "a1",
		"incipit/releases/v1.3.0-alpha.2/app-1.3.0-alpha.2.dmg":     "a2",
		"incipit/releases/v1.3.0-alpha.2/SHA256SUMS":                "sum",
		"incipit/releases/v1.3.0-alpha.3/app-1.3.0-alpha.3.dmg":     "a3",
		"incipit/releases/v1.3.0-alpha.4/app-1.3.0-alpha.4.dmg":     "a4",
	}
	latestPayload := base64.StdEncoding.EncodeToString([]byte(`{"version":"1.2.0","target":"darwin-arm64","channel":"release"}`))
	kvData := map[string]string{
		"latest:release:darwin-arm64": `{"v":2,"payload":"` + latestPayload + `","signatures":[{"kid":"k1","sig":"s"}]}`,
		"lock:release:darwin-arm64":   "1.0.0",
	}

	var mu sync.Mutex
	var kvFail bool
	var deleted []string

	r2Srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if r.URL.Query().Get("list-type") == "2" {
				prefix := r.URL.Query().Get("prefix")
				var matched []string
				mu.Lock()
				for k := range objects {
					if strings.HasPrefix(k, prefix) {
						matched = append(matched, k)
					}
				}
				mu.Unlock()
				slices.Sort(matched)
				// Force two pages when listing releases/ to test ContinuationToken handling.
				cont := r.URL.Query().Get("continuation-token")
				page := matched
				next := ""
				truncated := false
				if prefix == "incipit/releases/" && len(matched) > 3 {
					if cont == "" {
						page = matched[:3]
						next = "page2"
						truncated = true
					} else if cont == "page2" {
						page = matched[3:]
					}
				}
				var xmlBuf strings.Builder
				xmlBuf.WriteString(`<ListBucketResult>`)
				if truncated {
					xmlBuf.WriteString(`<IsTruncated>true</IsTruncated><NextContinuationToken>` + next + `</NextContinuationToken>`)
				} else {
					xmlBuf.WriteString(`<IsTruncated>false</IsTruncated>`)
				}
				for _, k := range page {
					xmlBuf.WriteString(`<Contents><Key>` + k + `</Key></Contents>`)
				}
				xmlBuf.WriteString(`</ListBucketResult>`)
				_, _ = w.Write([]byte(xmlBuf.String()))
				return
			}
			key := strings.TrimPrefix(r.URL.Path, "/incipit-dl/")
			mu.Lock()
			val, ok := objects[key]
			mu.Unlock()
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write([]byte(val))
		case http.MethodDelete:
			key := strings.TrimPrefix(r.URL.Path, "/incipit-dl/")
			mu.Lock()
			deleted = append(deleted, key)
			delete(objects, key)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer r2Srv.Close()

	kvSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		fail := kvFail
		mu.Unlock()
		if fail {
			http.Error(w, "kv unavailable", http.StatusInternalServerError)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/keys") {
			prefix := r.URL.Query().Get("prefix")
			var names []string
			for k := range kvData {
				if strings.HasPrefix(k, prefix) {
					names = append(names, fmt.Sprintf(`{"name":%q}`, k))
				}
			}
			slices.Sort(names)
			_, _ = fmt.Fprintf(w, `{"success":true,"result":[%s],"result_info":{"cursor":""}}`, strings.Join(names, ","))
			return
		}
		_, key, ok := strings.Cut(r.URL.Path, "/values/")
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		val, exists := kvData[key]
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(val))
	}))
	defer kvSrv.Close()

	cfg := deployConfig{
		RequireKV:       true,
		KeepRelease:     1,
		KeepPrerelease:  2,
		DryRun:          true,
		R2Endpoint:      r2Srv.URL,
		R2Bucket:        "incipit-dl",
		R2Prefix:        "incipit",
		R2Region:        "auto",
		R2AccessKey:     "testkey",
		R2SecretKey:     "testsecret",
		CFAccountID:     "acct-123",
		CFKVNamespaceID: "ns-456",
		CFAPIToken:      "cf-secret-token",
		CFAPIBase:       kvSrv.URL,
	}

	// KV failure must abort before any R2 delete calls.
	mu.Lock()
	kvFail = true
	mu.Unlock()
	var buf bytes.Buffer
	if err := prune(cfg, &buf); err == nil || len(deleted) != 0 {
		t.Fatalf("expected KV error abort; err=%v deleted=%v", err, deleted)
	}

	// Dry-run plans deletions without deleting anything.
	mu.Lock()
	kvFail = false
	mu.Unlock()
	buf.Reset()
	if err := prune(cfg, &buf); err != nil {
		t.Fatalf("prune dry-run: %v", err)
	}
	if len(deleted) != 0 {
		t.Fatalf("dry-run must not delete objects: %v", deleted)
	}

	// Real prune deletes only v1.1.0 (not latest, not locked) and v1.3.0-alpha.2 (outside keep=2, not in history).
	cfg.DryRun = false
	buf.Reset()
	if err := prune(cfg, &buf); err != nil {
		t.Fatalf("prune: %v\n%s", err, buf.String())
	}
	wantDeleted := []string{
		"incipit/releases/v1.3.0-alpha.2/SHA256SUMS",
		"incipit/releases/v1.3.0-alpha.2/app-1.3.0-alpha.2.dmg",
		"incipit/releases/v1.1.0/SHA256SUMS",
		"incipit/releases/v1.1.0/app-1.1.0.dmg",
	}
	if !slices.Equal(deleted, wantDeleted) {
		t.Fatalf("deleted keys:\ngot:  %v\nwant: %v\nlog:\n%s", deleted, wantDeleted, buf.String())
	}
	for _, mustStay := range []string{
		"incipit/install.sh",
		"incipit/prerelease/history-darwin-arm64.json",
		"incipit/releases/v1.0.0/app-1.0.0.dmg",
		"incipit/releases/v1.2.0/app-1.2.0.dmg",
		"incipit/releases/v1.3.0-alpha.1/app-1.3.0-alpha.1.tar.zst",
		"incipit/releases/v1.3.0-alpha.3/app-1.3.0-alpha.3.dmg",
		"incipit/releases/v1.3.0-alpha.4/app-1.3.0-alpha.4.dmg",
	} {
		if _, ok := objects[mustStay]; !ok {
			t.Fatalf("expected %s to remain protected", mustStay)
		}
	}
}
