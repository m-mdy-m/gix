// Package selfupdate replaces the running gix binary with a published
// release.
//
// It uses the same files the installers do: the bare binary and its
// .sha256 on a GitHub release. The latest release is found through the
// /releases/latest redirect rather than the GitHub API, so there is no
// rate limit and no token.
//
// The new binary is downloaded next to the old one, checked against its
// checksum, and run once with --version before it replaces anything, so
// a bad download never leaves the user without a working gix.
package selfupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/m-mdy-m/gix/internal/flow"
)

// Repo is the GitHub repository releases come from.
const Repo = "m-mdy-m/gix"

// maxBinarySize bounds a download, so a wrong URL cannot fill the disk.
const maxBinarySize = 200 << 20

// Updater downloads and installs releases. The zero value is not usable;
// call New.
type Updater struct {
	// BaseURL is the repository's GitHub URL. Tests point it elsewhere.
	BaseURL string
	HTTP    *http.Client
	GOOS    string
	GOARCH  string
	// UserAgent is sent with every request.
	UserAgent string
}

// New returns an Updater for the platform gix is running on.
func New(userAgent string) *Updater {
	return &Updater{
		BaseURL:   "https://github.com/" + Repo,
		HTTP:      &http.Client{Timeout: 5 * time.Minute},
		GOOS:      runtime.GOOS,
		GOARCH:    runtime.GOARCH,
		UserAgent: userAgent,
	}
}

// AssetName is the release file for a platform: gix-linux-x64,
// gix-darwin-arm64, gix-windows-x64.exe.
func AssetName(goos, goarch string) (string, error) {
	switch goos {
	case "linux", "darwin", "windows":
	default:
		return "", fmt.Errorf("no prebuilt gix for %s", goos)
	}

	var arch string
	switch goarch {
	case "amd64":
		arch = "x64"
	case "arm64":
		arch = "arm64"
	default:
		return "", fmt.Errorf("no prebuilt gix for %s/%s", goos, goarch)
	}

	name := "gix-" + goos + "-" + arch
	if goos == "windows" {
		name += ".exe"
	}
	return name, nil
}

// LatestTag asks GitHub which release is the latest, such as "v1.1.1".
//
// /releases/latest redirects to /releases/tag/<tag>. The redirect is read
// rather than followed. A repository with no releases redirects to
// /releases, which is reported as such.
func (u *Updater) LatestTag(ctx context.Context) (string, error) {
	client := *u.HTTP
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	resp, err := u.do(ctx, &client, http.MethodHead, u.BaseURL+"/releases/latest")
	if err != nil {
		return "", fmt.Errorf("could not reach GitHub: %w", err)
	}
	resp.Body.Close()

	loc := resp.Header.Get("Location")
	if resp.StatusCode < 300 || resp.StatusCode > 399 || loc == "" {
		return "", fmt.Errorf("could not find the latest release (HTTP %d)", resp.StatusCode)
	}

	parsed, err := url.Parse(loc)
	if err != nil {
		return "", fmt.Errorf("unexpected redirect from GitHub: %q", loc)
	}
	tag := path.Base(strings.TrimRight(parsed.Path, "/"))
	if !looksLikeTag(tag) {
		return "", errors.New("no published release found")
	}
	return tag, nil
}

func looksLikeTag(tag string) bool {
	return len(tag) > 1 && tag[0] == 'v' && tag[1] >= '0' && tag[1] <= '9'
}

func (u *Updater) do(ctx context.Context, client *http.Client, method, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if u.UserAgent != "" {
		req.Header.Set("User-Agent", u.UserAgent)
	}
	return client.Do(req)
}

func (u *Updater) get(ctx context.Context, rawURL string) (*http.Response, error) {
	resp, err := u.do(ctx, u.HTTP, http.MethodGet, rawURL)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("download failed: HTTP %d for %s", resp.StatusCode, rawURL)
	}
	return resp, nil
}

// Download fetches the release binary for this platform into dir and
// returns its path. dir should be the directory of the binary being
// replaced, so the final rename stays on one filesystem.
//
// The file is checked against the release's .sha256 and run once with
// --version before it is returned. On any failure it is removed.
func (u *Updater) Download(ctx context.Context, tag, dir string) (string, error) {
	name, err := AssetName(u.GOOS, u.GOARCH)
	if err != nil {
		return "", err
	}
	releaseURL := u.BaseURL + "/releases/download/" + tag + "/"

	suffix := ""
	if u.GOOS == "windows" {
		suffix = ".exe" // Windows will not run a file without it
	}
	tmp, err := os.CreateTemp(dir, ".gix-update-*"+suffix)
	if err != nil {
		return "", fmt.Errorf("cannot write to %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	keep := false
	defer func() {
		if !keep {
			tmp.Close()
			os.Remove(tmpPath)
		}
	}()

	resp, err := u.get(ctx, releaseURL+name)
	if err != nil {
		return "", fmt.Errorf("%w (does %s have a build for this platform?)", err, tag)
	}
	defer resp.Body.Close()

	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(resp.Body, maxBinarySize+1))
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	if n > maxBinarySize {
		return "", errors.New("download failed: file is larger than expected")
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	// A release without a checksum is refused rather than trusted.
	want, err := u.checksum(ctx, releaseURL+name+".sha256")
	if err != nil {
		return "", err
	}
	if got := hex.EncodeToString(hash.Sum(nil)); !strings.EqualFold(got, want) {
		return "", fmt.Errorf("checksum mismatch for %s (corrupted download?)", name)
	}

	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return "", err
	}
	if err := checkRuns(ctx, tmpPath, strings.TrimPrefix(tag, "v")); err != nil {
		return "", err
	}

	keep = true
	return tmpPath, nil
}

func (u *Updater) checksum(ctx context.Context, rawURL string) (string, error) {
	resp, err := u.get(ctx, rawURL)
	if err != nil {
		return "", fmt.Errorf("could not fetch the checksum: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(body))
	if len(fields) == 0 {
		return "", errors.New("the checksum file is empty")
	}
	return fields[0], nil
}

// checkRuns runs the downloaded binary with --version and expects the
// release's version in the output, which proves it is a working gix for
// this machine before it replaces the one that works.
func checkRuns(ctx context.Context, binary, version string) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, binary, "--version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("the downloaded binary does not run: %w", err)
	}
	if !strings.Contains(string(out), version) {
		return fmt.Errorf("the downloaded binary reports %q, expected version %s", strings.TrimSpace(string(out)), version)
	}
	return nil
}

// Replace puts newPath in place of exe.
//
// Unix lets a running program's file be replaced by renaming over it.
// Windows does not, but it does let a running .exe be renamed, so the old
// one is moved aside first and removed on a later run. If the second step
// fails the old binary is put back.
func Replace(newPath, exe string, windows bool) error {
	if !windows {
		return os.Rename(newPath, exe)
	}

	old := exe + ".old"
	os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		return err
	}
	if err := os.Rename(newPath, exe); err != nil {
		os.Rename(old, exe)
		return err
	}
	os.Remove(old) // fails while the old binary is still running; CleanupOld gets it later
	return nil
}

// CleanupOld removes the binary a previous Windows update moved aside.
func CleanupOld(exe string) { os.Remove(exe + ".old") }

// Action is what Run did, or would do.
type Action string

const (
	// UpToDate means the running version is already the one wanted.
	UpToDate Action = "up to date"
	// Available means a newer release exists; only reported with CheckOnly.
	Available Action = "available"
	// DevBuild means the running binary was built from source, not
	// installed from a release, so it is left alone unless forced.
	DevBuild Action = "development build"
	// Updated means the binary was replaced.
	Updated Action = "updated"
)

// Options configures Run.
type Options struct {
	// Current is the running version, as printed by `gix --version`.
	Current string
	// Tag is a specific release to install, such as v1.0.1. Empty means
	// the latest. A specific tag may be older than the running version.
	Tag string
	// Force installs even when already up to date, and replaces a
	// development build.
	Force bool
	// CheckOnly reports and installs nothing.
	CheckOnly bool
	// ExePath is the binary to replace.
	ExePath string
}

// Result is what a Run found and did.
type Result struct {
	Action  Action
	Current string // without the leading v; "" when unreadable
	Tag     string // release considered, such as v1.1.1
	Version string // Tag without the v
}

// Run works out which release is wanted and installs it.
func (u *Updater) Run(ctx context.Context, opts Options) (*Result, error) {
	explicit := opts.Tag != ""

	tag := opts.Tag
	if explicit {
		tag = "v" + strings.TrimPrefix(tag, "v")
	} else {
		var err error
		if tag, err = u.LatestTag(ctx); err != nil {
			return nil, err
		}
	}

	wanted, err := flow.ParseVersion(strings.TrimPrefix(tag, "v"))
	if err != nil {
		return nil, fmt.Errorf("%q is not a release version", tag)
	}

	res := &Result{Tag: tag, Version: wanted.String()}

	current, isRelease := ParseCurrent(opts.Current)
	if isRelease {
		res.Current = current.String()
	}

	var need bool
	switch {
	case opts.Force:
		need = true
	case !isRelease && !explicit:
		res.Action = DevBuild
		return res, nil
	case !isRelease:
		need = true
	case explicit:
		need = wanted.Compare(current) != 0
	default:
		need = wanted.Compare(current) > 0
	}

	if !need {
		res.Action = UpToDate
		return res, nil
	}
	if opts.CheckOnly {
		res.Action = Available
		return res, nil
	}

	exe := opts.ExePath
	tmp, err := u.Download(ctx, tag, filepath.Dir(exe))
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return nil, fmt.Errorf("%w\nrun it with the permissions the install folder needs (for example `sudo gix update`), or reinstall with the install script", err)
		}
		return nil, err
	}
	if err := Replace(tmp, exe, u.GOOS == "windows"); err != nil {
		os.Remove(tmp)
		return nil, fmt.Errorf("could not replace %s: %w", exe, err)
	}

	res.Action = Updated
	return res, nil
}

// ParseCurrent reads the running version. It reports false for a build
// that did not come from a release: "dev", or what `git describe` leaves
// between releases, like 1.1.1-3-gabc123 or 1.1.1-dirty.
func ParseCurrent(s string) (flow.Version, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	v, err := flow.ParseVersion(s)
	if err != nil || isDescribeSuffix(v.Pre) {
		return flow.Version{}, false
	}
	return v, true
}

// isDescribeSuffix recognises the "<n>-g<sha>" and "dirty" parts that
// `git describe` appends, which a real prerelease name (rc.1) never has.
func isDescribeSuffix(pre string) bool {
	if pre == "" {
		return false
	}
	if pre == "dirty" || strings.HasSuffix(pre, "-dirty") {
		return true
	}
	n, rest, ok := strings.Cut(pre, "-g")
	if !ok || n == "" {
		return false
	}
	for _, r := range n {
		if r < '0' || r > '9' {
			return false
		}
	}
	rest, _, _ = strings.Cut(rest, "-")
	if rest == "" {
		return false
	}
	for _, r := range rest {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}
