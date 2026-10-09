package flow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m-mdy-m/gix/internal/config"
)

func TestCheckHealthMissingBase(t *testing.T) {
	repo, cfg := flowRepo(t)

	if h := CheckHealth(cfg); !h.OK() {
		t.Errorf("CheckHealth = %+v, want OK on an untouched flow", h)
	}

	repo.Run("branch", "-D", "develop")

	h := CheckHealth(cfg)
	if h.OK() {
		t.Fatal("CheckHealth = OK after deleting develop, want a missing base")
	}
	if len(h.MissingBases) != 1 {
		t.Fatalf("MissingBases = %+v, want exactly one entry", h.MissingBases)
	}
	if h.MissingBases[0].Key != "develop" || h.MissingBases[0].Name != "develop" {
		t.Errorf("MissingBases[0] = %+v, want develop/develop", h.MissingBases[0])
	}
}

// A base branch renamed outside gix is invisible until the config names
// the new branch, and health has to point at the right name to fix it.
func TestCheckHealthFollowsRenamedBase(t *testing.T) {
	repo, cfg := flowRepo(t)
	repo.Run("branch", "-m", "main", "trunk")

	h := CheckHealth(cfg)
	if h.OK() {
		t.Fatal("CheckHealth = OK, want main reported missing after the rename")
	}
	if h.MissingBases[0].Key != "main" || h.MissingBases[0].Name != "main" {
		t.Errorf("MissingBases[0] = %+v, want main/main", h.MissingBases[0])
	}

	cfg.Branch["main"] = config.BranchSpec{Name: "trunk", Role: config.RoleBase}
	if err := config.Save(repo.Root, cfg); err != nil {
		t.Fatalf("saving config: %v", err)
	}
	if h := CheckHealth(cfg); !h.OK() {
		t.Errorf("CheckHealth = %+v, want OK once the config names trunk", h)
	}
}

func TestIsConfiguredBase(t *testing.T) {
	_, cfg := flowRepo(t)

	tests := []struct {
		branch  string
		wantOK  bool
		wantKey string
	}{
		{branch: "main", wantOK: true, wantKey: "main"},
		{branch: "develop", wantOK: true, wantKey: "develop"},
		{branch: "feature/api", wantOK: false},
	}

	for _, tc := range tests {
		t.Run(tc.branch, func(t *testing.T) {
			key, ok := IsConfiguredBase(cfg, tc.branch)
			if ok != tc.wantOK {
				t.Fatalf("IsConfiguredBase(%q) ok = %v, want %v", tc.branch, ok, tc.wantOK)
			}
			if key != tc.wantKey {
				t.Errorf("IsConfiguredBase(%q) key = %q, want %q", tc.branch, key, tc.wantKey)
			}
		})
	}
}

func TestIsConfiguredBaseUsesConfiguredName(t *testing.T) {
	_, cfg := flowRepo(t)
	cfg.Branch["main"] = config.BranchSpec{Name: "trunk", Role: config.RoleBase}

	if key, ok := IsConfiguredBase(cfg, "trunk"); !ok || key != "main" {
		t.Errorf("IsConfiguredBase(trunk) = %q, %v; want main, true", key, ok)
	}
	if _, ok := IsConfiguredBase(cfg, "main"); ok {
		t.Error("IsConfiguredBase(main) matched even though the config renamed it to trunk")
	}
}

func contains(haystack, needle string) bool { return strings.Contains(haystack, needle) }

func join(items []string) string { return strings.Join(items, ",") }

func fileExists(t *testing.T, root, name string) bool {
	t.Helper()

	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(name)))
	return err == nil
}
