package git

import (
	"strings"
	"testing"

	"github.com/m-mdy-m/gix/internal/testutil"
)

func TestMergeTreeConflicts(t *testing.T) {
	repo := testutil.NewRepo(t)

	repo.CommitOn("topic", "shared.txt", "from topic\n", "feat: topic")
	repo.Checkout("main")
	repo.CommitOn("main", "shared.txt", "from main\n", "chore: main changes it")

	files, err := MergeTreeConflicts("main", "topic")
	if err != nil {
		t.Fatalf("MergeTreeConflicts: %v", err)
	}
	if len(files) != 1 || files[0] != "shared.txt" {
		t.Errorf("conflicts = %v, want [shared.txt]", files)
	}
}

func TestMergeTreeCleanMerge(t *testing.T) {
	repo := testutil.NewRepo(t)

	repo.CommitOn("topic", "topic.txt", "topic work\n", "feat: topic")
	repo.Checkout("main")
	repo.CommitOn("main", "main.txt", "main work\n", "chore: main")

	files, err := MergeTreeConflicts("main", "topic")
	if err != nil {
		t.Fatalf("MergeTreeConflicts: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("conflicts = %v, want none for a clean merge", files)
	}
}

// Two branches touching different files merge cleanly even when both are
// far ahead of the merge base.
func TestMergeTreeDifferentFilesAreClean(t *testing.T) {
	repo := testutil.NewRepo(t)

	repo.CommitOn("topic", "a.txt", "a\n", "feat: topic a")
	repo.CommitOn("topic", "b.txt", "b\n", "feat: topic b")
	repo.Checkout("main")
	repo.CommitOn("main", "c.txt", "c\n", "chore: main c")

	files, err := MergeTreeConflicts("main", "topic")
	if err != nil {
		t.Fatalf("MergeTreeConflicts: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("conflicts = %v, want none", files)
	}
}

func TestMergeTreeConflictsMultipleFiles(t *testing.T) {
	repo := testutil.NewRepo(t)

	repo.CommitOn("topic", "one.txt", "topic one\n", "feat: one")
	repo.CommitOn("topic", "two.txt", "topic two\n", "feat: two")
	repo.Checkout("main")
	repo.CommitOn("main", "one.txt", "main one\n", "chore: main one")
	repo.CommitOn("main", "two.txt", "main two\n", "chore: main two")

	files, err := MergeTreeConflicts("main", "topic")
	if err != nil {
		t.Fatalf("MergeTreeConflicts: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("conflicts = %v, want both files", files)
	}
	if files[0] != "one.txt" || files[1] != "two.txt" {
		t.Errorf("conflicts = %v, want them in a stable order", files)
	}
}

// The conflict check must not touch the repository at all.
func TestMergeTreeConflictsLeavesRepoAlone(t *testing.T) {
	repo := testutil.NewRepo(t)

	repo.CommitOn("topic", "shared.txt", "from topic\n", "feat: topic")
	repo.Checkout("main")
	repo.CommitOn("main", "shared.txt", "from main\n", "chore: main")

	before := repo.Output("for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/")
	current := repo.CurrentBranch()

	if _, err := MergeTreeConflicts("main", "topic"); err != nil {
		t.Fatalf("MergeTreeConflicts: %v", err)
	}

	if after := repo.Output("for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/"); after != before {
		t.Errorf("MergeTreeConflicts moved branches:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if got := repo.CurrentBranch(); got != current {
		t.Errorf("current branch = %q, want %q", got, current)
	}
	if !repo.Clean() {
		t.Errorf("MergeTreeConflicts dirtied the tree:\n%s", repo.Output("status", "--porcelain"))
	}
}

// merge-tree refuses unrelated histories outright (exit 128). That is a
// failure to compare, not "no conflicts", so it must come back as an
// error. `gix branch sync` checks for a merge base first and reports
// unrelated histories itself, so it never reaches this.
func TestMergeTreeConflictsRefusesUnrelatedHistories(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Run("checkout", "--orphan", "island")
	repo.Write("island.txt", "island\n")
	repo.Commit("chore: island")

	files, err := MergeTreeConflicts("main", "island")
	if err == nil {
		t.Fatalf("MergeTreeConflicts = %v, nil; want an error for unrelated histories", files)
	}
}

func TestTagRefusesADuplicate(t *testing.T) {
	testutil.NewRepo(t)

	if err := Tag("v1.0.0", "first"); err != nil {
		t.Fatalf("Tag: %v", err)
	}
	if err := Tag("v1.0.0", "second"); err == nil {
		t.Error("Tag succeeded on an existing name, want an error")
	}
	if !TagExists("v1.0.0") {
		t.Error("the original tag disappeared")
	}
}

func TestListTags(t *testing.T) {
	repo := testutil.NewRepo(t)

	tags, err := ListTags()
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("ListTags = %v, want none on a fresh repo", tags)
	}

	for _, name := range []string{"v0.1.0", "v0.10.0", "v0.9.0"} {
		repo.Run("tag", name)
	}

	tags, err = ListTags()
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	// Version order, not alphabetical: v0.9.0 sorts before v0.10.0 as text.
	want := "v0.10.0,v0.9.0,v0.1.0"
	if got := strings.Join(tags, ","); got != want {
		t.Errorf("ListTags = %q, want %q in version order", got, want)
	}
}

func TestMergeNoFF(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.CommitOn("topic", "topic.txt", "topic\n", "feat: topic")
	repo.Checkout("main")

	if err := MergeNoFF("topic", "merge topic"); err != nil {
		t.Fatalf("MergeNoFF: %v", err)
	}

	// A fast-forward would leave no merge commit, so its presence is what
	// proves --no-ff took effect.
	parents := strings.Fields(repo.Output("log", "-1", "--format=%p"))
	if len(parents) != 2 {
		t.Errorf("tip has %d parents, want 2 for a --no-ff merge: %v", len(parents), parents)
	}
}

func TestMergeNoFFConflictIsRecoverable(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.CommitOn("topic", "shared.txt", "from topic\n", "feat: topic")
	repo.Checkout("main")
	repo.CommitOn("main", "shared.txt", "from main\n", "chore: main")

	if err := MergeNoFF("topic", "merge topic"); err == nil {
		t.Fatal("MergeNoFF succeeded on conflicting content, want an error")
	}
	if err := AbortMerge(); err != nil {
		t.Fatalf("AbortMerge: %v", err)
	}
	if !repo.Clean() {
		t.Errorf("AbortMerge left the tree dirty:\n%s", repo.Output("status", "--porcelain"))
	}
}

func TestRunStatusKeepsStdoutOnFailure(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.CommitOn("topic", "shared.txt", "from topic\n", "feat: topic")
	repo.Checkout("main")
	repo.CommitOn("main", "shared.txt", "from main\n", "chore: main")

	out, err := RunStatus("merge-tree", "--write-tree", "main", "topic")
	if err == nil {
		t.Fatal("RunStatus = nil error for a conflicting merge-tree, want non-zero")
	}
	if !strings.Contains(out, "shared.txt") {
		t.Errorf("RunStatus stdout = %q, want the conflicted file on stdout even though the command failed", out)
	}
}

// Run keeps its existing contract: no output on failure.
func TestRunReturnsNothingOnFailure(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.CommitOn("topic", "shared.txt", "from topic\n", "feat: topic")
	repo.Checkout("main")
	repo.CommitOn("main", "shared.txt", "from main\n", "chore: main")

	out, err := Run("merge-tree", "--write-tree", "main", "topic")
	if err == nil {
		t.Fatal("Run = nil error for a conflicting merge-tree, want non-zero")
	}
	if out != "" {
		t.Errorf("Run stdout = %q, want it empty on failure", out)
	}
}

// A ref git cannot find is not "no conflicts": merge-tree exits 1 for it
// with nothing on stdout, and that used to read as a clean merge.
func TestMergeTreeUnknownRefIsAnError(t *testing.T) {
	repo := testutil.NewRepo(t)

	repo.CommitOn("topic", "topic.txt", "topic work\n", "feat: topic")
	repo.Checkout("main")

	files, err := MergeTreeConflicts("main", "does-not-exist")
	if err == nil {
		t.Fatalf("MergeTreeConflicts = %v, nil; want an error for an unknown ref", files)
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("error = %q, want it to name the ref that failed", err)
	}
}
