package commands

import (
	"strings"
	"testing"

	"github.com/m-mdy-m/gix/internal/testutil"
)

// syncFixture builds develop with two feature branches: one that merges
// cleanly and one that conflicts with develop.
func syncFixture(t *testing.T) *testutil.Repo {
	t.Helper()

	repo := gixRepo(t)

	run(t, repo, "feature", "start", "clean")
	repo.Write("clean.txt", "clean work\n")
	repo.Commit("feat: clean")

	run(t, repo, "feature", "start", "conflicted")
	repo.Write("shared.txt", "from the feature branch\n")
	repo.Commit("feat: conflicted touches shared")

	run(t, repo, "feature", "start", "second")
	repo.Write("second.txt", "second work\n")
	repo.Commit("feat: second")

	// develop now moves on and changes the file one branch also changed.
	repo.Checkout("develop")
	repo.Write("develop.txt", "develop\n")
	repo.Commit("chore: develop moves on")
	repo.Write("shared.txt", "from develop\n")
	repo.Commit("chore: develop touches shared")

	return repo
}

func TestBranchSyncDryRun(t *testing.T) {
	repo := syncFixture(t)

	before := repo.Output("for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/")

	// A preview of a conflicting run still exits non-zero, so CI can rely
	// on that; exec captures the output without failing the test.
	out, err := exec(t, repo, "branch", "sync", "--from=develop", "--dry-run")
	if err == nil {
		t.Errorf("a dry run of a conflicting sync returned no error:\n%s", out)
	}

	if !strings.Contains(out, "dry run") {
		t.Errorf("`gix branch sync --dry-run` should say it is a preview:\n%s", out)
	}
	if !strings.Contains(out, "would merge") {
		t.Errorf("`gix branch sync --dry-run` should say what it would do:\n%s", out)
	}
	if !strings.Contains(out, "conflict") {
		t.Errorf("`gix branch sync --dry-run` should report the conflicting branch:\n%s", out)
	}
	if !strings.Contains(out, "shared.txt") {
		t.Errorf("`gix branch sync --dry-run` should name the conflicting file:\n%s", out)
	}
	if !strings.Contains(out, "has not changed anything") {
		t.Errorf("`gix branch sync --dry-run` should state nothing changed:\n%s", out)
	}

	if after := repo.Output("for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/"); after != before {
		t.Errorf("a dry run moved branches:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestBranchSyncReportsConflictsWithNonZeroExit(t *testing.T) {
	repo := syncFixture(t)

	out, err := exec(t, repo, "branch", "sync", "--from=develop", "--dry-run")
	if err == nil {
		t.Fatalf("`gix branch sync --dry-run` returned no error for a conflict:\n%s", out)
	}
	if !strings.Contains(out, "conflict") {
		t.Errorf("output should report the conflict:\n%s", out)
	}
}

func TestBranchSyncMergesAndContinuesPastConflicts(t *testing.T) {
	repo := syncFixture(t)
	cleanBefore := repo.Output("rev-parse", "feature/clean")
	conflictedBefore := repo.Output("rev-parse", "feature/conflicted")

	out, err := exec(t, repo, "branch", "sync", "--from=develop")
	if err == nil {
		t.Fatalf("`gix branch sync` returned no error with a conflicting branch:\n%s", out)
	}

	if !strings.Contains(out, "merged") {
		t.Errorf("output should report the successful merges:\n%s", out)
	}
	if !strings.Contains(out, "conflict") {
		t.Errorf("output should report the conflict:\n%s", out)
	}
	// The summary of what did and didn't happen travels on the error, which
	// main prints to stderr.
	if !strings.Contains(err.Error(), "other branches were synced") {
		t.Errorf("error = %q, want it to say the other branches were handled", err)
	}

	if got := repo.Output("rev-parse", "feature/clean"); got == cleanBefore {
		t.Error("feature/clean was not synced even though it merged cleanly")
	}
	if got := repo.Output("rev-parse", "feature/conflicted"); got != conflictedBefore {
		t.Error("feature/conflicted was changed despite the conflict")
	}
	if !repo.Clean() {
		t.Errorf("`gix branch sync` left the working tree dirty:\n%s", repo.Output("status", "--porcelain"))
	}
}

func TestBranchSyncRestoresTheCurrentBranch(t *testing.T) {
	repo := syncFixture(t)
	repo.Checkout("feature/second")

	exec(t, repo, "branch", "sync", "--from=develop")

	if got := repo.CurrentBranch(); got != "feature/second" {
		t.Errorf("current branch = %q, want feature/second", got)
	}
}

func TestBranchSyncUpToDate(t *testing.T) {
	repo := gixRepo(t)
	run(t, repo, "feature", "start", "api")
	repo.Write("api.txt", "api\n")
	repo.Commit("feat: api")

	out := run(t, repo, "branch", "sync", "--from=develop")

	if !strings.Contains(out, "up to date") {
		t.Errorf("`gix branch sync` should report nothing to do:\n%s", out)
	}
}

func TestBranchSyncNothingToSync(t *testing.T) {
	repo := gixRepo(t)

	out := run(t, repo, "branch", "sync", "--from=develop")

	if !strings.Contains(out, "nothing to sync") {
		t.Errorf("`gix branch sync` with no open branches should say so:\n%s", out)
	}
}

func TestBranchSyncMissingSource(t *testing.T) {
	repo := gixRepo(t)

	runFailsWith(t, repo, "does not exist", "branch", "sync", "--from=nonexistent")
}

func TestBranchSyncRefusesDirtyTree(t *testing.T) {
	repo := gixRepo(t)
	repo.Write("wip.txt", "uncommitted\n")

	runFailsWith(t, repo, "working tree is not clean", "branch", "sync", "--from=develop")
}

func TestBranchSyncNeedsGixConfig(t *testing.T) {
	repo := testutil.NewRepo(t)

	runFailsWith(t, repo, "gix is not initialized", "branch", "sync")
}

func TestBranchSyncRejectsExtraArgs(t *testing.T) {
	repo := gixRepo(t)

	runFails(t, repo, "branch", "sync", "unexpected")
}

func TestBranchSyncGroupIsRegistered(t *testing.T) {
	root := New()

	for _, c := range root.Commands() {
		if c.Name() != "branch" {
			continue
		}
		for _, sub := range c.Commands() {
			if sub.Name() == "sync" {
				if sub.Short == "" || sub.Long == "" {
					t.Error("`gix branch sync` is missing help text")
				}
				return
			}
		}
	}
	t.Error("`gix branch sync` is not registered")
}

func TestBranchSyncHasHelp(t *testing.T) {
	repo := gixRepo(t)

	run(t, repo, "branch", "sync", "--help")
}

// Sync is the first gix command that merges, so its help must state the
// rules it will not break.
func TestBranchSyncHelpStatesItsGuarantees(t *testing.T) {
	root := New()
	for _, c := range root.Commands() {
		if c.Name() != "branch" {
			continue
		}
		for _, sub := range c.Commands() {
			if sub.Name() != "sync" {
				continue
			}
			for _, want := range []string{"conflict", "dirty", "never", "dry-run"} {
				if !strings.Contains(strings.ToLower(sub.Long), want) {
					t.Errorf("`gix branch sync` help does not mention %q", want)
				}
			}
			return
		}
	}
}
