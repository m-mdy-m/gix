package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr string
	}{
		{
			name: "default config is valid",
			cfg:  Default(),
		},
		{
			name:    "nil config",
			cfg:     nil,
			wantErr: "nil config",
		},
		{
			name:    "no branches",
			cfg:     &Config{},
			wantErr: "no branches defined",
		},
		{
			name: "topic without prefix",
			cfg: &Config{Branch: map[string]BranchSpec{
				"main":    {Role: RoleBase},
				"feature": {Role: RoleTopic, Parent: "main"},
			}},
			wantErr: "topic branch has no prefix",
		},
		{
			name: "topic with invalid prefix",
			cfg: &Config{Branch: map[string]BranchSpec{
				"main":    {Role: RoleBase},
				"feature": {Role: RoleTopic, Prefix: "bad prefix/", Parent: "main"},
			}},
			wantErr: "not a valid branch name component",
		},
		{
			name: "unknown role",
			cfg: &Config{Branch: map[string]BranchSpec{
				"main": {Role: "spicy"},
			}},
			wantErr: `unknown type "spicy"`,
		},
		{
			name: "invalid upstream strategy",
			cfg: &Config{Branch: map[string]BranchSpec{
				"main":    {Role: RoleBase},
				"feature": {Role: RoleTopic, Prefix: "feature/", Parent: "main", UpstreamStrategy: "cherry-pick"},
			}},
			wantErr: `invalid upstream_strategy "cherry-pick"`,
		},
		{
			name: "invalid downstream strategy",
			cfg: &Config{Branch: map[string]BranchSpec{
				"main":    {Role: RoleBase},
				"feature": {Role: RoleTopic, Prefix: "feature/", Parent: "main", DownstreamStrategy: "nope"},
			}},
			wantErr: `invalid downstream_strategy "nope"`,
		},
		{
			name: "topic with no target",
			cfg: &Config{Branch: map[string]BranchSpec{
				"main":    {Role: RoleBase},
				"feature": {Role: RoleTopic, Prefix: "feature/"},
			}},
			wantErr: "no parent or merge_into target configured",
		},
		{
			name: "topic merging into itself",
			cfg: &Config{Branch: map[string]BranchSpec{
				"main":    {Role: RoleBase},
				"feature": {Role: RoleTopic, Prefix: "feature/", MergeInto: []string{"feature"}},
			}},
			wantErr: "cannot merge into itself",
		},
		{
			name: "topic merging into undefined base",
			cfg: &Config{Branch: map[string]BranchSpec{
				"main":    {Role: RoleBase},
				"feature": {Role: RoleTopic, Prefix: "feature/", MergeInto: []string{"trunk"}},
			}},
			wantErr: `merge target "trunk" is not a defined base branch`,
		},
		{
			name: "topic merging into another topic kind",
			cfg: &Config{Branch: map[string]BranchSpec{
				"main":    {Role: RoleBase},
				"feature": {Role: RoleTopic, Prefix: "feature/", MergeInto: []string{"hotfix"}},
				"hotfix":  {Role: RoleTopic, Prefix: "hotfix/", Parent: "main"},
			}},
			wantErr: "is a topic branch, not a base branch",
		},
		{
			// merge_into overrides which targets get validated, so a parent
			// pointing at another topic kind survives the target check and is
			// only caught by the cycle walk.
			name: "circular parents",
			cfg: &Config{Branch: map[string]BranchSpec{
				"main":    {Role: RoleBase},
				"feature": {Role: RoleTopic, Prefix: "feature/", Parent: "bugfix", MergeInto: []string{"main"}},
				"bugfix":  {Role: RoleTopic, Prefix: "bugfix/", Parent: "feature", MergeInto: []string{"main"}},
			}},
			wantErr: "circular parent dependency",
		},
		{
			name: "topic parent with merge_into override is allowed",
			cfg: &Config{Branch: map[string]BranchSpec{
				"main":    {Role: RoleBase},
				"feature": {Role: RoleTopic, Prefix: "feature/", Parent: "bugfix", MergeInto: []string{"main"}},
				"bugfix":  {Role: RoleTopic, Prefix: "bugfix/", Parent: "main"},
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.cfg)
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("Validate() = %v, want no error", err)
			case tc.wantErr != "" && err == nil:
				t.Fatalf("Validate() = nil, want error containing %q", tc.wantErr)
			case tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr):
				t.Fatalf("Validate() = %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestDefaultIsValid(t *testing.T) {
	if err := Validate(Default()); err != nil {
		t.Fatalf("Default() must validate, got %v", err)
	}
}

func TestSaveThenLoad(t *testing.T) {
	root := t.TempDir()

	if Exists(root) {
		t.Fatal("Exists() = true before Save")
	}

	want := Default()
	want.Branch["main"] = BranchSpec{Name: "trunk", Role: RoleBase}
	if err := Save(root, want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if !Exists(root) {
		t.Fatal("Exists() = false after Save")
	}

	raw, err := os.ReadFile(Path(root))
	if err != nil {
		t.Fatalf("reading saved config: %v", err)
	}
	if !strings.HasPrefix(string(raw), "# gix flow config") {
		t.Errorf("saved config is missing its header comment:\n%s", raw)
	}

	got, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.MainName() != "trunk" {
		t.Errorf("MainName() = %q, want %q", got.MainName(), "trunk")
	}
	if got.DevelopName() != "develop" {
		t.Errorf("DevelopName() = %q, want %q", got.DevelopName(), "develop")
	}
}

// Configs written by older gix versions must keep loading after fields
// are added or removed, so unknown keys may not become hard errors.
func TestLoadIgnoresUnknownKeys(t *testing.T) {
	root := t.TempDir()
	old := `gix:
  version: "1"
remote: origin
branch:
  main:
    name: main
    type: base
retired_feature:
  enabled: true
`
	if err := os.MkdirAll(filepath.Join(root, DirName), 0o755); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	if err := os.WriteFile(Path(root), []byte(old), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load with unknown key: %v", err)
	}
	if cfg.MainName() != "main" {
		t.Errorf("MainName() = %q, want %q", cfg.MainName(), "main")
	}
}

func TestLoadMissing(t *testing.T) {
	_, err := Load(t.TempDir())
	if err == nil {
		t.Fatal("Load() = nil error for a repo without .gix/config")
	}
	if !strings.Contains(err.Error(), "gix flow init") {
		t.Errorf("Load() = %q, want it to point at `gix flow init`", err)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, DirName), 0o755); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	if err := os.WriteFile(Path(root), []byte("branch: [oops\n"), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	if _, err := Load(root); err == nil {
		t.Fatal("Load() = nil error for malformed YAML")
	}
}

func TestNamesFallBackToKeys(t *testing.T) {
	cfg := &Config{Branch: map[string]BranchSpec{
		"main":    {Role: RoleBase},
		"develop": {Role: RoleBase},
	}}

	if got := cfg.MainName(); got != "main" {
		t.Errorf("MainName() = %q, want %q", got, "main")
	}
	if got := cfg.DevelopName(); got != "develop" {
		t.Errorf("DevelopName() = %q, want %q", got, "develop")
	}
}

func TestNamesNilConfig(t *testing.T) {
	var cfg *Config

	if got := cfg.MainName(); got != "main" {
		t.Errorf("MainName() = %q, want %q", got, "main")
	}
	if got := cfg.DevelopName(); got != "develop" {
		t.Errorf("DevelopName() = %q, want %q", got, "develop")
	}
	if got := cfg.Kinds(); got != nil {
		t.Errorf("Kinds() = %v, want nil", got)
	}
	if got := cfg.BaseKeys(); got != nil {
		t.Errorf("BaseKeys() = %v, want nil", got)
	}
}

// main and develop come first in a stable order, then any user-defined
// kinds and bases sorted by key, so output never depends on map order.
func TestKindsAndBaseKeysAreDeterministic(t *testing.T) {
	cfg := Default()
	cfg.Branch["zeta"] = BranchSpec{Role: RoleTopic, Prefix: "zeta/", Parent: "develop"}
	cfg.Branch["alpha"] = BranchSpec{Role: RoleTopic, Prefix: "alpha/", Parent: "develop"}
	cfg.Branch["staging"] = BranchSpec{Role: RoleBase}

	wantKinds := []string{"feature", "bugfix", "hotfix", "release", "alpha", "zeta"}
	wantBases := []string{"main", "develop", "staging"}

	for i := 0; i < 20; i++ {
		if got := cfg.Kinds(); !equalStrings(got, wantKinds) {
			t.Fatalf("Kinds() = %v, want %v", got, wantKinds)
		}
		if got := cfg.BaseKeys(); !equalStrings(got, wantBases) {
			t.Fatalf("BaseKeys() = %v, want %v", got, wantBases)
		}
	}
}

func TestStrategiesDefaultToMerge(t *testing.T) {
	spec := BranchSpec{Role: RoleTopic, Prefix: "feature/"}

	if got := spec.Upstream(); got != StrategyMerge {
		t.Errorf("Upstream() = %q, want %q", got, StrategyMerge)
	}
	if got := spec.Downstream(); got != StrategyMerge {
		t.Errorf("Downstream() = %q, want %q", got, StrategyMerge)
	}

	spec.UpstreamStrategy = StrategySquash
	if got := spec.Upstream(); got != StrategySquash {
		t.Errorf("Upstream() = %q, want %q", got, StrategySquash)
	}
}

func TestMergeTargetsPrefersMergeInto(t *testing.T) {
	spec := BranchSpec{Parent: "develop", MergeInto: []string{"main", "develop"}}

	want := []string{"main", "develop"}
	if got := spec.MergeTargets(); !equalStrings(got, want) {
		t.Errorf("MergeTargets() = %v, want %v", got, want)
	}

	only := BranchSpec{Parent: "develop"}
	if got := only.MergeTargets(); !equalStrings(got, []string{"develop"}) {
		t.Errorf("MergeTargets() = %v, want [develop]", got)
	}
}

func TestStrategyValid(t *testing.T) {
	for _, s := range []Strategy{StrategyMerge, StrategyRebase, StrategySquash} {
		if !s.Valid() {
			t.Errorf("Strategy(%q).Valid() = false, want true", s)
		}
	}
	for _, s := range []Strategy{"", "cherry-pick"} {
		if s.Valid() {
			t.Errorf("Strategy(%q).Valid() = true, want false", s)
		}
	}
}

func TestSpecMissingKind(t *testing.T) {
	cfg := Default()

	if _, ok := cfg.Spec("nope"); ok {
		t.Error("Spec(\"nope\") reported a kind that doesn't exist")
	}
	if spec, ok := cfg.Spec("feature"); !ok || spec.Prefix != "feature/" {
		t.Errorf("Spec(\"feature\") = %+v, ok = %v", spec, ok)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
