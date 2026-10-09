package flow

import (
	"strings"
	"testing"

	"github.com/m-mdy-m/gix/internal/config"
	"github.com/m-mdy-m/gix/internal/git"
	"github.com/m-mdy-m/gix/internal/testutil"
)

// mergedForPrune is a feature branch that has been finished into develop
// but kept, which is the case `gix branch prune --apply` exists for.
func mergedForPrune(t *testing.T, short string) (*testutil.Repo, *config.Config) {
	t.Helper()

	repo, cfg := flowRepo(t)
	if _, err := Start(cfg, "feature", short); err != nil {
		t.Fatalf("Start: %v", err)
	}
	repo.Write(short+".txt", "work\n")
	repo.Commit("feat: " + short)
	if _, err := Finish(cfg, "feature", short, FinishOptions{DeleteBranch: false}); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	repo.Checkout("develop")
	return repo, cfg
}

func TestApplyPruneDeletesMergedBranches(t *testing.T) {
	repo, cfg := mergedForPrune(t, "api")

	health, err := CheckBranches(cfg)
	if err != nil {
		t.Fatalf("CheckBranches: %v", err)
	}

	result, err := ApplyPrune(PlanPrune(cfg, health, "develop"))
	if err != nil {
		t.Fatalf("ApplyPrune: %v", err)
	}

	if len(result.Kept) != 0 {
		t.Errorf("Kept = %v, want nothing left behind", result.Kept)
	}
	if git.BranchExists("feature/api") {
		t.Error("the merged branch was not deleted")
	}
	if !git.BranchExists("develop") {
		t.Error("the base branch was deleted")
	}
	if !repo.Clean() {
		t.Errorf("prune left the tree dirty:\n%s", repo.Output("status", "--porcelain"))
	}
}

// An unmerged branch must never be a candidate, whatever it looks like.
func TestApplyPruneLeavesUnmergedWorkAlone(t *testing.T) {
	repo, cfg := flowRepo(t)
	startFeature(t, repo, "api")
	repo.Checkout("develop")
	repo.Branch("feature/abandoned")
	repo.Checkout("feature/abandoned")
	repo.Write("abandoned.txt", "work in progress\n")
	repo.Commit("feat: abandoned")
	repo.Checkout("develop")

	before := repo.Output("rev-parse", "feature/abandoned")

	health, err := CheckBranches(cfg)
	if err != nil {
		t.Fatalf("CheckBranches: %v", err)
	}

	result, err := ApplyPrune(PlanPrune(cfg, health, "develop"))
	if err != nil {
		t.Fatalf("ApplyPrune: %v", err)
	}

	if !git.BranchExists("feature/abandoned") {
		t.Fatal("an unmerged branch was deleted")
	}
	if got := repo.Output("rev-parse", "feature/abandoned"); got != before {
		t.Error("the unmerged branch moved")
	}
	for _, o := range result.Outcomes {
		if o.Name == "feature/abandoned" {
			t.Errorf("%+v, want the unmerged branch left out entirely", o)
		}
	}
}

// A branch whose upstream vanished is not necessarily merged, so it is
// listed but must never be deleted.
func TestApplyPruneKeepsBranchesWithAGoneUpstream(t *testing.T) {
	repo, cfg := flowRepo(t)
	repo.Branch("feature/orphan")
	trackBranch(t, repo, "feature/orphan", "origin", "feature/orphan")
	repo.Run("update-ref", "-d", "refs/remotes/origin/feature/orphan")

	health, err := CheckBranches(cfg)
	if err != nil {
		t.Fatalf("CheckBranches: %v", err)
	}

	plan := PlanPrune(cfg, health, "develop")
	if len(plan.Entries) != 1 || plan.Entries[0].Name != "feature/orphan" {
		t.Fatalf("Entries = %+v, want the orphan listed", plan.Entries)
	}
	if plan.Entries[0].Merged {
		t.Error("a branch with a gone upstream was marked merged")
	}

	result, err := ApplyPrune(plan)
	if err != nil {
		t.Fatalf("ApplyPrune: %v", err)
	}

	if !git.BranchExists("feature/orphan") {
		t.Fatal("a branch with a gone upstream was deleted, which is not safe")
	}
	if len(result.Kept) != 1 || result.Kept[0] != "feature/orphan" {
		t.Errorf("Kept = %v, want the orphan reported as kept", result.Kept)
	}
	if result.Outcomes[0].Detail == "" {
		t.Error("a kept branch gave no explanation")
	}
}

func TestApplyPruneRefusesADirtyTree(t *testing.T) {
	repo, cfg := mergedForPrune(t, "api")
	repo.Write("wip.txt", "uncommitted\n")

	health, err := CheckBranches(cfg)
	if err != nil {
		t.Fatalf("CheckBranches: %v", err)
	}

	_, err = ApplyPrune(PlanPrune(cfg, health, "develop"))
	if err == nil {
		t.Fatal("ApplyPrune succeeded on a dirty tree, want an error")
	}
	if !strings.Contains(err.Error(), "working tree is not clean") {
		t.Errorf("ApplyPrune = %q, want it to refuse for the dirty tree", err)
	}
	if !git.BranchExists("feature/api") {
		t.Error("a branch was deleted despite the refusal")
	}
}

func TestApplyPruneRefusesDuringAnUnfinishedMerge(t *testing.T) {
	repo, _ := flowRepo(t)
	startFeature(t, repo, "api")
	repo.Write("shared.txt", "feature\n")
	repo.Commit("feat: api")
	repo.Checkout("develop")
	repo.Write("shared.txt", "develop\n")
	repo.Commit("chore: develop")
	repo.Checkout("feature/api")
	if _, err := repo.Try("merge", "develop"); err == nil {
		t.Fatal("git merge succeeded on conflicting content, want a conflict")
	}

	_, err := ApplyPrune(PrunePlan{})
	if err == nil {
		t.Fatal("ApplyPrune succeeded during an unfinished merge, want an error")
	}
	if !strings.Contains(err.Error(), "unfinished git operation") {
		t.Errorf("ApplyPrune = %q, want it to refuse for the unfinished merge", err)
	}
}

// The current branch and the base branches are never candidates.
func TestApplyPruneNeverDeletesProtectedBranches(t *testing.T) {
	_, cfg := mergedForPrune(t, "api")

	health, err := CheckBranches(cfg)
	if err != nil {
		t.Fatalf("CheckBranches: %v", err)
	}

	if _, err := ApplyPrune(PlanPrune(cfg, health, "develop")); err != nil {
		t.Fatalf("ApplyPrune: %v", err)
	}

	for _, base := range []string{"main", "develop"} {
		if !git.BranchExists(base) {
			t.Errorf("%s was deleted", base)
		}
	}
}

func TestApplyPruneIsIdempotent(t *testing.T) {
	_, cfg := mergedForPrune(t, "api")

	health, err := CheckBranches(cfg)
	if err != nil {
		t.Fatalf("CheckBranches: %v", err)
	}

	if _, err := ApplyPrune(PlanPrune(cfg, health, "develop")); err != nil {
		t.Fatalf("first ApplyPrune: %v", err)
	}

	health, err = CheckBranches(cfg)
	if err != nil {
		t.Fatalf("CheckBranches: %v", err)
	}

	plan := PlanPrune(cfg, health, "develop")
	if !plan.Empty() {
		t.Errorf("second plan = %+v, want it empty", plan.Entries)
	}
	if _, err := ApplyPrune(plan); err != nil {
		t.Fatalf("second ApplyPrune: %v", err)
	}
}
