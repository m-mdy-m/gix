package commands

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/m-mdy-m/gix/internal/selfupdate"
	"github.com/m-mdy-m/gix/internal/ui"
)

// updateSetup serves one fake release, installs a fake "old" binary, and
// points the update command at both. It returns the binary's path and
// what the command printed.
func updateSetup(t *testing.T, current, latest string) (exe string, out *bytes.Buffer) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake release binary is a shell script")
	}

	u := selfupdate.New("gix-test")
	name, err := selfupdate.AssetName(u.GOOS, u.GOARCH)
	if err != nil {
		t.Skip(err)
	}

	bin := []byte("#!/bin/sh\necho \"gix version " + latest + "\"\n")
	sum := sha256.Sum256(bin)
	files := map[string][]byte{
		"v" + latest + "/" + name:             bin,
		"v" + latest + "/" + name + ".sha256": []byte(hex.EncodeToString(sum[:]) + "  " + name + "\n"),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/releases/latest" {
			http.Redirect(w, r, "/releases/tag/v"+latest, http.StatusFound)
			return
		}
		if body, ok := files[strings.TrimPrefix(r.URL.Path, "/releases/download/")]; ok {
			w.Write(body)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	u.BaseURL = srv.URL

	exe = filepath.Join(t.TempDir(), "gix")
	if err := os.WriteFile(exe, []byte("old gix"), 0o755); err != nil {
		t.Fatal(err)
	}

	oldVersion, oldNew, oldExe, oldOut, oldErr := Version, newUpdater, executablePath, ui.Out, ui.Err
	t.Cleanup(func() {
		Version, newUpdater, executablePath, ui.Out, ui.Err = oldVersion, oldNew, oldExe, oldOut, oldErr
	})

	out = &bytes.Buffer{}
	Version = current
	newUpdater = func() *selfupdate.Updater { return u }
	executablePath = func() (string, error) { return exe, nil }
	ui.Out, ui.Err = out, out
	return exe, out
}

func TestUpdateInstallsTheLatestRelease(t *testing.T) {
	exe, out := updateSetup(t, "1.1.1", "1.2.0")

	if err := runUpdate(context.Background(), updateRequest{}); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if !strings.Contains(out.String(), "1.1.1 -> 1.2.0") {
		t.Errorf("output = %q, want the version change", out)
	}
	if b, _ := os.ReadFile(exe); !strings.Contains(string(b), "1.2.0") {
		t.Errorf("binary = %q, want the 1.2.0 release", b)
	}
}

func TestUpdateCheckInstallsNothing(t *testing.T) {
	exe, out := updateSetup(t, "1.1.1", "1.2.0")

	if err := runUpdate(context.Background(), updateRequest{check: true}); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if !strings.Contains(out.String(), "update available: 1.1.1 -> 1.2.0") {
		t.Errorf("output = %q, want the available update named", out)
	}
	if b, _ := os.ReadFile(exe); string(b) != "old gix" {
		t.Errorf("binary = %q, want it untouched by --check", b)
	}
}

func TestUpdateWhenAlreadyCurrent(t *testing.T) {
	exe, out := updateSetup(t, "1.2.0", "1.2.0")

	if err := runUpdate(context.Background(), updateRequest{}); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if !strings.Contains(out.String(), "up to date") {
		t.Errorf("output = %q, want it to say up to date", out)
	}
	if b, _ := os.ReadFile(exe); string(b) != "old gix" {
		t.Errorf("binary = %q, want it untouched", b)
	}
}

// The case that started this: a binary built from source says "dev" or a
// git-describe string, and `gix update` must neither pretend it is old
// nor silently overwrite something the developer just built.
func TestUpdateLeavesADevelopmentBuildAloneUnlessForced(t *testing.T) {
	exe, out := updateSetup(t, "dev", "1.2.0")

	if err := runUpdate(context.Background(), updateRequest{}); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if !strings.Contains(out.String(), "development build") || !strings.Contains(out.String(), "--force") {
		t.Errorf("output = %q, want it to explain and point to --force", out)
	}
	if b, _ := os.ReadFile(exe); string(b) != "old gix" {
		t.Fatalf("binary = %q, want a development build left alone", b)
	}

	if err := runUpdate(context.Background(), updateRequest{force: true}); err != nil {
		t.Fatalf("runUpdate --force: %v", err)
	}
	if b, _ := os.ReadFile(exe); !strings.Contains(string(b), "1.2.0") {
		t.Errorf("binary = %q, want the release after --force", b)
	}
}

func TestUpdateRejectsArguments(t *testing.T) {
	cmd := newUpdateCmd()
	if cmd.Args == nil {
		t.Fatal("update accepts positional arguments, want none")
	}
	if err := cmd.Args(cmd, []string{"v1.0.0"}); err == nil {
		t.Error("update accepted a stray argument")
	}
	if len(cmd.Aliases) == 0 || cmd.Aliases[0] != "upgrade" {
		t.Errorf("aliases = %v, want upgrade", cmd.Aliases)
	}
}

func TestUpdateIsRegisteredWithItsAlias(t *testing.T) {
	for _, c := range New().Commands() {
		if c.Name() != "update" {
			continue
		}
		if c.Short == "" || c.Long == "" {
			t.Error("update has no help text")
		}
		if len(c.Aliases) != 1 || c.Aliases[0] != "upgrade" {
			t.Errorf("aliases = %v, want [upgrade]", c.Aliases)
		}
		return
	}
	t.Error("command \"update\" is not registered")
}
