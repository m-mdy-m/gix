package flow

import (
	"github.com/m-mdy-m/gix/internal/config"
	"github.com/m-mdy-m/gix/internal/git"
)

// MissingBase names one base branch that Validate accepted as
// syntactically fine but that doesn't actually exist as a branch in
// this repository — e.g. after a manual `git branch -m main trunk`.
type MissingBase struct {
	Key  string // config key, e.g. "main"
	Name string // resolved branch name that's missing, e.g. "main" or a renamed value
}

// Health summarizes whether the current repository actually satisfies
// the flow described by cfg, beyond what config.Validate can check
// from the file alone.
type Health struct {
	MissingBases []MissingBase
}

// OK reports whether every configured base branch exists in the
// repository.
func (h Health) OK() bool { return len(h.MissingBases) == 0 }

// CheckHealth inspects every configured base branch (main, develop,
// and any others) and reports which ones don't exist as actual
// branches in the repository.
func CheckHealth(cfg *config.Config) Health {
	var h Health
	for _, key := range cfg.BaseKeys() {
		name := resolveBranchName(cfg, key)
		if !git.BranchExists(name) {
			h.MissingBases = append(h.MissingBases, MissingBase{Key: key, Name: name})
		}
	}
	return h
}

// IsConfiguredBase reports whether branch matches the resolved name of
// any configured base branch, returning the config key it matched
// (e.g. "main").
func IsConfiguredBase(cfg *config.Config, branch string) (key string, ok bool) {
	for _, k := range cfg.BaseKeys() {
		if resolveBranchName(cfg, k) == branch {
			return k, true
		}
	}
	return "", false
}
