package commands

import (
	"strings"
	"testing"

	"github.com/m-mdy-m/gix/internal/config"
	"github.com/m-mdy-m/gix/internal/testutil"
)

func TestDoctorStrictExitsNonZeroOnProblems(t *testing.T) {
	repo := gixRepo(t)
	repo.Write("wip.txt", "uncommitted\n")

	out, err := exec(t, repo, "doctor", "--strict")
	if err == nil {
		t.Fatalf("`gix doctor --strict` returned no error with a dirty tree:\n%s", out)
	}
	if !strings.Contains(out, "uncommitted changes") {
		t.Errorf("the report should still be printed in full:\n%s", out)
	}
	if !strings.Contains(err.Error(), "needs attention") {
		t.Errorf("error = %q, want it to say the repository needs attention", err)
	}
}

func TestDoctorStrictExitsZeroWhenHealthy(t *testing.T) {
	repo := gixRepo(t)
	repo.Run("remote", "add", "origin", "https://example.invalid/gix.git")
	repo.Run("update-ref", "refs/remotes/origin/develop", "develop")
	repo.Run("config", "branch.develop.remote", "origin")
	repo.Run("config", "branch.develop.merge", "refs/heads/develop")

	out := run(t, repo, "doctor", "--strict")

	if !strings.Contains(out, "no problems found") {
		t.Errorf("a healthy repo should pass --strict:\n%s", out)
	}
}

// Without --strict, doctor reports and exits 0: deciding what blocks is
// the caller's call.
func TestDoctorExitsZeroByDefault(t *testing.T) {
	repo := gixRepo(t)
	repo.Write("wip.txt", "uncommitted\n")

	out := run(t, repo, "doctor")

	if !strings.Contains(out, "checks need attention") {
		t.Errorf("doctor should report the problem:\n%s", out)
	}
}

// --strict must not turn a warning-level problem into a fix, or a fix into
// a silent success.
func TestDoctorStrictWithFix(t *testing.T) {
	repo := gixRepo(t)
	repo.Run("remote", "add", "origin", "https://example.invalid/gix.git")
	repo.Run("update-ref", "refs/remotes/origin/develop", "develop")
	repo.Run("config", "branch.develop.remote", "origin")
	repo.Run("config", "branch.develop.merge", "refs/heads/develop")
	repo.Run("config", "--unset", "user.name")
	repo.Run("config", "--unset", "user.email")

	// The report is taken before the fix, so the missing identity is still
	// reported and --strict still fails.
	if _, err := exec(t, repo, "doctor", "--fix", "--strict",
		"--set-user-name=Test User", "--set-user-email=test@example.invalid"); err == nil {
		t.Error("`gix doctor --fix --strict` reported success despite the pre-fix problem")
	}
	if got := repo.Output("config", "--local", "user.email"); got != "test@example.invalid" {
		t.Fatalf("the identity was not written: %q", got)
	}

	// Once the problem is really gone, the same command passes.
	run(t, repo, "doctor", "--strict")
}

// unbornRepo has a valid .gix/config and a real commit on main, but HEAD
// points at an unborn branch: the shape a repository is in right after
// `git checkout --orphan`. Git cannot create a branch before any commit
// exists, so the history is created first and then orphaned.
func unbornRepo(t *testing.T) *testutil.Repo {
	t.Helper()

	dir := testutil.NewDir(t)
	dir.Run("init", "-b", "main")
	dir.Run("config", "user.name", "gix test")
	dir.Run("config", "user.email", "gix-test@example.invalid")

	if err := config.Save(dir.Root, config.Default()); err != nil {
		t.Fatalf("saving config: %v", err)
	}
	dir.Commit("chore(gix): initialize gix flow config")
	dir.Branch("develop")

	dir.Run("checkout", "--orphan", "scratch")

	return dir
}

func TestBranchDoctorOnAnUnbornBranch(t *testing.T) {
	dir := unbornRepo(t)

	out := run(t, dir, "branch", "doctor", "--all")

	if !strings.Contains(out, "BRANCH") {
		t.Errorf("`gix branch doctor` should still print its header:\n%s", out)
	}
}

func TestBranchPruneOnAnUnbornBranch(t *testing.T) {
	dir := unbornRepo(t)

	out := run(t, dir, "branch", "prune")

	if !strings.Contains(out, "nothing to prune") {
		t.Errorf("`gix branch prune` should report nothing to do:\n%s", out)
	}
}

// On an unborn HEAD the files left from the previous branch are
// uncommitted by definition, so gix must refuse with a reason rather than
// act on a repository it cannot reason about.
func TestCommandsOnAnUnbornBranch(t *testing.T) {
	dir := unbornRepo(t)

	runFailsWith(t, dir, "working tree is not clean", "feature", "start", "api")
	runFails(t, dir, "feature", "finish", "api")
}

func TestSyncOnAnUnbornBranch(t *testing.T) {
	dir := unbornRepo(t)

	out := run(t, dir, "branch", "sync", "--from=develop", "--dry-run")
	// There are no topic branches to sync, and that has to be said
	// plainly rather than crashing on an unresolvable HEAD.
	if !strings.Contains(out, "nothing to sync") {
		t.Errorf("a dry run with nothing to sync should say so:\n%s", out)
	}
}

func TestVersionShowOnAnUnbornBranch(t *testing.T) {
	dir := unbornRepo(t)

	out := run(t, dir, "version", "show")
	if !strings.Contains(out, "no release tags yet") {
		t.Errorf("`gix version show` should report no releases:\n%s", out)
	}
}

// A repository with no commits has no HEAD, so nothing that needs a branch
// may crash.
func TestCommandsWithNoCommitsDoNotPanic(t *testing.T) {
	dir := unbornRepo(t)

	for _, args := range [][]string{
		{"doctor"},
		{"status"},
		{"list"},
		{"config", "list"},
		{"signing", "status"},
		{"branch", "doctor", "--all"},
		{"branch", "prune"},
		{"branch", "sync", "--from=develop", "--dry-run"},
		{"version", "show"},
	} {
		run(t, dir, args...)
	}

	// Releasing needs the main branch, and says so rather than guessing.
	runFailsWith(t, dir, "releases are cut on main", "version", "bump", "minor", "--dry-run")
}

// Every command has to survive a repository whose config is present but
// describes branches that do not exist.
func TestCommandsWithAnUnusableFlow(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Run("remote", "add", "origin", "https://example.invalid/gix.git")

	run(t, repo, "flow", "init")
	repo.Checkout("main")
	repo.Run("branch", "-D", "develop")

	for _, args := range [][]string{
		{"doctor"},
		{"status"},
		{"list"},
		{"branch", "doctor", "--all"},
		{"branch", "prune"},
		{"branch", "sync", "--from=main", "--dry-run"},
		{"version", "show"},
	} {
		run(t, repo, args...)
	}
}
