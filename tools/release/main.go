// Command release prepares multi-platform draft release bundles, deploys
// published GitHub releases to Cloudflare R2 and Workers KV, and prunes
// superseded version directories from R2 while keeping fallback/active builds intact.
//
// Usage:
//
//	go tool release prepare-draft -in <artifacts-dir> -out <stage-dir>
//	go tool release deploy -dir <assets-dir> [-version <tag>] [-dry-run] [-require-kv]
//	go tool release prune [-keep-release 5] [-keep-prerelease 4] [-dry-run] [-require-kv]
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	checksumsName           = "SHA256SUMS"
	defaultCloudflareAPIBase = "https://api.cloudflare.com/client/v4"
	emptyPayloadSHA256      = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

var checksumLine = regexp.MustCompile(`^([0-9a-fA-F]{64}) [ *](.+)$`)

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		log.Fatal("usage: release <prepare-draft|deploy|prune> [flags]")
	}
	var err error
	switch os.Args[1] {
	case "prepare-draft":
		err = runPrepareDraft(os.Args[2:])
	case "deploy":
		err = runDeploy(os.Args[2:])
	case "prune":
		err = runPrune(os.Args[2:])
	default:
		err = fmt.Errorf("unknown subcommand %q (want prepare-draft, deploy, or prune)", os.Args[1])
	}
	if err != nil {
		log.Fatal(err)
	}
}

func runPrepareDraft(args []string) error {
	flags := flag.NewFlagSet("prepare-draft", flag.ContinueOnError)
	inDir := flags.String("in", "", "directory containing per-target artifact subdirectories")
	outDir := flags.String("out", "", "flat staging directory for GitHub release upload")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *inDir == "" || *outDir == "" {
		return errors.New("prepare-draft requires -in and -out")
	}
	return prepareDraft(*inDir, *outDir)
}

func prepareDraft(inDir, outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	seenSums := map[string]string{}
	origDeliverables := map[string]string{}
	stagedDeliverables := map[string]string{}

	err := filepath.WalkDir(inDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasSuffix(d.Name(), ".app") {
				return fs.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !isReleaseAsset(name) {
			return nil
		}
		stageName := stagedAssetName(name)
		// GitHub Releases rewrites characters outside [A-Za-z0-9._-] (e.g. ~, +)
		// to '.'. Signed manifests and install scripts embed exact URLs for archives,
		// deltas, and installers, so reject unrewritable non-package names early while
		// normalizing .deb/.rpm filenames before hashing into SHA256SUMS.
		if stageName != name && !isLinuxPackage(name) {
			return fmt.Errorf("asset %q contains characters GitHub Releases rewrites (outside [A-Za-z0-9._-])", name)
		}
		sum, err := fileSHA256Hex(path)
		if err != nil {
			return err
		}
		if prev, ok := seenSums[stageName]; ok {
			if prev != sum {
				return fmt.Errorf("conflicting %s across targets (%s != %s)", stageName, prev[:12], sum[:12])
			}
			if isChecksumTarget(name) {
				origDeliverables[name] = sum
			}
			return nil
		}
		seenSums[stageName] = sum
		dst := filepath.Join(outDir, stageName)
		if err := copyFile(path, dst, 0o644); err != nil {
			return err
		}
		if isChecksumTarget(name) {
			origDeliverables[name] = sum
			stagedDeliverables[stageName] = sum
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(stagedDeliverables) == 0 {
		return fmt.Errorf("no release deliverables found under %s", inDir)
	}
	// Verify every incoming SHA256SUMS entry against what we staged so subtle
	// partial-copy corruption fails before creating the draft release.
	err = filepath.WalkDir(inDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != checksumsName {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for name, want := range parseChecksums(raw) {
			got, ok := origDeliverables[name]
			if !ok {
				return fmt.Errorf("%s lists %q which was not staged", path, name)
			}
			if !strings.EqualFold(got, want) {
				return fmt.Errorf("%s checksum mismatch for %s: got %s want %s", path, name, got, want)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, checksumsName), formatChecksums(stagedDeliverables), 0o644)
}

func isLinuxPackage(name string) bool {
	return strings.HasSuffix(name, ".deb") || strings.HasSuffix(name, ".rpm")
}

// stagedAssetName maps filenames containing characters GitHub Releases rewrites
// (such as '~' in prerelease Debian/RPM versions) to their GitHub-safe form so
// merged SHA256SUMS matches `gh release download` verbatim during deploy.
func stagedAssetName(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for i := 0; i < len(name); i++ {
		ch := name[i]
		if ('A' <= ch && ch <= 'Z') || ('a' <= ch && ch <= 'z') || ('0' <= ch && ch <= '9') || ch == '.' || ch == '_' || ch == '-' {
			b.WriteByte(ch)
		} else {
			b.WriteByte('.')
		}
	}
	return b.String()
}

func isReleaseAsset(name string) bool {
	if isChecksumTarget(name) {
		return true
	}
	return (strings.HasPrefix(name, "update-") || strings.HasPrefix(name, "history-")) && strings.HasSuffix(name, ".json")
}

func isChecksumTarget(name string) bool {
	switch name {
	case "install.sh", "install.ps1":
		return true
	}
	switch {
	case strings.HasSuffix(name, ".dmg"),
		strings.HasSuffix(name, ".exe") && strings.Contains(name, "-setup-"),
		strings.HasSuffix(name, ".deb"),
		strings.HasSuffix(name, ".rpm"),
		strings.HasSuffix(name, ".tar.gz"),
		strings.HasSuffix(name, ".tar.zst"),
		strings.HasSuffix(name, ".delta"):
		return true
	}
	return false
}

func isRootScriptFile(name string) bool {
	return name == "install.sh" || name == "install.ps1"
}

type signedManifestEnvelope struct {
	V          int    `json:"v"`
	Payload    []byte `json:"payload"`
	Signatures []struct {
		KeyID string `json:"kid"`
		Sig   string `json:"sig"`
	} `json:"signatures"`
}

type manifestMeta struct {
	Version  string `json:"version"`
	Target   string `json:"target"`
	Channel  string `json:"channel"`
	Artifact struct {
		URL string `json:"url"`
	} `json:"artifact"`
}

func loadCloudConfig(dryRun, requireKV bool) deployConfig {
	return deployConfig{
		DryRun:          dryRun,
		RequireKV:       requireKV,
		R2Endpoint:      strings.TrimSpace(os.Getenv("R2_ENDPOINT")),
		R2Bucket:        strings.TrimSpace(os.Getenv("R2_BUCKET")),
		R2Prefix:        strings.Trim(strings.TrimSpace(os.Getenv("R2_PREFIX")), "/"),
		R2Region:        firstNonEmpty(os.Getenv("R2_REGION"), os.Getenv("AWS_REGION"), "auto"),
		R2AccessKey:     strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_ID")),
		R2SecretKey:     strings.TrimSpace(os.Getenv("R2_SECRET_ACCESS_KEY")),
		CFAccountID:     strings.TrimSpace(os.Getenv("CLOUDFLARE_ACCOUNT_ID")),
		CFKVNamespaceID: strings.TrimSpace(os.Getenv("CLOUDFLARE_KV_NAMESPACE_ID")),
		CFAPIToken:      strings.TrimSpace(os.Getenv("CLOUDFLARE_API_TOKEN")),
		CFAPIBase:       strings.TrimSuffix(strings.TrimSpace(os.Getenv("CLOUDFLARE_API_BASE")), "/"),
		HTTPClient:      &http.Client{Timeout: 10 * time.Minute},
		Now:             time.Now,
	}
}

func validateKVConfig(cfg deployConfig) (bool, error) {
	hasAnyKV := cfg.CFAccountID != "" || cfg.CFKVNamespaceID != "" || cfg.CFAPIToken != ""
	if cfg.RequireKV || hasAnyKV {
		if cfg.CFAccountID == "" || cfg.CFKVNamespaceID == "" || cfg.CFAPIToken == "" {
			return false, errors.New("Cloudflare KV requires CLOUDFLARE_ACCOUNT_ID, CLOUDFLARE_KV_NAMESPACE_ID, and CLOUDFLARE_API_TOKEN")
		}
		return true, nil
	}
	return false, nil
}

func runDeploy(args []string) error {
	flags := flag.NewFlagSet("deploy", flag.ContinueOnError)
	dir := flags.String("dir", "", "directory of downloaded GitHub release assets")
	version := flags.String("version", "", "expected release version or tag (leading v optional)")
	dryRun := flags.Bool("dry-run", false, "verify checksums and print upload plan without performing network calls")
	requireKV := flags.Bool("require-kv", false, "require Cloudflare Workers KV credentials instead of falling back to R2")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *dir == "" {
		return errors.New("deploy requires -dir")
	}
	cfg := loadCloudConfig(*dryRun, *requireKV)
	cfg.Dir = *dir
	cfg.ExpectedVer = strings.TrimPrefix(strings.TrimSpace(*version), "v")
	return deploy(cfg, os.Stdout)
}

func runPrune(args []string) error {
	flags := flag.NewFlagSet("prune", flag.ContinueOnError)
	dryRun := flags.Bool("dry-run", false, "print keep/delete plan without removing objects from R2")
	requireKV := flags.Bool("require-kv", false, "require Cloudflare Workers KV credentials to inspect live and locked manifests")
	keepRelease := flags.Int("keep-release", 5, "number of newest stable releases to retain on R2")
	keepPrerelease := flags.Int("keep-prerelease", 4, "number of newest prereleases to retain on R2")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg := loadCloudConfig(*dryRun, *requireKV)
	cfg.KeepRelease = *keepRelease
	cfg.KeepPrerelease = *keepPrerelease
	return prune(cfg, os.Stdout)
}

type deployConfig struct {
	Dir             string
	ExpectedVer     string
	DryRun          bool
	RequireKV       bool
	KeepRelease     int
	KeepPrerelease  int
	R2Endpoint      string
	R2Bucket        string
	R2Prefix        string
	R2Region        string
	R2AccessKey     string
	R2SecretKey     string
	CFAccountID     string
	CFKVNamespaceID string
	CFAPIToken      string
	CFAPIBase       string
	HTTPClient      *http.Client
	Now             func() time.Time
}

func deploy(cfg deployConfig, out io.Writer) error {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 10 * time.Minute}
	}
	entries, err := os.ReadDir(cfg.Dir)
	if err != nil {
		return err
	}
	sumPath := filepath.Join(cfg.Dir, checksumsName)
	rawSums, err := os.ReadFile(sumPath)
	if err != nil {
		return fmt.Errorf("missing %s: %w", checksumsName, err)
	}
	wantSums := parseChecksums(rawSums)
	if len(wantSums) == 0 {
		return fmt.Errorf("%s contains no entries", checksumsName)
	}

	var immutableFiles, versionMetaFiles, rootScriptFiles, historyFiles, manifestFiles []string
	channels := map[string]string{}
	versionDirs := map[string]string{}
	metas := map[string]manifestMeta{}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		full := filepath.Join(cfg.Dir, name)
		switch {
		case name == checksumsName:
			versionMetaFiles = append(versionMetaFiles, name)
		case strings.HasPrefix(name, "update-") && strings.HasSuffix(name, ".json"):
			meta, err := readManifestMeta(full, cfg.ExpectedVer)
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			vdir, err := artifactPrefixFromURL(meta.Artifact.URL, cfg.R2Prefix)
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			channels[name] = meta.Channel
			versionDirs[name] = vdir
			metas[name] = meta
			manifestFiles = append(manifestFiles, name)
		case strings.HasPrefix(name, "history-") && strings.HasSuffix(name, ".json"):
			historyFiles = append(historyFiles, name)
		case isChecksumTarget(name):
			want, ok := wantSums[name]
			if !ok {
				return fmt.Errorf("%s is missing checksum entry in %s", name, checksumsName)
			}
			got, err := fileSHA256Hex(full)
			if err != nil {
				return err
			}
			if !strings.EqualFold(got, want) {
				return fmt.Errorf("checksum mismatch for %s: got %s want %s", name, got, want)
			}
			if isRootScriptFile(name) {
				rootScriptFiles = append(rootScriptFiles, name)
			} else {
				immutableFiles = append(immutableFiles, name)
			}
		}
	}
	for name := range wantSums {
		if _, err := os.Stat(filepath.Join(cfg.Dir, name)); err != nil {
			return fmt.Errorf("%s lists %s which is missing from %s", checksumsName, name, cfg.Dir)
		}
	}
	if len(manifestFiles) == 0 {
		return fmt.Errorf("no update-*.json manifests found in %s", cfg.Dir)
	}
	channel := commonString(channels)
	if channel == "" {
		return errors.New("manifests span multiple or empty channels")
	}
	versionDir, ok := uniformString(versionDirs)
	if !ok {
		return errors.New("manifests span multiple version asset directories")
	}

	slices.Sort(immutableFiles)
	slices.Sort(versionMetaFiles)
	slices.Sort(rootScriptFiles)
	slices.Sort(historyFiles)
	slices.Sort(manifestFiles)

	var r2 *r2Client
	kvEnabled := cfg.DryRun && (cfg.RequireKV || cfg.CFAccountID != "" || cfg.CFKVNamespaceID != "" || cfg.CFAPIToken != "")
	if !cfg.DryRun {
		r2, err = newR2Client(cfg)
		if err != nil {
			return err
		}
		kvEnabled, err = validateKVConfig(cfg)
		if err != nil {
			return err
		}
	}
	apiBase := firstNonEmpty(cfg.CFAPIBase, defaultCloudflareAPIBase)

	uploadR2 := func(key, fileName, cacheControl string) error {
		fmt.Fprintf(out, "r2 put %s (%s)\n", key, cacheControl)
		if cfg.DryRun {
			return nil
		}
		return r2.put(key, filepath.Join(cfg.Dir, fileName), contentTypeFor(fileName), cacheControl)
	}

	// Upload order is strict: immutable archives/deltas -> version checksums -> root entrypoints & history -> manifests last.
	for _, name := range immutableFiles {
		if err := uploadR2(r2Key(cfg.R2Prefix, joinRelPath(versionDir, name)), name, "public, max-age=31536000, immutable"); err != nil {
			return err
		}
	}
	for _, name := range versionMetaFiles {
		if err := uploadR2(r2Key(cfg.R2Prefix, joinRelPath(versionDir, name)), name, "no-cache"); err != nil {
			return err
		}
	}
	for _, name := range rootScriptFiles {
		if err := uploadR2(r2Key(cfg.R2Prefix, name), name, "no-cache"); err != nil {
			return err
		}
	}
	for _, name := range historyFiles {
		if err := uploadR2(r2Key(cfg.R2Prefix, channel+"/"+name), name, "no-cache"); err != nil {
			return err
		}
	}
	for _, name := range manifestFiles {
		if kvEnabled {
			meta := metas[name]
			keys := []string{
				"manifest:" + meta.Channel + ":" + meta.Target + ":" + meta.Version,
				"latest:" + meta.Channel + ":" + meta.Target,
			}
			for _, key := range keys {
				fmt.Fprintf(out, "kv put %s\n", key)
				if !cfg.DryRun {
					if err := putCloudflareKV(cfg.HTTPClient, apiBase, cfg.CFAccountID, cfg.CFKVNamespaceID, cfg.CFAPIToken, key, filepath.Join(cfg.Dir, name)); err != nil {
						return err
					}
				}
			}
			continue
		}
		if err := uploadR2(r2Key(cfg.R2Prefix, channel+"/"+name), name, "no-cache"); err != nil {
			return err
		}
	}
	return nil
}

func commonString(vals map[string]string) string {
	v, ok := uniformString(vals)
	if !ok {
		return ""
	}
	return v
}

func uniformString(vals map[string]string) (string, bool) {
	var first string
	seen := false
	for _, v := range vals {
		if !seen {
			first = v
			seen = true
		} else if first != v {
			return "", false
		}
	}
	return first, seen
}

func artifactPrefixFromURL(rawURL, r2Prefix string) (string, error) {
	if rawURL == "" {
		return "", nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid artifact.url %q: %w", rawURL, err)
	}
	p := strings.Trim(u.Path, "/")
	i := strings.LastIndexByte(p, '/')
	if i <= 0 {
		return "", nil
	}
	dir := p[:i]
	if r2Prefix != "" {
		dir = strings.TrimPrefix(dir, r2Prefix+"/")
		if dir == r2Prefix {
			dir = ""
		}
	}
	return dir, nil
}

func joinRelPath(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}

func readManifestMeta(path, wantVer string) (manifestMeta, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return manifestMeta{}, err
	}
	return parseManifestMeta(raw, wantVer)
}

func parseManifestMeta(raw []byte, wantVer string) (manifestMeta, error) {
	var env signedManifestEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return manifestMeta{}, err
	}
	if env.V == 0 || len(env.Payload) == 0 || len(env.Signatures) == 0 {
		return manifestMeta{}, errors.New("manifest is not a valid signed envelope (missing v, payload, or signatures)")
	}
	var m manifestMeta
	if err := json.Unmarshal(env.Payload, &m); err != nil {
		return manifestMeta{}, fmt.Errorf("decoding signed manifest payload: %w", err)
	}
	if m.Version == "" || m.Target == "" || m.Channel == "" {
		return manifestMeta{}, errors.New("manifest missing version, target, or channel")
	}
	if wantVer != "" && m.Version != wantVer {
		return manifestMeta{}, fmt.Errorf("manifest version %q != expected %q", m.Version, wantVer)
	}
	return m, nil
}

type cfAPIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func kvNamespaceURL(apiBase, accountID, namespaceID string) string {
	return strings.TrimSuffix(apiBase, "/") +
		"/accounts/" + url.PathEscape(accountID) +
		"/storage/kv/namespaces/" + url.PathEscape(namespaceID)
}

func putCloudflareKV(client *http.Client, apiBase, accountID, namespaceID, token, key, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	targetURL := kvNamespaceURL(apiBase, accountID, namespaceID) + "/values/" + url.PathEscape(key)
	req, err := http.NewRequest(http.MethodPut, targetURL, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("KV PUT %s: %s: %s", key, resp.Status, strings.TrimSpace(string(body)))
	}
	var apiResp struct {
		Success bool         `json:"success"`
		Errors  []cfAPIError `json:"errors"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("KV PUT %s: invalid response %q: %w", key, strings.TrimSpace(string(body)), err)
	}
	if !apiResp.Success {
		return fmt.Errorf("KV PUT %s failed: %+v", key, apiResp.Errors)
	}
	return nil
}

func listCloudflareKVKeys(client *http.Client, apiBase, accountID, namespaceID, token, prefix string) ([]string, error) {
	var keys []string
	cursor := ""
	base := kvNamespaceURL(apiBase, accountID, namespaceID) + "/keys"
	for {
		q := url.Values{}
		if prefix != "" {
			q.Set("prefix", prefix)
		}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		u := base
		if len(q) > 0 {
			u += "?" + q.Encode()
		}
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("KV LIST %q: %s: %s", prefix, resp.Status, strings.TrimSpace(string(body)))
		}
		var out struct {
			Success bool         `json:"success"`
			Errors  []cfAPIError `json:"errors"`
			Result  []struct {
				Name string `json:"name"`
			} `json:"result"`
			ResultInfo struct {
				Cursor string `json:"cursor"`
			} `json:"result_info"`
		}
		if err := json.Unmarshal(body, &out); err != nil {
			return nil, fmt.Errorf("KV LIST %q: invalid JSON: %w", prefix, err)
		}
		if !out.Success {
			return nil, fmt.Errorf("KV LIST %q failed: %+v", prefix, out.Errors)
		}
		for _, entry := range out.Result {
			keys = append(keys, entry.Name)
		}
		if out.ResultInfo.Cursor == "" {
			break
		}
		cursor = out.ResultInfo.Cursor
	}
	return keys, nil
}

func getCloudflareKVValue(client *http.Client, apiBase, accountID, namespaceID, token, key string) ([]byte, bool, error) {
	targetURL := kvNamespaceURL(apiBase, accountID, namespaceID) + "/values/" + url.PathEscape(key)
	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, false, err
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	resp.Body.Close()
	if readErr != nil {
		return nil, false, readErr
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, false, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false, fmt.Errorf("KV GET %s: %s: %s", key, resp.Status, strings.TrimSpace(string(body)))
	}
	return body, true, nil
}

func prune(cfg deployConfig, out io.Writer) error {
	if cfg.KeepRelease < 0 || cfg.KeepPrerelease < 0 {
		return errors.New("keep-release and keep-prerelease must be non-negative")
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 10 * time.Minute}
	}
	r2, err := newR2Client(cfg)
	if err != nil {
		return err
	}
	kvEnabled, err := validateKVConfig(cfg)
	if err != nil {
		return err
	}
	apiBase := firstNonEmpty(cfg.CFAPIBase, defaultCloudflareAPIBase)

	reasons := map[string][]string{}
	addReason := func(ver, reason string) {
		if ver == "" {
			return
		}
		if !slices.Contains(reasons[ver], reason) {
			reasons[ver] = append(reasons[ver], reason)
		}
	}

	if kvEnabled {
		for _, prefix := range []string{"latest:", "lock:"} {
			keys, err := listCloudflareKVKeys(cfg.HTTPClient, apiBase, cfg.CFAccountID, cfg.CFKVNamespaceID, cfg.CFAPIToken, prefix)
			if err != nil {
				return err
			}
			for _, key := range keys {
				val, ok, err := getCloudflareKVValue(cfg.HTTPClient, apiBase, cfg.CFAccountID, cfg.CFKVNamespaceID, cfg.CFAPIToken, key)
				if err != nil {
					return err
				}
				if !ok {
					continue
				}
				if strings.HasPrefix(key, "latest:") {
					meta, err := parseManifestMeta(val, "")
					if err != nil {
						return fmt.Errorf("KV %s: %w", key, err)
					}
					addReason(meta.Version, "kv:"+key)
				} else {
					addReason(strings.TrimSpace(string(val)), "kv:"+key)
				}
			}
		}
	}

	for _, ch := range []string{"release", "prerelease"} {
		histPrefix := r2Key(cfg.R2Prefix, ch+"/history-")
		histKeys, err := r2.list(histPrefix)
		if err != nil {
			return err
		}
		for _, key := range histKeys {
			if !strings.HasSuffix(key, ".json") {
				continue
			}
			raw, status, err := r2.get(key)
			if err != nil {
				return err
			}
			if status == http.StatusNotFound {
				continue
			}
			var idx struct {
				History []struct {
					Version string `json:"version"`
				} `json:"history"`
			}
			if err := json.Unmarshal(raw, &idx); err != nil {
				return fmt.Errorf("R2 %s: invalid history JSON: %w", key, err)
			}
			for _, item := range idx.History {
				addReason(item.Version, "history:"+ch)
			}
		}
	}

	relPrefix := r2Key(cfg.R2Prefix, "releases/")
	relKeys, err := r2.list(relPrefix)
	if err != nil {
		return err
	}
	byVer := map[string][]string{}
	for _, key := range relKeys {
		rest := strings.TrimPrefix(key, relPrefix)
		if !strings.HasPrefix(rest, "v") {
			continue
		}
		slash := strings.IndexByte(rest, '/')
		if slash <= 1 {
			continue
		}
		ver := rest[1:slash]
		byVer[ver] = append(byVer[ver], key)
	}
	if len(byVer) == 0 {
		fmt.Fprintln(out, "no versioned releases found under releases/")
		return nil
	}

	var stable, pre []string
	for ver := range byVer {
		if isPrerelease(ver) {
			pre = append(pre, ver)
		} else {
			stable = append(stable, ver)
		}
	}
	sortDesc := func(vs []string) {
		slices.SortFunc(vs, func(a, b string) int { return compareVersions(b, a) })
	}
	sortDesc(stable)
	sortDesc(pre)

	for i, ver := range stable {
		if i < cfg.KeepRelease {
			addReason(ver, "keep-release")
		}
	}
	for i, ver := range pre {
		if i < cfg.KeepPrerelease {
			addReason(ver, "keep-prerelease")
		}
	}

	var allVers []string
	for ver := range byVer {
		allVers = append(allVers, ver)
	}
	sortDesc(allVers)

	prunedVersions := 0
	prunedObjects := 0
	for _, ver := range allVers {
		rs := reasons[ver]
		keys := byVer[ver]
		slices.Sort(keys)
		if len(rs) > 0 {
			fmt.Fprintf(out, "keep v%s (%d files; %s)\n", ver, len(keys), strings.Join(rs, ", "))
			continue
		}
		prunedVersions++
		fmt.Fprintf(out, "prune v%s (%d files)\n", ver, len(keys))
		for _, key := range keys {
			if cfg.DryRun {
				fmt.Fprintf(out, "r2 delete %s (dry-run)\n", key)
				continue
			}
			fmt.Fprintf(out, "r2 delete %s\n", key)
			if err := r2.delete(key); err != nil {
				return err
			}
			prunedObjects++
		}
	}
	if cfg.DryRun {
		fmt.Fprintf(out, "prune dry-run complete: %d candidate version(s)\n", prunedVersions)
	} else {
		fmt.Fprintf(out, "prune complete: deleted %d file(s) across %d version(s)\n", prunedObjects, prunedVersions)
	}
	return nil
}

func isPrerelease(v string) bool {
	v = strings.TrimPrefix(v, "v")
	v, _, _ = strings.Cut(v, "+")
	_, pre, ok := strings.Cut(v, "-")
	return ok && pre != ""
}

func compareVersions(a, b string) int {
	a, b = strings.TrimPrefix(a, "v"), strings.TrimPrefix(b, "v")
	a, _, _ = strings.Cut(a, "+")
	b, _, _ = strings.Cut(b, "+")
	ar, apre, _ := strings.Cut(a, "-")
	br, bpre, _ := strings.Cut(b, "-")
	if c := compareVersionFields(ar, br); c != 0 {
		return c
	}
	switch {
	case apre == bpre:
		return 0
	case apre == "":
		return 1
	case bpre == "":
		return -1
	}
	return compareVersionFields(apre, bpre)
}

func compareVersionFields(a, b string) int {
	af, bf := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(af) || i < len(bf); i++ {
		var x, y string
		if i < len(af) {
			x = af[i]
		}
		if i < len(bf) {
			y = bf[i]
		}
		xn, xerr := strconv.Atoi(x)
		yn, yerr := strconv.Atoi(y)
		switch {
		case x == y:
			continue
		case x == "":
			return -1
		case y == "":
			return 1
		case xerr == nil && yerr == nil:
			if xn < yn {
				return -1
			}
			if xn > yn {
				return 1
			}
		case xerr == nil:
			return -1
		case yerr == nil:
			return 1
		default:
			return strings.Compare(x, y)
		}
	}
	return 0
}

type r2Client struct {
	endpoint  *url.URL
	bucket    string
	region    string
	accessKey string
	secretKey string
	client    *http.Client
	now       func() time.Time
}

func newR2Client(cfg deployConfig) (*r2Client, error) {
	if cfg.R2Endpoint == "" || cfg.R2Bucket == "" || cfg.R2AccessKey == "" || cfg.R2SecretKey == "" {
		return nil, errors.New("R2 deployment requires R2_ENDPOINT, R2_BUCKET, R2_ACCESS_KEY_ID, and R2_SECRET_ACCESS_KEY")
	}
	u, err := url.Parse(cfg.R2Endpoint)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return nil, fmt.Errorf("invalid R2_ENDPOINT %q", cfg.R2Endpoint)
	}
	nowFn := cfg.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	cl := cfg.HTTPClient
	if cl == nil {
		cl = &http.Client{Timeout: 10 * time.Minute}
	}
	return &r2Client{
		endpoint:  u,
		bucket:    cfg.R2Bucket,
		region:    cfg.R2Region,
		accessKey: cfg.R2AccessKey,
		secretKey: cfg.R2SecretKey,
		client:    cl,
		now:       nowFn,
	}, nil
}

func (c *r2Client) put(key, path, contentType, cacheControl string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sum := sha256.New()
	size, err := io.Copy(sum, f)
	if err != nil {
		return err
	}
	var body io.Reader = http.NoBody
	if size > 0 {
		body = io.NewSectionReader(f, 0, size)
	}
	escaped := "/" + url.PathEscape(c.bucket) + "/" + escapeS3Key(key)
	target := c.endpoint.Scheme + "://" + c.endpoint.Host + escaped
	req, err := http.NewRequest(http.MethodPut, target, body)
	if err != nil {
		return err
	}
	req.ContentLength = size
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Cache-Control", cacheControl)
	c.sign(req, hex.EncodeToString(sum.Sum(nil)), c.now())

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("R2 PUT %s: %s: %s", key, resp.Status, strings.TrimSpace(string(msg)))
	}
	return nil
}

func (c *r2Client) list(prefix string) ([]string, error) {
	var keys []string
	token := ""
	for {
		q := url.Values{"list-type": {"2"}}
		if prefix != "" {
			q.Set("prefix", prefix)
		}
		if token != "" {
			q.Set("continuation-token", token)
		}
		target := c.endpoint.Scheme + "://" + c.endpoint.Host + "/" + url.PathEscape(c.bucket) + "?" + encodeS3Query(q)
		req, err := http.NewRequest(http.MethodGet, target, nil)
		if err != nil {
			return nil, err
		}
		c.sign(req, emptyPayloadSHA256, c.now())
		resp, err := c.client.Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("R2 LIST %q: %s: %s", prefix, resp.Status, strings.TrimSpace(string(body)))
		}
		var out struct {
			IsTruncated           bool   `xml:"IsTruncated"`
			NextContinuationToken string `xml:"NextContinuationToken"`
			Contents              []struct {
				Key string `xml:"Key"`
			} `xml:"Contents"`
		}
		if err := xml.Unmarshal(body, &out); err != nil {
			return nil, fmt.Errorf("R2 LIST %q: invalid XML: %w", prefix, err)
		}
		for _, item := range out.Contents {
			keys = append(keys, item.Key)
		}
		if !out.IsTruncated || out.NextContinuationToken == "" {
			break
		}
		token = out.NextContinuationToken
	}
	return keys, nil
}

func (c *r2Client) get(key string) ([]byte, int, error) {
	escaped := "/" + url.PathEscape(c.bucket) + "/" + escapeS3Key(key)
	target := c.endpoint.Scheme + "://" + c.endpoint.Host + escaped
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return nil, 0, err
	}
	c.sign(req, emptyPayloadSHA256, c.now())
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	resp.Body.Close()
	if readErr != nil {
		return nil, resp.StatusCode, readErr
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, resp.StatusCode, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.StatusCode, fmt.Errorf("R2 GET %s: %s: %s", key, resp.Status, strings.TrimSpace(string(body)))
	}
	return body, resp.StatusCode, nil
}

func (c *r2Client) delete(key string) error {
	escaped := "/" + url.PathEscape(c.bucket) + "/" + escapeS3Key(key)
	target := c.endpoint.Scheme + "://" + c.endpoint.Host + escaped
	req, err := http.NewRequest(http.MethodDelete, target, nil)
	if err != nil {
		return err
	}
	c.sign(req, emptyPayloadSHA256, c.now())
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("R2 DELETE %s: %s: %s", key, resp.Status, strings.TrimSpace(string(msg)))
	}
	return nil
}

func (c *r2Client) sign(req *http.Request, payloadSHA256 string, at time.Time) {
	at = at.UTC()
	date := at.Format("20060102")
	amzDate := at.Format("20060102T150405Z")
	req.Header.Set("Host", req.URL.Host)
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadSHA256)

	canonicalURI := req.URL.EscapedPath()
	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n", req.URL.Host, payloadSHA256, amzDate)
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI,
		canonicalQuery(req.URL.RawQuery),
		canonicalHeaders,
		signedHeaders,
		payloadSHA256,
	}, "\n")
	scope := fmt.Sprintf("%s/%s/s3/aws4_request", date, c.region)
	hashReq := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		hex.EncodeToString(hashReq[:]),
	}, "\n")
	kDate := hmacSHA256([]byte("AWS4"+c.secretKey), date)
	kRegion := hmacSHA256(kDate, c.region)
	kService := hmacSHA256(kRegion, "s3")
	kSigning := hmacSHA256(kService, "aws4_request")
	sig := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))
	req.Header.Set("Authorization", fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s", c.accessKey, scope, signedHeaders, sig))
}

func canonicalQuery(raw string) string {
	if raw == "" {
		return ""
	}
	vals, err := url.ParseQuery(raw)
	if err != nil {
		return raw
	}
	return encodeS3Query(vals)
}

func encodeS3Query(vals url.Values) string {
	if len(vals) == 0 {
		return ""
	}
	keys := make([]string, 0, len(vals))
	for k := range vals {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	var parts []string
	for _, k := range keys {
		ek := encodeS3Component(k)
		vs := slices.Clone(vals[k])
		slices.Sort(vs)
		for _, v := range vs {
			parts = append(parts, ek+"="+encodeS3Component(v))
		}
	}
	return strings.Join(parts, "&")
}

func encodeS3Component(s string) string {
	const hexDigits = "0123456789ABCDEF"
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ('A' <= ch && ch <= 'Z') || ('a' <= ch && ch <= 'z') || ('0' <= ch && ch <= '9') ||
			ch == '-' || ch == '.' || ch == '_' || ch == '~' {
			b.WriteByte(ch)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hexDigits[ch>>4])
		b.WriteByte(hexDigits[ch&0x0F])
	}
	return b.String()
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func escapeS3Key(key string) string {
	parts := strings.Split(key, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

func r2Key(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "/" + name
}

func contentTypeFor(name string) string {
	switch filepath.Ext(name) {
	case ".dmg":
		return "application/x-apple-diskimage"
	case ".exe":
		return "application/vnd.microsoft.portable-executable"
	case ".deb":
		return "application/vnd.debian.binary-package"
	case ".rpm":
		return "application/x-rpm"
	case ".gz":
		return "application/gzip"
	case ".zst":
		return "application/zstd"
	case ".json":
		return "application/json"
	case ".sh", ".ps1":
		return "text/plain; charset=utf-8"
	default:
		if name == checksumsName {
			return "text/plain; charset=utf-8"
		}
		return "application/octet-stream"
	}
}

func fileSHA256Hex(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyFile(src, dst string, mode fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func parseChecksums(data []byte) map[string]string {
	sums := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		if m := checksumLine.FindStringSubmatch(strings.TrimSuffix(line, "\r")); m != nil {
			sums[m[2]] = strings.ToLower(m[1])
		}
	}
	return sums
}

func formatChecksums(sums map[string]string) []byte {
	var b strings.Builder
	for _, name := range slices.Sorted(maps.Keys(sums)) {
		fmt.Fprintf(&b, "%s  %s\n", sums[name], name)
	}
	return []byte(b.String())
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
