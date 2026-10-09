package git

import (
	"strings"
	"testing"

	"github.com/m-mdy-m/gix/internal/testutil"
)

func TestReadConfig(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Run("config", "gix.test.value", "hello")

	cfg, err := ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}

	entry, ok := cfg.Get("gix.test.value")
	if !ok {
		t.Fatalf("gix.test.value is missing from the snapshot: %+v", cfg)
	}
	if entry.Value != "hello" {
		t.Errorf("value = %q, want %q", entry.Value, "hello")
	}
	if entry.Scope != ScopeLocal {
		t.Errorf("scope = %q, want %q", entry.Scope, ScopeLocal)
	}
	if got := cfg.ConfigValue("gix.test.value"); got != "hello" {
		t.Errorf("ConfigValue = %q, want %q", got, "hello")
	}
}

func TestReadConfigReportsScopes(t *testing.T) {
	testutil.NewRepo(t)

	cfg, err := ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}

	// The test harness writes user.name locally; the isolated global config
	// is readable too, so scope has to distinguish them.
	entry, ok := cfg.Get("user.name")
	if !ok {
		t.Fatal("user.name is missing from the snapshot")
	}
	if entry.Scope != ScopeLocal {
		t.Errorf("scope = %q, want %q for a value set with `git config` in the repo", entry.Scope, ScopeLocal)
	}
}

func TestConfigUnsetKey(t *testing.T) {
	testutil.NewRepo(t)

	cfg, err := ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}

	if _, ok := cfg.Get("gix.definitely.not.set"); ok {
		t.Error("an unset key reported as set")
	}
	if got := cfg.ConfigValue("gix.definitely.not.set"); got != "" {
		t.Errorf("ConfigValue on an unset key = %q, want empty", got)
	}
}

func TestConfigLastValueWins(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Run("config", "--add", "gix.multi", "first")
	repo.Run("config", "--add", "gix.multi", "second")

	cfg, err := ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}

	if got := cfg.ConfigValue("gix.multi"); got != "second" {
		t.Errorf("value = %q, want the last value %q", got, "second")
	}
}

func TestSetConfigLocal(t *testing.T) {
	testutil.NewRepo(t)

	if err := SetConfig("gix.test.name", "Local Name", ScopeLocal); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}

	cfg, err := ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	entry, ok := cfg.Get("gix.test.name")
	if !ok {
		t.Fatal("the key was not set")
	}
	if entry.Value != "Local Name" {
		t.Errorf("value = %q, want %q", entry.Value, "Local Name")
	}
	if entry.Scope != ScopeLocal {
		t.Errorf("scope = %q, want %q", entry.Scope, ScopeLocal)
	}
}

func TestSetConfigDefaultsToLocal(t *testing.T) {
	testutil.NewRepo(t)

	if err := SetConfig("gix.test.name", "Default Scope", ""); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}

	cfg, err := ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	entry, _ := cfg.Get("gix.test.name")
	if entry.Scope != ScopeLocal {
		t.Errorf("scope = %q, want %q when no scope is given", entry.Scope, ScopeLocal)
	}
}

func TestSetConfigGlobal(t *testing.T) {
	repo := testutil.NewRepo(t)
	// A global write must not touch the repo's own config.
	if !repo.Clean() {
		t.Fatal("repo is dirty before the test ran")
	}

	if err := SetConfig("gix.test.global", "Global Value", ScopeGlobal); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}

	cfg, err := ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	entry, ok := cfg.Get("gix.test.global")
	if !ok {
		t.Fatal("the global key was not set")
	}
	if entry.Scope != ScopeGlobal {
		t.Errorf("scope = %q, want %q", entry.Scope, ScopeGlobal)
	}
}

// gix refuses scopes it doesn't own rather than editing them anyway.
func TestSetConfigRejectsOtherScopes(t *testing.T) {
	testutil.NewRepo(t)

	for _, scope := range []string{"system", "worktree", "command", "nonsense"} {
		err := SetConfig("gix.test.name", "Nope", scope)
		if err == nil {
			t.Errorf("SetConfig with scope %q = nil error, want a rejection", scope)
			continue
		}
		if !strings.Contains(err.Error(), "unsupported config scope") {
			t.Errorf("SetConfig with scope %q = %q, want it to name the problem", scope, err)
		}
	}
}

func TestSetConfigNamesWhatFailed(t *testing.T) {
	testutil.NewRepo(t)

	err := SetConfig("", "value", ScopeLocal)
	if err == nil {
		t.Fatal("SetConfig with an empty key = nil error, want a failure")
	}
	if !strings.Contains(err.Error(), "local config") {
		t.Errorf("SetConfig = %q, want it to say which config it tried to write", err)
	}
}

func TestRemotes(t *testing.T) {
	repo := testutil.NewRepo(t)

	remotes, err := Remotes()
	if err != nil {
		t.Fatalf("Remotes: %v", err)
	}
	if len(remotes) != 0 {
		t.Errorf("Remotes() = %v, want none on a fresh repo", remotes)
	}
	if HasRemote("origin") {
		t.Error("HasRemote(origin) = true on a repo with no remotes")
	}

	repo.Run("remote", "add", "origin", "https://example.invalid/gix.git")
	repo.Run("remote", "add", "upstream", "https://example.invalid/other.git")

	remotes, err = Remotes()
	if err != nil {
		t.Fatalf("Remotes: %v", err)
	}
	if got := strings.Join(remotes, ","); got != "origin,upstream" {
		t.Errorf("Remotes() = %q, want them in git's alphabetical order", got)
	}
	if !HasRemote("origin") || !HasRemote("upstream") {
		t.Error("HasRemote did not find a configured remote")
	}
	if HasRemote("nope") {
		t.Error("HasRemote(nope) = true for a remote that isn't configured")
	}
}

func TestUpstreamOf(t *testing.T) {
	repo := testutil.NewRepo(t)

	upstream, err := UpstreamOf("main")
	if err != nil {
		t.Fatalf("UpstreamOf: %v", err)
	}
	if upstream != "" {
		t.Errorf("UpstreamOf(main) = %q, want empty for a branch that tracks nothing", upstream)
	}

	// Configure tracking directly: a real push would need a real remote.
	repo.Run("remote", "add", "origin", "https://example.invalid/gix.git")
	repo.Run("update-ref", "refs/remotes/origin/main", "main")
	repo.Run("config", "branch.main.remote", "origin")
	repo.Run("config", "branch.main.merge", "refs/heads/main")

	upstream, err = UpstreamOf("main")
	if err != nil {
		t.Fatalf("UpstreamOf: %v", err)
	}
	if upstream != "origin/main" {
		t.Errorf("UpstreamOf(main) = %q, want %q", upstream, "origin/main")
	}
}

func TestHooksPath(t *testing.T) {
	repo := testutil.NewRepo(t)

	path, err := HooksPath()
	if err != nil {
		t.Fatalf("HooksPath: %v", err)
	}
	if path == "" {
		t.Fatal("HooksPath() = empty, want the hooks directory")
	}

	// core.hooksPath wins over the default location.
	repo.Run("config", "core.hooksPath", ".githooks")
	path, err = HooksPath()
	if err != nil {
		t.Fatalf("HooksPath: %v", err)
	}
	if path != ".githooks" {
		t.Errorf("HooksPath() = %q, want %q once core.hooksPath is set", path, ".githooks")
	}
}
