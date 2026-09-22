package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/m-mdy-m/gix/internal/logger"
)

// DirName is the directory under the repo root gix stores its config in.
const DirName = ".gix"

// FileName is the config file's name inside DirName.
const FileName = "config"

// SchemaVersion is written to new configs and checked (loosely) on load.
const SchemaVersion = "1"

// log is this package's diagnostic logger, active only under --verbose
// / --debug (see internal/logger).
var log = logger.WithPrefix("config")

// Path returns the absolute path to the config file for a repo rooted
// at repoRoot.
func Path(repoRoot string) string {
	return filepath.Join(repoRoot, DirName, FileName)
}

// Exists reports whether a gix config already exists in repoRoot.
func Exists(repoRoot string) bool {
	_, err := os.Stat(Path(repoRoot))
	return err == nil
}

func Default() *Config {
	return &Config{
		Gix:    GixMeta{Version: SchemaVersion},
		Remote: "origin",
		Branch: map[string]BranchSpec{
			"main": {
				Name: "main",
				Role: RoleBase,
			},
			"develop": {
				Name:     "develop",
				Role:     RoleBase,
				Parent:   "main",
				AutoSync: true,
			},
			"feature": {
				Role:               RoleTopic,
				Prefix:             "feature/",
				Parent:             "develop",
				UpstreamStrategy:   StrategyMerge,
				DownstreamStrategy: StrategyMerge,
				DeleteOnFinish:     true,
			},
			"bugfix": {
				Role:               RoleTopic,
				Prefix:             "bugfix/",
				Parent:             "develop",
				UpstreamStrategy:   StrategyMerge,
				DownstreamStrategy: StrategyMerge,
				DeleteOnFinish:     true,
			},
			"hotfix": {
				Role:               RoleTopic,
				Prefix:             "hotfix/",
				Parent:             "main",
				MergeInto:          []string{"main", "develop"},
				UpstreamStrategy:   StrategyMerge,
				DownstreamStrategy: StrategyMerge,
				Tag:                true,
				DeleteOnFinish:     true,
			},
			"release": {
				Role:               RoleTopic,
				Prefix:             "release/",
				Parent:             "develop",
				MergeInto:          []string{"main", "develop"},
				UpstreamStrategy:   StrategyMerge,
				DownstreamStrategy: StrategyMerge,
				Tag:                true,
				DeleteOnFinish:     true,
			},
		},
	}
}

func Load(repoRoot string) (*Config, error) {
	path := Path(repoRoot)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("gix is not initialized in this repository — run `gix flow init` first")
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if err := Validate(cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	log.Debugf("loaded config from %s (%d branch kinds)", path, len(cfg.Branch))
	return cfg, nil
}

// Save writes cfg to repoRoot's config file, creating .gix/ if needed.
func Save(repoRoot string, cfg *Config) error {
	if err := Validate(cfg); err != nil {
		return err
	}

	dir := filepath.Join(repoRoot, DirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	out, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	path := Path(repoRoot)
	header := "# gix flow config — see `gix config list` or docs/FLAGS.md\n"
	if err := os.WriteFile(path, append([]byte(header), out...), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	log.Debugf("wrote config to %s", path)
	return nil
}

func Validate(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("nil config")
	}
	if len(cfg.Branch) == 0 {
		return fmt.Errorf("no branches defined")
	}

	baseKeys := make(map[string]bool, len(cfg.Branch))
	for key, spec := range cfg.Branch {
		if spec.Role == RoleBase {
			baseKeys[key] = true
		}
	}

	for key, spec := range cfg.Branch {
		switch spec.Role {
		case RoleBase:
			// nothing further to check
		case RoleTopic:
			if spec.Prefix == "" {
				return fmt.Errorf("branch.%s: topic branch has no prefix", key)
			}
			if !isValidBranchNamePart(spec.Prefix) {
				return fmt.Errorf("branch.%s: prefix %q is not a valid branch name component", key, spec.Prefix)
			}
			if spec.UpstreamStrategy != "" && !spec.UpstreamStrategy.Valid() {
				return fmt.Errorf("branch.%s: invalid upstream_strategy %q", key, spec.UpstreamStrategy)
			}
			if spec.DownstreamStrategy != "" && !spec.DownstreamStrategy.Valid() {
				return fmt.Errorf("branch.%s: invalid downstream_strategy %q", key, spec.DownstreamStrategy)
			}
			targets := spec.MergeTargets()
			if len(targets) == 0 {
				return fmt.Errorf("branch.%s: no parent or merge_into target configured", key)
			}
			for _, t := range targets {
				if t == key {
					return fmt.Errorf("branch.%s: cannot merge into itself", key)
				}
				if !baseKeys[t] {
					if _, isKind := cfg.Branch[t]; isKind {
						return fmt.Errorf("branch.%s: merge target %q is a %s branch, not a base branch — topic kinds can only merge into base branches", key, t, cfg.Branch[t].Role)
					}
					return fmt.Errorf("branch.%s: merge target %q is not a defined base branch", key, t)
				}
			}
		default:
			return fmt.Errorf("branch.%s: unknown type %q (expected %q or %q)", key, spec.Role, RoleBase, RoleTopic)
		}
	}

	if err := checkParentCycles(cfg); err != nil {
		return err
	}

	return nil
}

func checkParentCycles(cfg *Config) error {
	for start, spec := range cfg.Branch {
		if spec.Role != RoleTopic {
			continue
		}
		visited := map[string]bool{start: true}
		cur := spec.Parent
		chain := []string{start}
		for cur != "" {
			chain = append(chain, cur)
			if visited[cur] {
				return fmt.Errorf("branch.%s: circular parent dependency detected: %s", start, strings.Join(chain, " -> "))
			}
			next, ok := cfg.Branch[cur]
			if !ok || next.Role == RoleBase {
				break // bottoms out at a base branch (or an already-reported missing key) — fine
			}
			visited[cur] = true
			cur = next.Parent
		}
	}
	return nil
}

// isValidBranchNamePart reports whether s is safe to use as a branch
// name or branch name prefix under Git's ref-naming rules.
func isValidBranchNamePart(s string) bool {
	if s == "" {
		return false
	}
	if strings.Contains(s, "..") {
		return false
	}
	if strings.HasPrefix(s, "/") {
		return false
	}
	if strings.ContainsAny(s, " \t\n~^:?*[\\") {
		return false
	}
	if strings.HasSuffix(s, ".lock") {
		return false
	}
	return true
}
