package selfupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

func TestAssetName(t *testing.T) {
	tests := []struct {
		goos, goarch, want string
		wantErr            bool
	}{
		{"linux", "amd64", "gix-linux-x64", false},
		{"linux", "arm64", "gix-linux-arm64", false},
		{"darwin", "amd64", "gix-darwin-x64", false},
		{"darwin", "arm64", "gix-darwin-arm64", false},
		{"windows", "amd64", "gix-windows-x64.exe", false},
		{"windows", "arm64", "gix-windows-arm64.exe", false},
		{"freebsd", "amd64", "", true},
		{"linux", "386", "", true},
	}
	for _, tt := range tests {
		got, err := AssetName(tt.goos, tt.goarch)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("AssetName(%s, %s) = %q, %v; want %q, error=%v", tt.goos, tt.goarch, got, err, tt.want, tt.wantErr)
		}
	}
}

func TestParseCurrent(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		release bool
	}{
		{"1.1.1", "1.1.1", true},
		{"v1.1.1", "1.1.1", true},
		{"1.2.0-rc.1", "1.2.0-rc.1", true},
		{"dev", "", false},
		{"", "", false},
		{"1.1.1-3-gabc1234", "", false},
		{"v1.1.1-3-gabc1234-dirty", "", false},
		{"1.1.1-dirty", "", false},
		{"0.0.0-dev", "0.0.0-dev", true}, // CI branch builds: older than every release
	}
	for _, tt := range tests {
		v, ok := ParseCurrent(tt.in)
		if ok != tt.release {
			t.Errorf("ParseCurrent(%q) release = %v, want %v", tt.in, ok, tt.release)
			continue
		}
		if ok && v.String() != tt.want {
			t.Errorf("ParseCurrent(%q) = %s, want %s", tt.in, v, tt.want)
		}
	}
}

// release is a fake GitHub release server.
type release struct {
	srv      *httptest.Server
	latest   string // tag /releases/latest redirects to; "" for none
	files    map[string][]byte
	requests atomic.Int32
}

func newRelease(t *testing.T) *release {
	t.Helper()

	r := &release{files: map[string][]byte{}}
	r.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.requests.Add(1)
		switch {
		case req.URL.Path == "/releases/latest":
			if r.latest == "" {
				http.Redirect(w, req, "/releases", http.StatusFound)
				return
			}
			http.Redirect(w, req, "/releases/tag/"+r.latest, http.StatusFound)
		case strings.HasPrefix(req.URL.Path, "/releases/download/"):
			body, ok := r.files[strings.TrimPrefix(req.URL.Path, "/releases/download/")]
			if !ok {
				http.NotFound(w, req)
				return
			}
			w.Write(body)
		default:
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(r.srv.Close)
	return r
}

// publish adds a release for the platform the test runs on, whose binary
// is a script that prints "gix version <reported>".
func (r *release) publish(t *testing.T, u *Updater, tag, reported string) []byte {
	t.Helper()

	name, err := AssetName(u.GOOS, u.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	bin := []byte("#!/bin/sh\necho \"gix version " + reported + "\"\n")
	sum := sha256.Sum256(bin)

	r.files[tag+"/"+name] = bin
	r.files[tag+"/"+name+".sha256"] = []byte(hex.EncodeToString(sum[:]) + "  " + name + "\n")
	return bin
}

func (r *release) updater() *Updater {
	u := New("gix-test")
	u.BaseURL = r.srv.URL
	return u
}

// scriptsRun skips tests that execute the fake binary where that cannot work.
func scriptsRun(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake release binary is a shell script")
	}
}

func installedExe(t *testing.T) (dir, exe string) {
	t.Helper()
	dir = t.TempDir()
	exe = filepath.Join(dir, "gix")
	if err := os.WriteFile(exe, []byte("old gix"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir, exe
}

func content(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func noLeftovers(t *testing.T, dir string) {
	t.Helper()
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".gix-update-") {
			t.Errorf("temporary file %s was left behind", e.Name())
		}
	}
}

func TestLatestTag(t *testing.T) {
	r := newRelease(t)
	u := r.updater()

	r.latest = "v1.1.1"
	tag, err := u.LatestTag(context.Background())
	if err != nil || tag != "v1.1.1" {
		t.Fatalf("LatestTag = %q, %v; want v1.1.1", tag, err)
	}

	// A repository with no releases redirects to /releases.
	r.latest = ""
	if tag, err := u.LatestTag(context.Background()); err == nil {
		t.Errorf("LatestTag = %q, nil; want an error when nothing is published", tag)
	}
}

func TestRunUpdatesToTheLatestRelease(t *testing.T) {
	scriptsRun(t)
	r := newRelease(t)
	u := r.updater()
	r.latest = "v9.9.9"
	bin := r.publish(t, u, "v9.9.9", "9.9.9")
	dir, exe := installedExe(t)

	res, err := u.Run(context.Background(), Options{Current: "1.1.1", ExePath: exe})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Action != Updated || res.Version != "9.9.9" || res.Current != "1.1.1" {
		t.Errorf("result = %+v, want Updated 1.1.1 -> 9.9.9", res)
	}
	if got := content(t, exe); got != string(bin) {
		t.Errorf("binary = %q, want the downloaded release", got)
	}
	if info, _ := os.Stat(exe); info.Mode()&0o111 == 0 {
		t.Error("the new binary is not executable")
	}
	noLeftovers(t, dir)
}

func TestRunLeavesAnUpToDateBinaryAlone(t *testing.T) {
	r := newRelease(t)
	u := r.updater()
	r.latest = "v1.1.1"
	_, exe := installedExe(t)

	for _, current := range []string{"1.1.1", "v1.1.1", "1.2.0"} {
		res, err := u.Run(context.Background(), Options{Current: current, ExePath: exe})
		if err != nil {
			t.Fatalf("Run(%s): %v", current, err)
		}
		if res.Action != UpToDate {
			t.Errorf("Run(%s) = %s, want %s", current, res.Action, UpToDate)
		}
	}
	if got := content(t, exe); got != "old gix" {
		t.Errorf("binary = %q, want it untouched", got)
	}
}

func TestRunCheckOnlyInstallsNothing(t *testing.T) {
	r := newRelease(t)
	u := r.updater()
	r.latest = "v9.9.9"
	r.publish(t, u, "v9.9.9", "9.9.9")
	_, exe := installedExe(t)

	res, err := u.Run(context.Background(), Options{Current: "1.1.1", ExePath: exe, CheckOnly: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Action != Available || res.Version != "9.9.9" {
		t.Errorf("result = %+v, want Available 9.9.9", res)
	}
	if got := content(t, exe); got != "old gix" {
		t.Errorf("binary = %q, want it untouched by --check", got)
	}
}

func TestRunRefusesABadChecksum(t *testing.T) {
	scriptsRun(t)
	r := newRelease(t)
	u := r.updater()
	r.latest = "v9.9.9"
	r.publish(t, u, "v9.9.9", "9.9.9")
	name, _ := AssetName(u.GOOS, u.GOARCH)
	r.files["v9.9.9/"+name+".sha256"] = []byte(strings.Repeat("0", 64) + "  " + name + "\n")
	dir, exe := installedExe(t)

	_, err := u.Run(context.Background(), Options{Current: "1.1.1", ExePath: exe})
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("Run error = %v, want a checksum mismatch", err)
	}
	if got := content(t, exe); got != "old gix" {
		t.Errorf("binary = %q, want the old one kept", got)
	}
	noLeftovers(t, dir)
}

func TestRunRefusesAReleaseWithoutAChecksum(t *testing.T) {
	scriptsRun(t)
	r := newRelease(t)
	u := r.updater()
	r.latest = "v9.9.9"
	r.publish(t, u, "v9.9.9", "9.9.9")
	name, _ := AssetName(u.GOOS, u.GOARCH)
	delete(r.files, "v9.9.9/"+name+".sha256")
	dir, exe := installedExe(t)

	if _, err := u.Run(context.Background(), Options{Current: "1.1.1", ExePath: exe}); err == nil {
		t.Fatal("Run succeeded without a checksum, want it refused")
	}
	if got := content(t, exe); got != "old gix" {
		t.Errorf("binary = %q, want the old one kept", got)
	}
	noLeftovers(t, dir)
}

func TestRunRefusesABinaryThatReportsTheWrongVersion(t *testing.T) {
	scriptsRun(t)
	r := newRelease(t)
	u := r.updater()
	r.latest = "v9.9.9"
	r.publish(t, u, "v9.9.9", "0.0.1") // checksum is right, content is not the release
	dir, exe := installedExe(t)

	_, err := u.Run(context.Background(), Options{Current: "1.1.1", ExePath: exe})
	if err == nil || !strings.Contains(err.Error(), "expected version 9.9.9") {
		t.Fatalf("Run error = %v, want the version mismatch named", err)
	}
	if got := content(t, exe); got != "old gix" {
		t.Errorf("binary = %q, want the old one kept", got)
	}
	noLeftovers(t, dir)
}

func TestRunWithADevelopmentBuild(t *testing.T) {
	scriptsRun(t)
	r := newRelease(t)
	u := r.updater()
	r.latest = "v9.9.9"
	bin := r.publish(t, u, "v9.9.9", "9.9.9")

	for _, current := range []string{"dev", "1.1.1-3-gabc1234"} {
		_, exe := installedExe(t)

		res, err := u.Run(context.Background(), Options{Current: current, ExePath: exe})
		if err != nil {
			t.Fatalf("Run(%s): %v", current, err)
		}
		if res.Action != DevBuild {
			t.Errorf("Run(%s) = %s, want %s", current, res.Action, DevBuild)
		}
		if got := content(t, exe); got != "old gix" {
			t.Errorf("Run(%s) changed a development build without --force", current)
		}

		res, err = u.Run(context.Background(), Options{Current: current, ExePath: exe, Force: true})
		if err != nil || res.Action != Updated {
			t.Fatalf("Run(%s, force) = %+v, %v; want Updated", current, res, err)
		}
		if got := content(t, exe); got != string(bin) {
			t.Errorf("Run(%s, force) left %q, want the release", current, got)
		}
	}
}

func TestRunInstallsASpecificOlderRelease(t *testing.T) {
	scriptsRun(t)
	r := newRelease(t)
	u := r.updater()
	r.latest = "v9.9.9"
	r.publish(t, u, "v9.9.9", "9.9.9")
	old := r.publish(t, u, "v1.0.1", "1.0.1")
	_, exe := installedExe(t)

	res, err := u.Run(context.Background(), Options{Current: "9.9.9", Tag: "1.0.1", ExePath: exe})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Action != Updated || res.Version != "1.0.1" {
		t.Errorf("result = %+v, want Updated to 1.0.1", res)
	}
	if got := content(t, exe); got != string(old) {
		t.Errorf("binary = %q, want the 1.0.1 release", got)
	}

	// Asking for the version already running is not an update.
	res, err = u.Run(context.Background(), Options{Current: "1.0.1", Tag: "v1.0.1", ExePath: exe})
	if err != nil || res.Action != UpToDate {
		t.Errorf("Run(same version) = %+v, %v; want UpToDate", res, err)
	}
}

func TestRunWithAnUnknownTag(t *testing.T) {
	r := newRelease(t)
	u := r.updater()
	_, exe := installedExe(t)

	if _, err := u.Run(context.Background(), Options{Current: "1.1.1", Tag: "v8.8.8", ExePath: exe}); err == nil {
		t.Fatal("Run succeeded for a release that does not exist")
	}
	if _, err := u.Run(context.Background(), Options{Current: "1.1.1", Tag: "nightly", ExePath: exe}); err == nil {
		t.Fatal("Run accepted a tag that is not a version")
	}
	if got := content(t, exe); got != "old gix" {
		t.Errorf("binary = %q, want it untouched", got)
	}
}

func TestReplaceWindowsStyleMovesTheOldBinaryAside(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "gix.exe")
	next := filepath.Join(dir, "next.exe")
	os.WriteFile(exe, []byte("old"), 0o755)
	os.WriteFile(next, []byte("new"), 0o755)

	if err := Replace(next, exe, true); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if got := content(t, exe); got != "new" {
		t.Errorf("binary = %q, want the new one", got)
	}
	if _, err := os.Stat(next); !os.IsNotExist(err) {
		t.Error("the downloaded file is still around")
	}
}

func TestReplaceWindowsStylePutsTheOldBinaryBackOnFailure(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "gix.exe")
	os.WriteFile(exe, []byte("old"), 0o755)

	err := Replace(filepath.Join(dir, "missing.exe"), exe, true)
	if err == nil {
		t.Fatal("Replace succeeded with no new binary")
	}
	if got := content(t, exe); got != "old" {
		t.Errorf("binary = %q, want the old one restored", got)
	}
	if _, err := os.Stat(exe + ".old"); !os.IsNotExist(err) {
		t.Error("a .old file was left behind")
	}
}

func TestCleanupOld(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "gix.exe")
	os.WriteFile(exe+".old", []byte("old"), 0o755)

	CleanupOld(exe)
	if _, err := os.Stat(exe + ".old"); !os.IsNotExist(err) {
		t.Error("CleanupOld left the .old file")
	}
	CleanupOld(exe) // nothing to remove is fine
}
