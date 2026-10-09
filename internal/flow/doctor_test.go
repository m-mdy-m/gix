package flow

import (
	"strings"
	"testing"

	"github.com/m-mdy-m/gix/internal/config"
	"github.com/m-mdy-m/gix/internal/git"
	"github.com/m-mdy-m/gix/internal/testutil"
)

// find returns the named check, failing the test if it isn't there.
func find(t *testing.T, r Report, name string) Check {
	t.Helper()

	for _, c := range r.Checks {
		if c.Name == name {
			return c
		}
	}
	var names []string
	for _, c := range r.Checks {
		names = append(names, c.Name)
	}
	t.Fatalf("no %q check in the report, got %v", name, names)
	return Check{}
}

// A repo with a remote, an upstream, a flow config, and a clean tree
// should come back with nothing to report.
func TestCheckRepoHealthy(t *testing.T) {
	repo, _ := flowRepo(t)
	setUpstream(t, repo, "main", "origin/main")

	r := CheckRepo(repo.Root)

	if !r.OK() {
		for _, c := range r.Problems() {
			t.Errorf("%s is %v: %s", c.Name, c.Severity, c.Problem)
		}
	}
	if r.Count(Fail) != 0 {
		t.Errorf("Fail count = %d, want 0", r.Count(Fail))
	}
	if len(r.Checks) == 0 {
		t.Error("the report is empty, want every check to have run")
	}
}

func TestCheckRepoRunsFixedOrder(t *testing.T) {
	repo, _ := flowRepo(t)

	first := CheckRepo(repo.Root)
	second := CheckRepo(repo.Root)

	if len(first.Checks) != len(second.Checks) {
		t.Fatalf("check count changed between runs: %d then %d", len(first.Checks), len(second.Checks))
	}
	for i := range first.Checks {
		if first.Checks[i].Name != second.Checks[i].Name {
			t.Fatalf("check %d changed order: %q then %q", i, first.Checks[i].Name, second.Checks[i].Name)
		}
	}
}

func TestCheckRepoOutsideARepository(t *testing.T) {
	testutil.NewDir(t)

	r := CheckRepo("")

	repo := find(t, r, "repository")
	if repo.Severity != Fail {
		t.Errorf("repository severity = %v, want Fail outside a repo", repo.Severity)
	}
	if !strings.Contains(repo.Fix, "gix init") {
		t.Errorf("fix = %q, want it to point at `gix init`", repo.Fix)
	}
}

func TestCheckRepoWithoutGixConfig(t *testing.T) {
	repo := testutil.NewRepo(t)

	r := CheckRepo(repo.Root)

	c := find(t, r, "gix config")
	if c.Severity != Warn {
		t.Errorf("gix config severity = %v, want Warn when gix isn't initialized", c.Severity)
	}
	if !strings.Contains(c.Fix, "gix flow init") {
		t.Errorf("fix = %q, want it to point at `gix flow init`", c.Fix)
	}

	// Everything else should still be checked; a missing config must not
	// stop the rest of the report.
	if find(t, r, "identity").Severity == Fail {
		t.Error("identity check was skipped or failed just because gix isn't initialized")
	}
}

func TestCheckRepoWithInvalidConfig(t *testing.T) {
	repo := testutil.NewRepo(t)
	writeRawConfig(t, repo.Root, "gix:\n  version: \"1\"\nbranch: {}\n")

	r := CheckRepo(repo.Root)

	c := find(t, r, "gix config")
	if c.Severity != Fail {
		t.Errorf("gix config severity = %v, want Fail for an unreadable config", c.Severity)
	}
}

func TestCheckRepoIdentityMissing(t *testing.T) {
	repo, _ := flowRepo(t)
	repo.Run("config", "--unset", "user.name")
	repo.Run("config", "--unset", "user.email")

	r := CheckRepo(repo.Root)

	c := find(t, r, "identity")
	if c.Severity != Fail {
		t.Errorf("identity severity = %v, want Fail with no identity", c.Severity)
	}
	if !strings.Contains(c.Fix, "gix doctor --fix") {
		t.Errorf("fix = %q, want it to give the doctor --fix command", c.Fix)
	}
}

func TestCheckRepoIdentityPartiallyMissing(t *testing.T) {
	tests := []struct {
		name        string
		unset       string
		wantProblem string
	}{
		{name: "no email", unset: "user.email", wantProblem: "user.email is not set"},
		{name: "no name", unset: "user.name", wantProblem: "user.name is not set"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo, _ := flowRepo(t)
			repo.Run("config", "--unset", tc.unset)

			r := CheckRepo(repo.Root)

			c := find(t, r, "identity")
			if c.Severity != Fail {
				t.Errorf("identity severity = %v, want Fail", c.Severity)
			}
			if c.Problem != tc.wantProblem {
				t.Errorf("problem = %q, want %q", c.Problem, tc.wantProblem)
			}
		})
	}
}

func TestCheckRepoIdentityReportsScope(t *testing.T) {
	repo, _ := flowRepo(t)

	c := find(t, CheckRepo(repo.Root), "identity")

	if !strings.Contains(c.Value, "(local)") {
		t.Errorf("value = %q, want it to say where the identity came from", c.Value)
	}
}

// A name in one scope and an email in another works, but it's a trap: it
// breaks the moment one of them moves, and neither `git config` command
// shows both halves at once.
func TestCheckRepoIdentitySplitAcrossScopes(t *testing.T) {
	repo, _ := flowRepo(t)
	repo.Run("config", "--unset", "user.email")
	repo.Run("config", "--global", "user.email", "elsewhere@example.invalid")

	c := find(t, CheckRepo(repo.Root), "identity")

	if c.Severity != Warn {
		t.Errorf("identity severity = %v, want Warn for a split-scope identity", c.Severity)
	}
	if !strings.Contains(c.Problem, "global config") || !strings.Contains(c.Problem, "local config") {
		t.Errorf("problem = %q, want it to name both scopes involved", c.Problem)
	}
	if !strings.Contains(c.Fix, "--scope") {
		t.Errorf("fix = %q, want it to offer putting both in one scope", c.Fix)
	}
}

// An identity that is complete only because of global config still works,
// and doctor says so rather than claiming a problem.
func TestCheckRepoIdentityFromGlobalOnly(t *testing.T) {
	repo, _ := flowRepo(t)
	repo.Run("config", "--unset", "user.name")
	repo.Run("config", "--unset", "user.email")
	repo.Run("config", "--global", "user.name", "Global Name")
	repo.Run("config", "--global", "user.email", "global@example.invalid")

	c := find(t, CheckRepo(repo.Root), "identity")

	if c.Severity != OK {
		t.Errorf("identity severity = %v, want OK for a global identity, problem: %s", c.Severity, c.Problem)
	}
	if !strings.Contains(c.Value, "global") {
		t.Errorf("value = %q, want it to report the global scope", c.Value)
	}
}

func TestCheckRepoMissingRemote(t *testing.T) {
	repo, _ := flowRepo(t)

	c := find(t, CheckRepo(repo.Root), "remote")

	if c.Severity != Warn {
		t.Errorf("remote severity = %v, want Warn with no remotes", c.Severity)
	}
	if !strings.Contains(c.Fix, "git remote add origin") {
		t.Errorf("fix = %q, want the exact command to add a remote", c.Fix)
	}
}

func TestCheckRepoRenamedRemote(t *testing.T) {
	repo, _ := flowRepo(t)
	repo.Run("remote", "add", "upstream", "https://example.invalid/gix.git")

	c := find(t, CheckRepo(repo.Root), "remote")

	if c.Severity != Warn {
		t.Errorf("remote severity = %v, want Warn when origin is missing", c.Severity)
	}
	if !strings.Contains(c.Problem, `"origin"`) {
		t.Errorf("problem = %q, want it to name the remote gix expected", c.Problem)
	}
	if !strings.Contains(c.Value, "upstream") {
		t.Errorf("value = %q, want it to show which remotes do exist", c.Value)
	}
}

func TestCheckRepoRemoteFollowsConfig(t *testing.T) {
	repo, cfg := flowRepo(t)
	repo.Run("remote", "add", "mirror", "https://example.invalid/gix.git")
	cfg.Remote = "mirror"
	saveConfig(t, repo, cfg)

	c := find(t, CheckRepo(repo.Root), "remote")

	if c.Severity != OK {
		t.Errorf("remote severity = %v, want OK when the configured remote exists", c.Severity)
	}
	if c.Value != "mirror" {
		t.Errorf("value = %q, want %q from .gix/config", c.Value, "mirror")
	}
}

func TestCheckRepoNoUpstream(t *testing.T) {
	repo, _ := flowRepo(t)

	c := find(t, CheckRepo(repo.Root), "upstream")

	if c.Severity != Warn {
		t.Errorf("upstream severity = %v, want Warn for a branch that tracks nothing", c.Severity)
	}
	if !strings.Contains(c.Fix, "git push -u") {
		t.Errorf("fix = %q, want the command to start tracking", c.Fix)
	}
}

func TestCheckRepoWithUpstream(t *testing.T) {
	repo, _ := flowRepo(t)
	setUpstream(t, repo, "main", "origin/main")

	c := find(t, CheckRepo(repo.Root), "upstream")

	if c.Severity != OK {
		t.Errorf("upstream severity = %v, want OK, problem: %s", c.Severity, c.Problem)
	}
	if c.Value != "main -> origin/main" {
		t.Errorf("value = %q, want it to show the tracking pair", c.Value)
	}
}

func TestCheckRepoDetachedHead(t *testing.T) {
	repo, _ := flowRepo(t)
	repo.Run("checkout", "--detach")

	r := CheckRepo(repo.Root)

	head := find(t, r, "HEAD")
	if head.Severity != Warn {
		t.Errorf("HEAD severity = %v, want Warn on a detached HEAD", head.Severity)
	}
	if !strings.Contains(head.Fix, "git checkout") {
		t.Errorf("fix = %q, want it to explain how to get back on a branch", head.Fix)
	}

	// There's no branch, so upstream has nothing to report.
	if up := find(t, r, "upstream"); up.Severity != OK {
		t.Errorf("upstream severity = %v, want OK (not applicable) on a detached HEAD", up.Severity)
	}
}

func TestCheckRepoUnbornBranch(t *testing.T) {
	dir := testutil.NewDir(t)
	dir.Run("init", "-b", "main")

	head := find(t, CheckRepo(dir.Root), "HEAD")

	if head.Severity != Warn {
		t.Errorf("HEAD severity = %v, want Warn with no commits yet", head.Severity)
	}
	if !strings.Contains(head.Problem, "no commits") {
		t.Errorf("problem = %q, want it to say there are no commits", head.Problem)
	}
}

func TestCheckRepoDirtyWorkingTree(t *testing.T) {
	repo, _ := flowRepo(t)
	repo.Write("wip.txt", "wip\n")

	c := find(t, CheckRepo(repo.Root), "working tree")

	if c.Severity != Warn {
		t.Errorf("working tree severity = %v, want Warn when dirty", c.Severity)
	}
	if !strings.Contains(c.Fix, "stash") {
		t.Errorf("fix = %q, want it to suggest committing or stashing", c.Fix)
	}
}

func TestCheckRepoUnfinishedOperation(t *testing.T) {
	repo, _ := flowRepo(t)
	repo.CommitOn("feature/api", "shared.txt", "topic\n", "feat: shared")
	repo.Checkout("develop")
	repo.CommitOn("develop", "shared.txt", "develop\n", "chore: shared")
	repo.Checkout("feature/api")
	if _, err := repo.Try("merge", "develop"); err == nil {
		t.Fatal("git merge succeeded on conflicting content, want a conflict")
	}

	c := find(t, CheckRepo(repo.Root), "in-progress operation")

	if c.Severity != Warn {
		t.Errorf("severity = %v, want Warn during a merge", c.Severity)
	}
	if c.Value != string(git.OpMerge) {
		t.Errorf("value = %q, want %q", c.Value, git.OpMerge)
	}
	if !strings.Contains(c.Fix, "--abort") {
		t.Errorf("fix = %q, want it to say how to get out of the merge", c.Fix)
	}
}

func TestCheckRepoMissingBaseBranch(t *testing.T) {
	repo, _ := flowRepo(t)
	repo.Checkout("main")
	repo.Run("branch", "-D", "develop")

	c := find(t, CheckRepo(repo.Root), "flow")

	if c.Severity != Fail {
		t.Errorf("flow severity = %v, want Fail when a configured base is missing", c.Severity)
	}
	if c.Value != "develop" {
		t.Errorf("value = %q, want it to name the missing branch", c.Value)
	}
}

func TestCheckRepoSigningOff(t *testing.T) {
	repo, _ := flowRepo(t)

	c := find(t, CheckRepo(repo.Root), "signing")

	if c.Severity != OK {
		t.Errorf("signing severity = %v, want OK when signing isn't configured", c.Severity)
	}
	if c.Value != "off" {
		t.Errorf("value = %q, want %q", c.Value, "off")
	}
}

func TestCheckRepoSigningProgramMissing(t *testing.T) {
	repo, _ := flowRepo(t)
	repo.Run("config", "commit.gpgsign", "true")
	repo.Run("config", "user.signingkey", "ABCDEF0123456789")
	repo.Run("config", "gpg.program", "gix-nonexistent-signer")

	c := find(t, CheckRepo(repo.Root), "signing")

	if c.Severity != Warn {
		t.Errorf("signing severity = %v, want Warn when the program is missing", c.Severity)
	}
	if !strings.Contains(c.Problem, "gix-nonexistent-signer") {
		t.Errorf("problem = %q, want it to name the missing program", c.Problem)
	}
}

func TestReportProblems(t *testing.T) {
	r := Report{Checks: []Check{
		{Name: "a", Severity: OK},
		{Name: "b", Severity: Warn},
		{Name: "c", Severity: Fail},
		{Name: "d", Severity: OK},
	}}

	if r.OK() {
		t.Error("OK() = true with problems present")
	}
	if got := r.Count(Warn); got != 1 {
		t.Errorf("Count(Warn) = %d, want 1", got)
	}
	if got := r.Count(Fail); got != 1 {
		t.Errorf("Count(Fail) = %d, want 1", got)
	}

	problems := r.Problems()
	if len(problems) != 2 || problems[0].Name != "b" || problems[1].Name != "c" {
		t.Errorf("Problems() = %+v, want b and c in report order", problems)
	}
}

func TestReportOKWhenEmpty(t *testing.T) {
	var r Report

	if !r.OK() {
		t.Error("OK() = false on an empty report")
	}
	if got := r.Problems(); got != nil {
		t.Errorf("Problems() = %v, want nil on an empty report", got)
	}
}

// No check may ever recommend something destructive: git history and
// uncommitted work are the user's, not gix's.
func TestNoCheckSuggestsHistoryRewrite(t *testing.T) {
	repo, _ := flowRepo(t)

	banned := []string{"reset --hard", "push --force", "push -f", "clean -fd", "checkout .", "stash drop", "gc --prune"}
	for _, c := range CheckRepo(repo.Root).Checks {
		for _, b := range banned {
			if strings.Contains(c.Fix, b) {
				t.Errorf("%s suggests %q, which can destroy work or history", c.Name, b)
			}
		}
	}
}

func saveConfig(t *testing.T, repo *testutil.Repo, cfg *config.Config) {
	t.Helper()

	if err := config.Save(repo.Root, cfg); err != nil {
		t.Fatalf("saving config: %v", err)
	}
	repo.Commit("chore(gix): update flow config")
}

// setUpstream makes a branch track a remote branch without needing a real
// remote to push to.
func setUpstream(t *testing.T, repo *testutil.Repo, branch, upstream string) {
	t.Helper()

	remote, remoteBranch, ok := strings.Cut(upstream, "/")
	if !ok {
		t.Fatalf("upstream %q is not in remote/branch form", upstream)
	}
	if _, err := repo.Try("rev-parse", "--verify", remoteBranch); err != nil {
		repo.Run("branch", remoteBranch)
	}

	repo.Run("remote", "add", remote, "https://example.invalid/"+remote+".git")
	repo.Run("update-ref", "refs/remotes/"+upstream, remoteBranch)
	repo.Run("config", "branch."+branch+".remote", remote)
	repo.Run("config", "branch."+branch+".merge", "refs/heads/"+remoteBranch)
}

func TestCheckRepoReportsGitVersion(t *testing.T) {
	repo, _ := flowRepo(t)

	c := find(t, CheckRepo(repo.Root), "git")
	if c.Value == "" && c.Severity == OK {
		t.Errorf("git check = %+v, want the installed version as its value", c)
	}
}
