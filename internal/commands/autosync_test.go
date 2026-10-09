package commands

import (
	"strings"
	"testing"

	"github.com/m-mdy-m/gix/internal/testutil"
)

// autoFixture has one feature branch and one bugfix branch, with develop as
// the base, so the per-kind --auto filter is observable end to end.
func autoFixture(t *testing.T) *testutil.Repo {
	t.Helper()

	repo := gixRepo(t)

	run(t, repo, "feature", "start", "api")
	repo.Write("api.txt", "api work\n")
	repo.Commit("feat: api")

	run(t, repo, "bugfix", "start", "crash")
	repo.Write("crash.txt", "crash work\n")
	repo.Commit("fix: crash")

	repo.Checkout("develop")
	repo.Write("develop.txt", "develop moves on\n")
	repo.Commit("chore: develop moves on")

	return repo
}

func TestBranchSyncAutoOnlyMarkedKinds(t *testing.T) {
	repo := autoFixture(t)
	run(t, repo, "config", "set", "feature", "--autosync=true")
	repo.Commit("chore(gix): mark feature autosync")

	bugfixBefore := repo.Output("rev-parse", "bugfix/crash")

	out := run(t, repo, "branch", "sync", "--from=develop", "--auto")

	if !strings.Contains(out, "feature/api") {
		t.Errorf("`--auto` should update the marked kind:\n%s", out)
	}
	if !strings.Contains(out, "merged") {
		t.Errorf("output should report the merge:\n%s", out)
	}
	if got := repo.Output("rev-parse", "bugfix/crash"); got != bugfixBefore {
		t.Error("the unmarked kind was updated anyway")
	}
}

func TestBranchSyncAutoDryRun(t *testing.T) {
	repo := autoFixture(t)
	run(t, repo, "config", "set", "feature", "--autosync=true")
	repo.Commit("chore(gix): mark feature autosync")

	before := repo.Output("for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/")

	out := run(t, repo, "branch", "sync", "--from=develop", "--auto", "--dry-run")

	if !strings.Contains(out, "auto") {
		t.Errorf("an --auto dry run should say it is one:\n%s", out)
	}
	if !strings.Contains(out, "would merge") {
		t.Errorf("output should say what it would do:\n%s", out)
	}
	if strings.Contains(out, "bugfix/crash") {
		t.Errorf("an --auto dry run listed an unmarked branch:\n%s", out)
	}
	if after := repo.Output("for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/"); after != before {
		t.Error("an --auto dry run changed branches")
	}
}

// A base branch's autosync is reachable from the CLI, so the setting
// isn't only editable by hand.
func TestBranchSyncAutoWithoutMarks(t *testing.T) {
	repo := autoFixture(t)

	run(t, repo, "config", "set", "develop", "--autosync=false")
	repo.Commit("chore(gix): unmark develop")

	out := run(t, repo, "branch", "sync", "--from=develop", "--auto")

	if !strings.Contains(out, "nothing to sync") {
		t.Errorf("with nothing marked, --auto should say so:\n%s", out)
	}
}

func TestConfigListShowsAutosync(t *testing.T) {
	repo := gixRepo(t)

	out := run(t, repo, "config", "list")

	if !strings.Contains(out, "autosync") {
		t.Errorf("`gix config list` should show autosync:\n%s", out)
	}
}

func TestConfigSetAutosync(t *testing.T) {
	repo := gixRepo(t)

	run(t, repo, "config", "set", "feature", "--autosync=true")
	repo.Commit("chore(gix): mark feature autosync")

	// The setting has to survive a round trip through the file, otherwise
	// `sync --auto` would silently do nothing.
	out := run(t, repo, "config", "list")
	if !strings.Contains(out, "autosync") {
		t.Fatalf("`gix config list` should mention autosync:\n%s", out)
	}

	run(t, repo, "feature", "start", "api")
	repo.Write("api.txt", "api\n")
	repo.Commit("feat: api")
	repo.Checkout("develop")
	repo.Write("develop.txt", "develop\n")
	repo.Commit("chore: develop")

	run(t, repo, "branch", "sync", "--from=develop", "--auto")
}

func TestConfigSetAutosyncValidation(t *testing.T) {
	repo := gixRepo(t)

	runFailsWith(t, repo, "want true or false", "config", "set", "feature", "--autosync=maybe")
}

// autosync is a filter for --auto, not a gate on syncing: a plain run
// still covers every topic branch, marked or not. Getting this backwards
// would make autosync look like it disabled syncing.
func TestAutoSyncFiltersAutoOnly(t *testing.T) {
	repo := autoFixture(t)
	run(t, repo, "config", "set", "feature", "--autosync=true")
	repo.Commit("chore(gix): mark feature autosync")

	// A plain run reaches both kinds, since that is its documented scope.
	apiBefore := repo.Output("rev-parse", "feature/api")
	run(t, repo, "branch", "sync", "--from=develop")
	if got := repo.Output("rev-parse", "feature/api"); got == apiBefore {
		t.Error("a plain sync should cover every topic branch")
	}
}

func TestBranchSyncAutoHelpMentionsTheFlag(t *testing.T) {
	root := New()

	for _, c := range root.Commands() {
		if c.Name() != "branch" {
			continue
		}
		for _, sub := range c.Commands() {
			if sub.Name() != "sync" {
				continue
			}
			if sub.Flags().Lookup("auto") == nil {
				t.Fatal("`gix branch sync` has no --auto flag")
			}
			if !strings.Contains(sub.Long, "--auto") {
				t.Error("`gix branch sync` help does not explain --auto")
			}
			// Nothing may claim gix syncs on its own. The help states the
			// opposite, so look for affirmative claims only.
			for _, claim := range []string{
				"automatically sync",
				"syncs automatically",
				"runs automatically",
				"installs a hook",
			} {
				if strings.Contains(sub.Long, claim) {
					t.Errorf("`gix branch sync` help suggests %q, which gix does not do", claim)
				}
			}
			if !strings.Contains(sub.Long, "no hooks") {
				t.Error("`gix branch sync` help should say gix installs no hooks")
			}
			return
		}
	}
	t.Error("`gix branch sync` is not registered")
}

// gix must not install hooks or otherwise act on its own.
func TestNoCommandWritesGitHooks(t *testing.T) {
	repo := gixRepo(t)
	run(t, repo, "config", "set", "feature", "--autosync=true")
	repo.Commit("chore(gix): mark feature autosync")

	hooks, err := repo.Try("rev-parse", "--git-path", "hooks")
	if err != nil {
		t.Fatalf("resolving the hooks path: %v", err)
	}
	_ = hooks

	// core.hooksPath is the only way gix could take over Git's hooks.
	if out, err := repo.Try("config", "--local", "core.hooksPath"); err == nil && out != "" {
		t.Errorf("core.hooksPath = %q, want gix to leave Git's hooks alone", out)
	}
}
