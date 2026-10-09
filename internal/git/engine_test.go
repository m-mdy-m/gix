package git

import (
	"strings"
	"testing"

	"github.com/m-mdy-m/gix/internal/testutil"
)

func TestRepoDetectionOutsideRepo(t *testing.T) {
	// testutil switches into its own repo; step one directory further out.
	t.Chdir(t.TempDir())

	if IsRepo() {
		t.Error("IsRepo() = true outside any repository")
	}
	if _, err := RepoRoot(); err == nil {
		t.Error("RepoRoot() = nil error outside any repository")
	}
	if _, err := CurrentBranch(); err == nil {
		t.Error("CurrentBranch() = nil error outside any repository")
	}
}

func TestHeadStateOnBranch(t *testing.T) {
	testutil.NewRepo(t)

	head, err := GetHeadState()
	if err != nil {
		t.Fatalf("GetHeadState: %v", err)
	}
	if head.Kind != HeadOnBranch {
		t.Errorf("Kind = %v, want HeadOnBranch", head.Kind)
	}
	if head.Branch != "main" {
		t.Errorf("Branch = %q, want %q", head.Branch, "main")
	}
	if head.Commit == "" {
		t.Error("Commit is empty")
	}
	if !HasCommits() {
		t.Error("HasCommits() = false on a repo with a commit")
	}
}

func TestHeadStateUnborn(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Run("checkout", "--orphan", "fresh")

	head, err := GetHeadState()
	if err != nil {
		t.Fatalf("GetHeadState: %v", err)
	}
	if head.Kind != HeadUnborn {
		t.Errorf("Kind = %v, want HeadUnborn", head.Kind)
	}
	if head.Branch != "fresh" {
		t.Errorf("Branch = %q, want %q", head.Branch, "fresh")
	}
	if HasCommits() {
		t.Error("HasCommits() = true on an orphan branch")
	}
}

func TestHeadStateDetached(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Run("checkout", "--detach")

	head, err := GetHeadState()
	if err != nil {
		t.Fatalf("GetHeadState: %v", err)
	}
	if head.Kind != HeadDetached {
		t.Errorf("Kind = %v, want HeadDetached", head.Kind)
	}
	if head.Branch != "" {
		t.Errorf("Branch = %q, want it empty on detached HEAD", head.Branch)
	}
}

func TestIsClean(t *testing.T) {
	repo := testutil.NewRepo(t)

	clean, err := IsClean()
	if err != nil {
		t.Fatalf("IsClean: %v", err)
	}
	if !clean {
		t.Error("IsClean() = false on a fresh repo")
	}

	repo.Write("scratch.txt", "wip\n")
	clean, err = IsClean()
	if err != nil {
		t.Fatalf("IsClean: %v", err)
	}
	if clean {
		t.Error("IsClean() = true with an untracked file present")
	}
}

func TestBranchHelpers(t *testing.T) {
	testutil.NewRepo(t)

	if BranchExists("topic") {
		t.Error("BranchExists(\"topic\") = true before it is created")
	}

	if err := CheckoutNew("topic", "main"); err != nil {
		t.Fatalf("CheckoutNew: %v", err)
	}
	if !BranchExists("topic") {
		t.Error("BranchExists(\"topic\") = false after creation")
	}

	names, err := ListBranches()
	if err != nil {
		t.Fatalf("ListBranches: %v", err)
	}
	// for-each-ref sorts by refname, so this order is stable across runs.
	if got := strings.Join(names, ","); got != "main,topic" {
		t.Errorf("ListBranches() = %q, want %q", got, "main,topic")
	}

	refs, err := ListBranchRefs()
	if err != nil {
		t.Fatalf("ListBranchRefs: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("ListBranchRefs() returned %d refs, want 2", len(refs))
	}
	if refs[0].Name != "main" || refs[0].Current {
		t.Errorf("ListBranchRefs()[0] = %+v, want main with Current false", refs[0])
	}
	if refs[1].Name != "topic" || !refs[1].Current {
		t.Errorf("ListBranchRefs()[1] = %+v, want topic marked Current", refs[1])
	}
}

func TestListBranchesEmptyRepo(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Run("checkout", "--orphan", "fresh")

	names, err := ListBranches()
	if err != nil {
		t.Fatalf("ListBranches: %v", err)
	}
	// An unborn orphan branch has no ref yet, so only main is listed.
	if got := strings.Join(names, ","); got != "main" {
		t.Errorf("ListBranches() = %q, want %q", got, "main")
	}
}

func TestDeleteBranch(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.CommitOn("topic", "topic.txt", "work\n", "topic commit")
	repo.Checkout("main")

	if err := DeleteBranch("topic", false); err == nil {
		t.Error("DeleteBranch(-d) succeeded on an unmerged branch, want an error")
	}
	if err := DeleteBranch("topic", true); err != nil {
		t.Fatalf("DeleteBranch(-D): %v", err)
	}
	if BranchExists("topic") {
		t.Error("BranchExists(\"topic\") = true after forced delete")
	}
}

func TestIdentity(t *testing.T) {
	testutil.NewRepo(t)

	id := CurrentIdentity()
	if !id.Configured() {
		t.Errorf("CurrentIdentity() = %+v, want a configured identity", id)
	}
	if id.Name != "gix test" {
		t.Errorf("Name = %q, want %q", id.Name, "gix test")
	}

	if _, err := Run("config", "--unset", "user.name"); err != nil {
		t.Fatalf("unsetting user.name: %v", err)
	}
	id = CurrentIdentity()
	if id.Configured() {
		t.Errorf("CurrentIdentity() = %+v, want Configured() = false with no name", id)
	}
}

func TestInProgressOperation(t *testing.T) {
	repo := testutil.NewRepo(t)

	op, found, err := InProgressOperation()
	if err != nil {
		t.Fatalf("InProgressOperation: %v", err)
	}
	if found {
		t.Errorf("InProgressOperation() found %q on a clean repo", op)
	}

	repo.CommitOn("feature/base", "a.txt", "a\n", "base commit")
	repo.Checkout("main")
	repo.CommitOn("main", "a.txt", "conflicting\n", "diverging commit")
	repo.Checkout("feature/base")
	// Staged, unmerged content is what leaves a merge in progress.
	if _, err := repo.Try("merge", "main"); err == nil {
		t.Fatal("git merge succeeded on conflicting content, want a conflict")
	}

	op, found, err = InProgressOperation()
	if err != nil {
		t.Fatalf("InProgressOperation: %v", err)
	}
	if !found {
		t.Fatal("InProgressOperation() found nothing during a conflicted merge")
	}
	if op != OpMerge {
		t.Errorf("op = %q, want %q", op, OpMerge)
	}
}

func TestAheadBehind(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.CommitOn("topic", "topic.txt", "work\n", "topic commit")

	ahead, behind, err := AheadBehind("topic", "main")
	if err != nil {
		t.Fatalf("AheadBehind: %v", err)
	}
	if ahead != 1 || behind != 0 {
		t.Errorf("AheadBehind(topic, main) = %d, %d; want 1, 0", ahead, behind)
	}

	repo.Checkout("topic")
	repo.CommitOn("main", "main.txt", "more\n", "main commit")

	ahead, behind, err = AheadBehind("topic", "main")
	if err != nil {
		t.Fatalf("AheadBehind: %v", err)
	}
	if ahead != 1 || behind != 1 {
		t.Errorf("AheadBehind(topic, main) = %d, %d; want 1, 1", ahead, behind)
	}
}

func TestIsAncestor(t *testing.T) {
	repo := testutil.NewRepo(t)

	if !IsAncestor("main", "main") {
		t.Error("IsAncestor(main, main) = false, want true")
	}

	repo.CommitOn("topic", "topic.txt", "work\n", "topic commit")
	if !IsAncestor("main", "topic") {
		t.Error("IsAncestor(main, topic) = false, want true (topic is ahead)")
	}
	if IsAncestor("topic", "main") {
		t.Error("IsAncestor(topic, main) = true, want false (topic is ahead)")
	}
}

func TestTagHelpers(t *testing.T) {
	testutil.NewRepo(t)

	if TagExists("v1.0.0") {
		t.Error("TagExists(\"v1.0.0\") = true before it is created")
	}
	if err := Tag("v1.0.0", "release 1.0.0"); err != nil {
		t.Fatalf("Tag: %v", err)
	}
	if !TagExists("v1.0.0") {
		t.Error("TagExists(\"v1.0.0\") = false after creation")
	}
	if TagExists("v1.0") {
		t.Error("TagExists(\"v1.0\") = true for a different tag name")
	}
}

func TestMerge(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.CommitOn("topic", "topic.txt", "work\n", "topic commit")
	repo.Checkout("main")

	if err := Merge("topic", "merge topic"); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if !IsAncestor("topic", "main") {
		t.Error("topic is not an ancestor of main after merging")
	}
}

func TestMergeConflictLeavesOperationInProgress(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.CommitOn("topic", "shared.txt", "topic version\n", "topic commit")
	repo.Checkout("main")
	repo.CommitOn("main", "shared.txt", "main version\n", "main commit")
	repo.Checkout("topic")

	if err := Merge("main", "merge main"); err == nil {
		t.Fatal("Merge() succeeded on conflicting content, want an error")
	}

	op, found, err := InProgressOperation()
	if err != nil {
		t.Fatalf("InProgressOperation: %v", err)
	}
	if !found || op != OpMerge {
		t.Errorf("InProgressOperation() = %q, %v; want %q, true", op, found, OpMerge)
	}
}

func TestFastForwardMergeRefusesDivergence(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.CommitOn("topic", "topic.txt", "work\n", "topic commit")
	repo.Checkout("main")
	repo.CommitOn("main", "main.txt", "more\n", "main commit")

	if err := FastForwardMerge("topic"); err == nil {
		t.Error("FastForwardMerge() succeeded on divergent branches, want an error")
	}
}

func TestGetStatus(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.CommitOn("feature/api", "api.txt", "work\n", "feature commit")

	st, err := GetStatus("main")
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if st.Branch != "feature/api" {
		t.Errorf("Branch = %q, want %q", st.Branch, "feature/api")
	}
	if !st.Clean {
		t.Error("Clean = false on a clean tree")
	}
	if !st.HasBase {
		t.Fatal("HasBase = false even though main exists")
	}
	if st.Ahead != 1 || st.Behind != 0 {
		t.Errorf("Ahead/Behind = %d/%d, want 1/0", st.Ahead, st.Behind)
	}
}

func TestGetStatusWithoutComparableBase(t *testing.T) {
	testutil.NewRepo(t)

	st, err := GetStatus("nonexistent")
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if st.HasBase {
		t.Error("HasBase = true for a base branch that doesn't exist")
	}
}

func TestRunSurfacesStderr(t *testing.T) {
	_, err := Run("checkout", "no-such-branch")
	if err == nil {
		t.Fatal("Run() = nil error for a branch that doesn't exist")
	}
	if !strings.Contains(err.Error(), "git checkout no-such-branch") {
		t.Errorf("Run() = %q, want it to name the failing command", err)
	}
}
