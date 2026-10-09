package commands

import (
	"strings"
	"testing"

	"github.com/m-mdy-m/gix/internal/testutil"
)

// trackedBranch makes a branch look like it tracks origin/<branch>.
func trackedBranch(t *testing.T, repo *testutil.Repo, branch string) {
	t.Helper()

	if _, err := repo.Try("remote", "add", "origin", "https://example.invalid/gix.git"); err != nil {
		out, outErr := repo.Try("remote")
		if outErr != nil || !remoteListed(out, "origin") {
			t.Fatalf("adding remote: %v\n%s", outErr, out)
		}
	}
	repo.Run("update-ref", "refs/remotes/origin/"+branch, branch)
	repo.Run("config", "branch."+branch+".remote", "origin")
	repo.Run("config", "branch."+branch+".merge", "refs/heads/"+branch)
}

func remoteListed(out, name string) bool {
	for _, line := range strings.Split(out, "\n") {
		if line == name {
			return true
		}
	}
	return false
}

func TestBranchDoctorShowsAHeaderAndRows(t *testing.T) {
	repo := gixRepo(t)

	out := run(t, repo, "branch", "doctor", "--all")

	for _, want := range []string{"BRANCH", "KIND", "UPSTREAM", "AHEAD", "BEHIND", "STATE", "develop", "base", "untracked"} {
		if !strings.Contains(out, want) {
			t.Errorf("`gix branch doctor --all` output is missing %q:\n%s", want, out)
		}
	}
}

func TestBranchDoctorMarksTheCurrentBranch(t *testing.T) {
	repo := gixRepo(t)

	out := run(t, repo, "branch", "doctor", "--all")

	if !strings.Contains(out, "develop (current)") {
		t.Errorf("`gix branch doctor` should mark the branch you are on:\n%s", out)
	}
}

func TestBranchDoctorShowsLastCommitAge(t *testing.T) {
	repo := gixRepo(t)

	out := run(t, repo, "branch", "doctor", "--all")

	if !strings.Contains(out, "LAST COMMIT") {
		t.Errorf("`gix branch doctor` should show a LAST COMMIT column:\n%s", out)
	}
	if !strings.Contains(out, "today") {
		t.Errorf("`gix branch doctor` should report a just-made commit as today:\n%s", out)
	}
}

// gix reports the age and leaves the "is this stale?" judgement to the
// user, so it must not label anything on its own.
func TestBranchDoctorDoesNotLabelBranchesStale(t *testing.T) {
	repo := gixRepo(t)

	out := run(t, repo, "branch", "doctor", "--all")

	if strings.Contains(strings.ToLower(out), "stale") {
		t.Errorf("`gix branch doctor` decided a branch is stale on its own:\n%s", out)
	}
}

func TestBranchDoctorHidesHealthyBranchesByDefault(t *testing.T) {
	repo := gixRepo(t)
	trackedBranch(t, repo, "develop")
	trackedBranch(t, repo, "main")

	out := run(t, repo, "branch", "doctor")

	if strings.Contains(out, "develop (current)") {
		t.Errorf("`gix branch doctor` should omit a healthy branch by default:\n%s", out)
	}
	if !strings.Contains(out, "every branch is in sync") {
		t.Errorf("`gix branch doctor` should say so instead of printing nothing:\n%s", out)
	}
	if !strings.Contains(out, "--all") {
		t.Errorf("`gix branch doctor` should point at --all:\n%s", out)
	}
}

func TestBranchDoctorBehindAndAhead(t *testing.T) {
	repo := gixRepo(t)
	repo.Branch("feature/api")
	trackedBranch(t, repo, "feature/api")

	// develop moves on, so the feature branch falls behind.
	repo.Write("develop.txt", "develop\n")
	repo.Commit("chore: develop moves")
	repo.Run("update-ref", "refs/remotes/origin/feature/api", "develop")
	repo.Checkout("feature/api")
	repo.Write("api.txt", "api\n")
	repo.Commit("feat: api")

	out := run(t, repo, "branch", "doctor")

	if !strings.Contains(out, "diverged") {
		t.Errorf("`gix branch doctor` should report a diverged branch:\n%s", out)
	}
	if !strings.Contains(out, "feature") {
		t.Errorf("`gix branch doctor` should name the branch and its kind:\n%s", out)
	}
	if !strings.Contains(out, "origin/feature/api") {
		t.Errorf("`gix branch doctor` should show the upstream:\n%s", out)
	}
	if !strings.Contains(out, "resolve that before pushing") {
		t.Errorf("`gix branch doctor` should explain what to do about divergence:\n%s", out)
	}
}

func TestBranchDoctorGoneUpstream(t *testing.T) {
	repo := gixRepo(t)
	repo.Branch("feature/fire")
	trackedBranch(t, repo, "feature/fire")
	repo.Run("update-ref", "-d", "refs/remotes/origin/feature/fire")

	out := run(t, repo, "branch", "doctor")

	if !strings.Contains(out, "gone") {
		t.Errorf("`gix branch doctor` should report a gone upstream:\n%s", out)
	}
	if !strings.Contains(out, "deleted or not fetched") {
		t.Errorf("`gix branch doctor` should explain a gone upstream:\n%s", out)
	}
}

func TestBranchDoctorReportsMergedBranches(t *testing.T) {
	repo := gixRepo(t)

	run(t, repo, "feature", "start", "api")
	repo.Write("api.txt", "api\n")
	repo.Commit("feat: api")
	run(t, repo, "feature", "finish", "api", "--no-delete")

	out := run(t, repo, "branch", "doctor", "--all")

	if !strings.Contains(out, "already merged into develop") {
		t.Errorf("`gix branch doctor` should say the branch is merged:\n%s", out)
	}
	if !strings.Contains(out, "branch prune") {
		t.Errorf("`gix branch doctor` should point at `gix branch prune`:\n%s", out)
	}
}

func TestBranchDoctorNeedsGixConfig(t *testing.T) {
	repo := testutil.NewRepo(t)

	runFailsWith(t, repo, "gix is not initialized", "branch", "doctor")
	runFailsWith(t, repo, "gix is not initialized", "branch", "prune")
}

func TestBranchDoctorRejectsExtraArgs(t *testing.T) {
	repo := gixRepo(t)

	runFails(t, repo, "branch", "doctor", "unexpected")
	runFails(t, repo, "branch", "prune", "unexpected")
}

func TestBranchPruneOnAFreshFlow(t *testing.T) {
	repo := gixRepo(t)

	out := run(t, repo, "branch", "prune")

	if !strings.Contains(out, "nothing to prune") {
		t.Errorf("`gix branch prune` on a fresh flow should say so:\n%s", out)
	}
}

func TestBranchPruneListsMergedBranch(t *testing.T) {
	repo := gixRepo(t)

	run(t, repo, "feature", "start", "api")
	repo.Write("api.txt", "api\n")
	repo.Commit("feat: api")
	run(t, repo, "feature", "finish", "api", "--no-delete")

	out := run(t, repo, "branch", "prune")

	if !strings.Contains(out, "feature/api") {
		t.Errorf("`gix branch prune` should list the merged branch:\n%s", out)
	}
	if !strings.Contains(out, "already merged into develop") {
		t.Errorf("`gix branch prune` should say why:\n%s", out)
	}
	if !strings.Contains(out, "gix has not deleted anything") {
		t.Errorf("`gix branch prune` must state that nothing was deleted:\n%s", out)
	}
	if !strings.Contains(out, "BRANCH") || !strings.Contains(out, "WHY") {
		t.Errorf("`gix branch prune` should print a table:\n%s", out)
	}
}

// A preview that deleted something would be the worst possible bug in this
// command, so check the branches survive it.
func TestBranchPruneDeletesNothing(t *testing.T) {
	repo := gixRepo(t)

	run(t, repo, "feature", "start", "api")
	repo.Write("api.txt", "api\n")
	repo.Commit("feat: api")
	run(t, repo, "feature", "finish", "api", "--no-delete")

	before := repo.Output("for-each-ref", "--format=%(refname)", "refs/heads/")

	run(t, repo, "branch", "prune")
	run(t, repo, "branch", "doctor", "--all")

	if after := repo.Output("for-each-ref", "--format=%(refname)", "refs/heads/"); after != before {
		t.Errorf("branches changed:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestBranchPruneLeavesUnmergedBranchesAlone(t *testing.T) {
	repo := gixRepo(t)

	run(t, repo, "feature", "start", "wip")
	repo.Write("wip.txt", "work\n")
	repo.Commit("feat: wip")

	out := run(t, repo, "branch", "prune")

	if strings.Contains(out, "feature/wip") {
		t.Errorf("`gix branch prune` listed a branch with unmerged work:\n%s", out)
	}
	if !strings.Contains(out, "nothing to prune") {
		t.Errorf("`gix branch prune` should report nothing to do:\n%s", out)
	}
}

func TestBranchPruneNeverListsBaseBranches(t *testing.T) {
	repo := gixRepo(t)
	run(t, repo, "feature", "start", "api")
	repo.Write("api.txt", "api\n")
	repo.Commit("feat: api")
	run(t, repo, "feature", "finish", "api", "--no-delete")

	out := run(t, repo, "branch", "prune")
	if !strings.Contains(out, "feature/api") {
		t.Fatalf("setup failed, expected a prunable branch:\n%s", out)
	}

	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "develop") || strings.HasPrefix(strings.TrimSpace(line), "main") {
			t.Errorf("`gix branch prune` listed a base branch: %q", line)
		}
	}
}

// The preview must never delete, and must say what --apply would do.
func TestBranchPrunePreviewStaysAPreview(t *testing.T) {
	repo := gixRepo(t)
	run(t, repo, "feature", "start", "api")
	repo.Write("api.txt", "api\n")
	repo.Commit("feat: api")
	run(t, repo, "feature", "finish", "api", "--no-delete")

	before := repo.Output("for-each-ref", "--format=%(refname)", "refs/heads/")

	out := run(t, repo, "branch", "prune")

	if !strings.Contains(out, "has not deleted anything") {
		t.Errorf("the preview should say it deleted nothing:\n%s", out)
	}
	if !strings.Contains(out, "--apply") {
		t.Errorf("the preview should point at --apply:\n%s", out)
	}
	if after := repo.Output("for-each-ref", "--format=%(refname)", "refs/heads/"); after != before {
		t.Errorf("the preview deleted branches:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestBranchPruneApplyDeletesMerged(t *testing.T) {
	repo := gixRepo(t)
	run(t, repo, "feature", "start", "api")
	repo.Write("api.txt", "api\n")
	repo.Commit("feat: api")
	run(t, repo, "feature", "finish", "api", "--no-delete")

	out := run(t, repo, "branch", "prune", "--apply")

	if !strings.Contains(out, "deleted feature/api") {
		t.Errorf("`--apply` should report the deletion:\n%s", out)
	}
	if _, err := repo.Try("show-ref", "--verify", "--quiet", "refs/heads/feature/api"); err == nil {
		t.Error("the merged branch still exists after --apply")
	}
	if !repo.Clean() {
		t.Errorf("prune left the tree dirty:\n%s", repo.Output("status", "--porcelain"))
	}
}

// --apply must never force anything: an unmerged branch stays.
func TestBranchPruneApplyRefusesUnmergedWork(t *testing.T) {
	repo := gixRepo(t)
	run(t, repo, "feature", "start", "wip")
	repo.Write("wip.txt", "work in progress\n")
	repo.Commit("feat: wip")

	before := repo.Output("rev-parse", "feature/wip")

	exec(t, repo, "branch", "prune", "--apply")

	if _, err := repo.Try("show-ref", "--verify", "--quiet", "refs/heads/feature/wip"); err != nil {
		t.Fatal("an unmerged branch was deleted")
	}
	if got := repo.Output("rev-parse", "feature/wip"); got != before {
		t.Error("the unmerged branch moved")
	}
}

func TestBranchPruneApplyRefusesADirtyTree(t *testing.T) {
	repo := gixRepo(t)
	run(t, repo, "feature", "start", "api")
	repo.Write("api.txt", "api\n")
	repo.Commit("feat: api")
	run(t, repo, "feature", "finish", "api", "--no-delete")
	repo.Write("wip.txt", "uncommitted\n")

	runFailsWith(t, repo, "working tree is not clean", "branch", "prune", "--apply")

	if _, err := repo.Try("show-ref", "--verify", "--quiet", "refs/heads/feature/api"); err != nil {
		t.Error("a branch was deleted despite the refusal")
	}
}

func TestBranchPruneApplyKeepsGoneUpstream(t *testing.T) {
	repo := gixRepo(t)
	run(t, repo, "feature", "start", "orphan")
	repo.Write("orphan.txt", "work\n")
	repo.Commit("feat: orphan")
	trackedBranch(t, repo, "feature/orphan")
	repo.Run("update-ref", "-d", "refs/remotes/origin/feature/orphan")

	exec(t, repo, "branch", "prune", "--apply")

	if _, err := repo.Try("show-ref", "--verify", "--quiet", "refs/heads/feature/orphan"); err != nil {
		t.Fatal("a branch with a gone upstream was deleted, which is not safe")
	}
}

func TestBranchPruneApplyIsIdempotent(t *testing.T) {
	repo := gixRepo(t)
	run(t, repo, "feature", "start", "api")
	repo.Write("api.txt", "api\n")
	repo.Commit("feat: api")
	run(t, repo, "feature", "finish", "api", "--no-delete")

	run(t, repo, "branch", "prune", "--apply")

	out := run(t, repo, "branch", "prune", "--apply")
	if !strings.Contains(out, "nothing to prune") {
		t.Errorf("a second --apply should find nothing left:\n%s", out)
	}
}

// --apply deletes merged branches; there must be no way to force anything.
func TestBranchPruneHasNoForceFlag(t *testing.T) {
	repo := gixRepo(t)

	runFails(t, repo, "branch", "prune", "--force")
	runFails(t, repo, "branch", "prune", "-D")
}

func TestBranchGroupIsRegistered(t *testing.T) {
	root := New()

	for _, c := range root.Commands() {
		if c.Name() != "branch" {
			continue
		}
		if c.Short == "" || c.Long == "" {
			t.Error("`gix branch` is missing help text")
		}
		want := map[string]bool{"doctor": false, "prune": false}
		for _, sub := range c.Commands() {
			if _, ok := want[sub.Name()]; ok {
				want[sub.Name()] = true
			}
			if sub.Short == "" {
				t.Errorf("`gix branch %s` has no Short description", sub.Name())
			}
		}
		for name, found := range want {
			if !found {
				t.Errorf("`gix branch %s` is not registered", name)
			}
		}
		return
	}
	t.Error("`gix branch` is not registered")
}

func TestBranchCommandsHaveHelp(t *testing.T) {
	repo := gixRepo(t)

	for _, args := range [][]string{
		{"branch", "--help"},
		{"branch", "doctor", "--help"},
		{"branch", "prune", "--help"},
	} {
		run(t, repo, args...)
	}
}

// Inspection must be safe to run while you work.
func TestBranchCommandsAreReadOnly(t *testing.T) {
	repo := gixRepo(t)
	run(t, repo, "feature", "start", "api")
	repo.Write("api.txt", "api\n")
	repo.Commit("feat: api")

	before := repo.Output("for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/")
	head := repo.Output("rev-parse", "HEAD")

	run(t, repo, "branch", "doctor", "--all")
	run(t, repo, "branch", "prune")

	if after := repo.Output("for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/"); after != before {
		t.Errorf("inspecting moved or created a branch:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if after := repo.Output("rev-parse", "HEAD"); after != head {
		t.Error("inspecting moved HEAD")
	}
}
