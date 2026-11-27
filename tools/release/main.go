// Command release prepares multi-platform draft release bundles and deploys
// published GitHub releases cleanly to Cloudflare R2 and the manifest edge server.
//
// Usage:
//
//	go tool release prepare-draft -in <artifacts-dir> -out <stage-dir>
//	go tool release deploy -dir <assets-dir> [-version <tag>] [-dry-run]
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	"strings"
	"time"
)

const checksumsName = "SHA256SUMS"

var checksumLine = regexp.MustCompile(`^([0-9a-fA-F]{64}) [ *](.+)$`)

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		log.Fatal("usage: release <prepare-draft|deploy> [flags]")
	}
	var err error
	switch os.Args[1] {
	case "prepare-draft":
		err = runPrepareDraft(os.Args[2:])
	case "deploy":
		err = runDeploy(os.Args[2:])
	default:
		err = fmt.Errorf("unknown subcommand %q (want prepare-draft or deploy)", os.Args[1])
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
	deliverables := map[string]string{}

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
		sum, err := fileSHA256Hex(path)
		if err != nil {
			return err
		}
		if prev, ok := seenSums[name]; ok {
			if prev != sum {
				return fmt.Errorf("conflicting %s across targets (%s != %s)", name, prev[:12], sum[:12])
			}
			return nil
		}
		seenSums[name] = sum
		dst := filepath.Join(outDir, name)
		if err := copyFile(path, dst, 0o644); err != nil {
			return err
		}
		if isChecksumTarget(name) {
			deliverables[name] = sum
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(deliverables) == 0 {
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
			got, ok := deliverables[name]
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
	return os.WriteFile(filepath.Join(outDir, checksumsName), formatChecksums(deliverables), 0o644)
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

func isMutableR2File(name string) bool {
	return name == checksumsName || name == "install.sh" || name == "install.ps1"
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
	Version string `json:"version"`
	Target  string `json:"target"`
	Channel string `json:"channel"`
}

func runDeploy(args []string) error {
	flags := flag.NewFlagSet("deploy", flag.ContinueOnError)
	dir := flags.String("dir", "", "directory of downloaded GitHub release assets")
	version := flags.String("version", "", "expected release version or tag (leading v optional)")
	dryRun := flags.Bool("dry-run", false, "verify checksums and print upload plan without performing network calls")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *dir == "" {
		return errors.New("deploy requires -dir")
	}
	cfg := deployConfig{
		Dir:            *dir,
		ExpectedVer:    strings.TrimPrefix(strings.TrimSpace(*version), "v"),
		DryRun:         *dryRun,
		R2Endpoint:     strings.TrimSpace(os.Getenv("R2_ENDPOINT")),
		R2Bucket:       strings.TrimSpace(os.Getenv("R2_BUCKET")),
		R2Prefix:       strings.Trim(strings.TrimSpace(os.Getenv("R2_PREFIX")), "/"),
		R2Region:       firstNonEmpty(os.Getenv("R2_REGION"), os.Getenv("AWS_REGION"), "auto"),
		R2AccessKey:    strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_ID")),
		R2SecretKey:    strings.TrimSpace(os.Getenv("R2_SECRET_ACCESS_KEY")),
		ManifestURL:    strings.TrimSuffix(strings.TrimSpace(os.Getenv("MANIFEST_DEPLOY_URL")), "/"),
		ManifestToken:  strings.TrimSpace(os.Getenv("MANIFEST_DEPLOY_TOKEN")),
		HTTPClient:     &http.Client{Timeout: 10 * time.Minute},
		Now:            time.Now,
	}
	return deploy(cfg, os.Stdout)
}

type deployConfig struct {
	Dir           string
	ExpectedVer   string
	DryRun        bool
	R2Endpoint    string
	R2Bucket      string
	R2Prefix      string
	R2Region      string
	R2AccessKey   string
	R2SecretKey   string
	ManifestURL   string
	ManifestToken string
	HTTPClient    *http.Client
	Now           func() time.Time
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

	var immutableFiles, mutableRootFiles, historyFiles, manifestFiles []string
	channels := map[string]string{}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		full := filepath.Join(cfg.Dir, name)
		switch {
		case name == checksumsName:
			mutableRootFiles = append(mutableRootFiles, name)
		case strings.HasPrefix(name, "update-") && strings.HasSuffix(name, ".json"):
			meta, err := readManifestMeta(full, cfg.ExpectedVer)
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			channels[name] = meta.Channel
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
			if isMutableR2File(name) {
				mutableRootFiles = append(mutableRootFiles, name)
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
	channel := commonChannel(channels)
	if channel == "" {
		return errors.New("manifests span multiple or empty channels")
	}

	slices.Sort(immutableFiles)
	slices.Sort(mutableRootFiles)
	slices.Sort(historyFiles)
	slices.Sort(manifestFiles)

	var r2 *r2Client
	if !cfg.DryRun {
		r2, err = newR2Client(cfg)
		if err != nil {
			return err
		}
		if cfg.ManifestURL != "" && cfg.ManifestToken == "" {
			return errors.New("MANIFEST_DEPLOY_TOKEN is required when MANIFEST_DEPLOY_URL is set")
		}
	}

	uploadR2 := func(key, fileName, cacheControl string) error {
		fmt.Fprintf(out, "r2 put %s (%s)\n", key, cacheControl)
		if cfg.DryRun {
			return nil
		}
		return r2.put(key, filepath.Join(cfg.Dir, fileName), contentTypeFor(fileName), cacheControl)
	}

	// Upload order is strict: immutable archives/deltas -> mutable metadata & history -> manifests last.
	for _, name := range immutableFiles {
		if err := uploadR2(r2Key(cfg.R2Prefix, name), name, "public, max-age=31536000, immutable"); err != nil {
			return err
		}
	}
	for _, name := range mutableRootFiles {
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
		if cfg.ManifestURL != "" {
			targetURL := cfg.ManifestURL + "/" + url.PathEscape(channel) + "/" + url.PathEscape(name)
			fmt.Fprintf(out, "worker put %s\n", targetURL)
			if !cfg.DryRun {
				if err := putWorkerManifest(cfg.HTTPClient, targetURL, cfg.ManifestToken, filepath.Join(cfg.Dir, name)); err != nil {
					return err
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

func commonChannel(channels map[string]string) string {
	var ch string
	for _, c := range channels {
		if c == "" {
			return ""
		}
		if ch == "" {
			ch = c
		} else if ch != c {
			return ""
		}
	}
	return ch
}

func readManifestMeta(path, wantVer string) (manifestMeta, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return manifestMeta{}, err
	}
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

func putWorkerManifest(client *http.Client, targetURL, token, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, targetURL, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("PUT %s: %s: %s", targetURL, resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
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
		"",
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
