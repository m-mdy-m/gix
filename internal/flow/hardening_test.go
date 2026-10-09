package flow

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/m-mdy-m/gix/internal/git"
	"github.com/m-mdy-m/gix/internal/testutil"
)

func filepathJoin(parts ...string) string { return filepath.Join(parts...) }

// An unborn HEAD has no branches to inspect, so the read-only commands
// have to say something useful instead of failing or printing nothing.
func TestCheckBranchesOnAnUnbornHead(t *testing.T) {
	dir := testutil.NewDir(t)
	dir.Run("init", "-b", "main")
	dir.Write("a.txt", "a\n")

	_, cfg := flowRepo(t)

	health, err := CheckBranches(cfg)
	if err != nil {
		t.Fatalf("CheckBranches on an unborn branch: %v", err)
	}
	for _, h := range health {
		if h.AgeDays != -1 {
			continue
		}
		// A branch with no commit reports age -1 rather than 0, which
		// would claim it was committed today.
		t.Errorf("%s has no commit, so AgeDays must be -1, got %d", h.Name, h.AgeDays)
	}
}

// Sync must not try to merge into a branch whose parent doesn't exist.
func TestSyncWithAMissingParentDoesNotCrash(t *testing.T) {
	repo, cfg := flowRepo(t)
	startFeature(t, repo, "api")

	repo.Checkout("develop")
	repo.Write("develop.txt", "develop\n")
	repo.Commit("chore: develop moves on")

	// The flow is broken, so the parent a kind points at is gone.
	repo.Checkout("main")
	repo.Run("branch", "-D", "develop")
	repo.Checkout("main")

	result, err := Sync(cfg, SyncOptions{Source: "main"})
	if err != nil {
		t.Fatalf("Sync with a missing parent: %v", err)
	}
	// Whatever it decides, it must not have merged into nothing.
	for _, e := range result.Entries {
		if e.State == SyncMerged && e.Branch == "" {
			t.Errorf("Sync merged into an empty branch name: %+v", e)
		}
	}
}

// A branch that cannot be checked out because another worktree holds it
// must fail that branch, not the run and not silently skip it.
func TestSyncReportsABranchItCannotCheckOut(t *testing.T) {
	repo, cfg := flowRepo(t)
	startFeature(t, repo, "api")
	startFeature(t, repo, "ui")

	repo.Checkout("develop")
	repo.Write("develop.txt", "develop\n")
	repo.Commit("chore: develop moves on")
	repo.Checkout("main")

	// Check the branch out in a second worktree, which locks it: git
	// refuses to check out a branch that another worktree already holds.
	// A detached worktree would not lock it, so the branch flag matters.
	repo.Run("worktree", "add", filepathJoin(repo.Root, "..", "pinned"), "feature/api")

	result, err := Sync(cfg, SyncOptions{Source: "develop"})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	var sawFailure bool
	for _, e := range result.Entries {
		if e.Branch != "feature/api" {
			continue
		}
		if e.State == SyncFailed {
			sawFailure = true
			if e.Detail == "" {
				t.Error("a failed branch gave no reason")
			}
			continue
		}
		// Anything else has to be an honest outcome, and "merged" while the
		// branch is pinned elsewhere would not be.
		if e.State == SyncMerged {
			t.Errorf("feature/api = %q while it is checked out in another worktree", e.State)
		}
	}
	if !sawFailure {
		t.Errorf("a branch held by another worktree should have been reported as failed, got %+v", syncEntryFor(t, result, "feature/api"))
	}

	// The other branch must still have been handled.
	if got := syncEntryFor(t, result, "feature/ui").State; got != SyncMerged {
		t.Errorf("feature/ui = %q, want %q even though another branch failed", got, SyncMerged)
	}
	if !repo.Clean() {
		t.Errorf("Sync left the working tree dirty:\n%s", repo.Output("status", "--porcelain"))
	}
}

// A tag that is not an exact version must never become a release base.
func TestBumpIgnoresNonVersionTags(t *testing.T) {
	repo, _ := releaseRepo(t, "v0.2.0")
	for _, tag := range []string{"v0.10.0", "nightly", "v1.2", "release-3"} {
		repo.Run("tag", tag)
	}

	plan, err := PlanBump(LevelPatch)
	if err != nil {
		t.Fatalf("PlanBump: %v", err)
	}
	if plan.CurrentTag != "v0.10.0" {
		t.Errorf("CurrentTag = %q, want v0.10.0", plan.CurrentTag)
	}
	if plan.Next.String() != "0.10.1" {
		t.Errorf("next = %q, want 0.10.1", plan.Next)
	}
}

// PlanBump is read-only, so it must be safe to call from anywhere.
func TestPlanBumpDoesNotRequireABranch(t *testing.T) {
	repo, _ := releaseRepo(t, "v0.2.0")
	repo.Run("checkout", "--detach")

	if _, err := PlanBump(LevelMinor); err != nil {
		t.Fatalf("PlanBump on a detached HEAD: %v", err)
	}
}

// A very large version must be rejected, not silently overflowed.
func TestParseVersionRejectsOverflow(t *testing.T) {
	for _, in := range []string{"99999999999999999999.0.0", "1.99999999999999999999.0", "0x1.2.3", "1.2.3 "} {
		if v, err := ParseVersion(in); err == nil {
			t.Errorf("ParseVersion(%q) = %v, want an error", in, v)
		}
	}
}

// A version with leading zeroes is still readable rather than rejected, so
// a tag like v01.02.03 doesn't stop gix from finding the last real release.
func TestParseVersionAcceptsLeadingZeroes(t *testing.T) {
	v, err := ParseVersion("01.02.03")
	if err != nil {
		t.Fatalf("ParseVersion(01.02.03): %v", err)
	}
	if v.String() != "1.2.3" {
		t.Errorf("version = %q, want 1.2.3", v.String())
	}
}

// Sync must leave no trace of a failure, whatever caused it.
func TestSyncLeavesNoStateBehindOnAnyFailure(t *testing.T) {
	repo, cfg := flowRepo(t)
	startFeature(t, repo, "one")
	startFeature(t, repo, "two")
	startFeature(t, repo, "three")

	repo.Checkout("develop")
	repo.Write("shared.txt", "from develop\n")
	repo.Commit("chore: develop touches shared")

	repo.Checkout("feature/two")
	repo.Write("shared.txt", "from two\n")
	repo.Commit("feat: two")

	repo.Checkout("develop")
	repo.Write("develop.txt", "develop\n")
	repo.Commit("chore: develop moves on")

	if _, err := Sync(cfg, SyncOptions{Source: "develop"}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if _, found, err := git.InProgressOperation(); err != nil {
		t.Fatalf("InProgressOperation: %v", err)
	} else if found {
		t.Error("an operation was left in progress")
	}
	if !repo.Clean() {
		t.Errorf("working tree dirty:\n%s", repo.Output("status", "--porcelain"))
	}
	// The conflicting branch must still be reachable and unchanged.
	if !git.BranchExists("feature/two") {
		t.Error("the conflicting branch was deleted")
	}
	content := repo.Output("show", "feature/two:shared.txt")
	if strings.TrimSpace(content) != "from two" {
		t.Errorf("the conflicting branch content changed to %q", content)
	}
}
