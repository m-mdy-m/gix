package flow

import (
	"strings"
	"testing"

	"github.com/m-mdy-m/gix/internal/config"
	"github.com/m-mdy-m/gix/internal/git"
)

func syncEntryFor(t *testing.T, result *SyncResult, branch string) SyncEntry {
	t.Helper()

	for _, e := range result.Entries {
		if e.Branch == branch {
			return e
		}
	}
	var names []string
	for _, e := range result.Entries {
		names = append(names, e.Branch)
	}
	t.Fatalf("no %q in the sync result, got %v", branch, names)
	return SyncEntry{}
}

func TestSyncMergesSourceIntoTopicBranches(t *testing.T) {
	repo, cfg := flowRepo(t)

	startFeature(t, repo, "api")
	startFeature(t, repo, "ui")

	// A change on develop that neither feature has.
	repo.Checkout("develop")
	repo.Write("develop.txt", "develop\n")
	repo.Commit("chore: develop moves on")

	result, err := Sync(cfg, SyncOptions{Source: "develop"})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if !result.OK() {
		t.Fatalf("Sync reported conflicts: %+v", result.Entries)
	}

	for _, branch := range []string{"feature/api", "feature/ui"} {
		entry := syncEntryFor(t, result, branch)
		if entry.State != SyncMerged {
			t.Errorf("%s = %q, want %q", branch, entry.State, SyncMerged)
		}
		if !git.IsAncestor("develop", branch) {
			t.Errorf("%s does not contain develop after the sync", branch)
		}
	}
}

// Base branches and the source must never be touched.
func TestSyncNeverTouchesBaseOrSource(t *testing.T) {
	repo, cfg := flowRepo(t)
	startFeature(t, repo, "api")

	mainBefore := repo.Output("rev-parse", "main")
	developBefore := repo.Output("rev-parse", "develop")

	if _, err := Sync(cfg, SyncOptions{Source: "develop"}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if got := repo.Output("rev-parse", "main"); got != mainBefore {
		t.Error("Sync moved main")
	}
	if got := repo.Output("rev-parse", "develop"); got != developBefore {
		t.Error("Sync moved the source branch")
	}
}

func TestSyncSkipsBranchesAlreadyUpToDate(t *testing.T) {
	repo, cfg := flowRepo(t)
	startFeature(t, repo, "api")

	result, err := Sync(cfg, SyncOptions{Source: "develop"})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	entry := syncEntryFor(t, result, "feature/api")
	if entry.State != SyncUpToDate {
		t.Errorf("state = %q, want %q for a branch that already contains develop", entry.State, SyncUpToDate)
	}
}

// A second sync with nothing new must not create an empty merge commit.
func TestSyncIsIdempotent(t *testing.T) {
	repo, cfg := flowRepo(t)

	startFeature(t, repo, "api")
	repo.Checkout("develop")
	repo.Write("develop.txt", "develop\n")
	repo.Commit("chore: develop moves on")

	if _, err := Sync(cfg, SyncOptions{Source: "develop"}); err != nil {
		t.Fatalf("first Sync: %v", err)
	}
	after := repo.Output("rev-parse", "feature/api")

	if _, err := Sync(cfg, SyncOptions{Source: "develop"}); err != nil {
		t.Fatalf("second Sync: %v", err)
	}
	if got := repo.Output("rev-parse", "feature/api"); got != after {
		t.Errorf("second Sync moved feature/api from %s to %s, want no change", after, got)
	}
}

func TestSyncDryRunChangesNothing(t *testing.T) {
	repo, cfg := flowRepo(t)

	startFeature(t, repo, "api")
	startFeature(t, repo, "ui")
	repo.Checkout("develop")
	repo.Write("develop.txt", "develop\n")
	repo.Commit("chore: develop moves on")

	before := repo.Output("for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/")
	current := repo.CurrentBranch()

	result, err := Sync(cfg, SyncOptions{Source: "develop", DryRun: true})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if after := repo.Output("for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/"); after != before {
		t.Errorf("dry run moved branches:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if got := repo.CurrentBranch(); got != current {
		t.Errorf("dry run switched to %s, want %s", got, current)
	}
	if !result.DryRun {
		t.Error("DryRun = false on a dry run")
	}

	for _, branch := range []string{"feature/api", "feature/ui"} {
		entry := syncEntryFor(t, result, branch)
		if entry.State != SyncWouldMerge {
			t.Errorf("%s = %q, want %q in a dry run", branch, entry.State, SyncWouldMerge)
		}
	}
}

// A dry run must work even on a dirty tree: refusing to preview is how you
// miss the problem you wanted to look at.
func TestSyncDryRunWorksOnADirtyTree(t *testing.T) {
	repo, cfg := flowRepo(t)
	startFeature(t, repo, "api")
	repo.Checkout("develop")
	repo.Write("develop.txt", "develop\n")
	repo.Commit("chore: develop moves on")
	repo.Write("wip.txt", "uncommitted\n")

	result, err := Sync(cfg, SyncOptions{Source: "develop", DryRun: true})
	if err != nil {
		t.Fatalf("Sync --dry-run on a dirty tree: %v", err)
	}
	if got := syncEntryFor(t, result, "feature/api").State; got != SyncWouldMerge {
		t.Errorf("state = %q, want %q", got, SyncWouldMerge)
	}
}

func TestSyncRefusesADirtyTree(t *testing.T) {
	repo, cfg := flowRepo(t)
	startFeature(t, repo, "api")
	repo.Write("wip.txt", "uncommitted\n")

	_, err := Sync(cfg, SyncOptions{Source: "develop"})
	if err == nil {
		t.Fatal("Sync succeeded on a dirty tree, want an error")
	}
	if !strings.Contains(err.Error(), "working tree is not clean") {
		t.Errorf("Sync = %q, want it to refuse for the dirty tree", err)
	}
}

func TestSyncRefusesDuringAnUnfinishedMerge(t *testing.T) {
	repo, cfg := flowRepo(t)
	startFeature(t, repo, "api")
	repo.Write("shared.txt", "feature\n")
	repo.Commit("feat: api touches shared")
	repo.Checkout("develop")
	repo.Write("shared.txt", "develop\n")
	repo.Commit("chore: develop touches shared")
	repo.Checkout("feature/api")
	if _, err := repo.Try("merge", "develop"); err == nil {
		t.Fatal("git merge succeeded on conflicting content, want a conflict")
	}

	_, err := Sync(cfg, SyncOptions{Source: "develop"})
	if err == nil {
		t.Fatal("Sync succeeded during an unfinished merge, want an error")
	}
	if !strings.Contains(err.Error(), "unfinished git operation") {
		t.Errorf("Sync = %q, want it to refuse for the unfinished merge", err)
	}
}

func TestSyncReportsConflictsAndContinues(t *testing.T) {
	repo, cfg := flowRepo(t)

	startFeature(t, repo, "clean")
	startFeature(t, repo, "conflicted")

	// develop changes a file only the conflicted branch also changed.
	repo.Checkout("develop")
	repo.Write("develop.txt", "develop\n")
	repo.Commit("chore: develop moves on")
	repo.Write("shared.txt", "from develop\n")
	repo.Commit("chore: develop touches shared")

	repo.Checkout("feature/conflicted")
	repo.Write("shared.txt", "from feature\n")
	repo.Commit("feat: conflicted touches shared")

	result, err := Sync(cfg, SyncOptions{Source: "develop"})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	// The clean branch must still have been synced.
	clean := syncEntryFor(t, result, "feature/clean")
	if clean.State != SyncMerged {
		t.Errorf("feature/clean = %q, want %q even though another branch conflicted", clean.State, SyncMerged)
	}
	if !git.IsAncestor("develop", "feature/clean") {
		t.Error("feature/clean was not synced after another branch conflicted")
	}

	conflicted := syncEntryFor(t, result, "feature/conflicted")
	if conflicted.State != SyncConflict {
		t.Errorf("feature/conflicted = %q, want %q", conflicted.State, SyncConflict)
	}
	if !strings.Contains(conflicted.Detail, "shared.txt") {
		t.Errorf("detail = %q, want it to name the conflicting file", conflicted.Detail)
	}

	if result.OK() {
		t.Error("OK() = true with a conflict reported")
	}
	if result.Conflicted != 1 {
		t.Errorf("Conflicted = %d, want 1", result.Conflicted)
	}
}

// A conflict must not leave the repository mid-merge, or the next branch
// would be blocked by leftover state.
func TestSyncLeavesNoOperationInProgress(t *testing.T) {
	repo, cfg := flowRepo(t)

	startFeature(t, repo, "clean")
	startFeature(t, repo, "conflicted")

	repo.Checkout("develop")
	repo.Write("shared.txt", "from develop\n")
	repo.Commit("chore: develop touches shared")

	repo.Checkout("feature/conflicted")
	repo.Write("shared.txt", "from feature\n")
	repo.Commit("feat: conflicted touches shared")

	if _, err := Sync(cfg, SyncOptions{Source: "develop"}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if _, found, err := git.InProgressOperation(); err != nil {
		t.Fatalf("InProgressOperation: %v", err)
	} else if found {
		t.Error("Sync left an unfinished git operation behind")
	}
	if !repo.Clean() {
		t.Errorf("Sync left the working tree dirty:\n%s", repo.Output("status", "--porcelain"))
	}
}

// The user must be put back on the branch they started on.
func TestSyncRestoresTheCurrentBranch(t *testing.T) {
	repo, cfg := flowRepo(t)

	startFeature(t, repo, "api")
	startFeature(t, repo, "ui")

	repo.Checkout("develop")
	repo.Write("develop.txt", "develop\n")
	repo.Commit("chore: develop moves on")
	repo.Checkout("feature/ui")

	if _, err := Sync(cfg, SyncOptions{Source: "develop"}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if got := repo.CurrentBranch(); got != "feature/ui" {
		t.Errorf("current branch = %q, want feature/ui after the sync", got)
	}
}

// Sync must not rewrite history, which rules out rebase entirely.
// Every field in the shipped config should mean something. `downstream`
// is what decides how a sync folds the source in, so changing it has to
// change what happens — not just what `gix config list` prints.
func TestDownstreamStrategyDecidesTheSync(t *testing.T) {
	tests := []struct {
		strategy config.Strategy
		want     SyncState
		wantWhy  string
	}{
		{strategy: config.StrategyMerge, want: SyncMerged},
		{strategy: config.StrategyRebase, want: SyncSkipped, wantWhy: "rebase"},
		{strategy: config.StrategySquash, want: SyncSkipped, wantWhy: "squash"},
	}

	for _, tc := range tests {
		t.Run(string(tc.strategy), func(t *testing.T) {
			repo, cfg := flowRepo(t)
			startFeature(t, repo, "api")
			repo.Checkout("develop")
			repo.Write("develop.txt", "develop\n")
			repo.Commit("chore: develop moves on")

			spec, _ := cfg.Spec("feature")
			spec.DownstreamStrategy = tc.strategy
			cfg.Branch["feature"] = spec
			saveConfig(t, repo, cfg)

			result, err := Sync(cfg, SyncOptions{Source: "develop"})
			if err != nil {
				t.Fatalf("Sync: %v", err)
			}

			entry := syncEntryFor(t, result, "feature/api")
			if entry.State != tc.want {
				t.Errorf("state = %q, want %q for a %s downstream", entry.State, tc.want, tc.strategy)
			}
			if tc.wantWhy != "" && !strings.Contains(entry.Detail, tc.wantWhy) {
				t.Errorf("detail = %q, want it to name %s", entry.Detail, tc.wantWhy)
			}

			// Only a merge may leave the branch containing the source.
			contains := git.IsAncestor("develop", "feature/api")
			if want := tc.want == SyncMerged; contains != want {
				t.Errorf("branch contains develop = %v, want %v", contains, want)
			}
		})
	}
}

func TestSyncSkipsKindsWithANonMergeStrategy(t *testing.T) {
	repo, cfg := flowRepo(t)
	startFeature(t, repo, "api")

	repo.Checkout("develop")
	repo.Write("develop.txt", "develop\n")
	repo.Commit("chore: develop moves on")

	spec, ok := cfg.Spec("feature")
	if !ok {
		t.Fatal("the feature kind is missing from the config")
	}
	spec.DownstreamStrategy = config.StrategyRebase
	cfg.Branch["feature"] = spec
	saveConfig(t, repo, cfg)

	result, err := Sync(cfg, SyncOptions{Source: "develop"})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	entry := syncEntryFor(t, result, "feature/api")
	if entry.State != SyncSkipped {
		t.Errorf("state = %q, want %q for a rebase downstream", entry.State, SyncSkipped)
	}
	if !strings.Contains(entry.Detail, "rebase") {
		t.Errorf("detail = %q, want it to name the strategy that stopped it", entry.Detail)
	}
	if git.IsAncestor("develop", "feature/api") {
		t.Error("Sync merged anyway despite a rebase downstream strategy")
	}
}

func TestSyncMissingSource(t *testing.T) {
	_, cfg := flowRepo(t)

	_, err := Sync(cfg, SyncOptions{Source: "nonexistent"})
	if err == nil {
		t.Fatal("Sync = nil error for a source that doesn't exist")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Sync = %q, want it to report the missing source", err)
	}
}

func TestSyncDefaultsToTheFirstBaseBranch(t *testing.T) {
	repo, cfg := flowRepo(t)

	head := repo.CurrentBranch()
	if _, ok := IsConfiguredBase(cfg, head); !ok {
		t.Skipf("the test repo is not on a base branch (%s), so the default path is covered elsewhere", head)
	}

	startFeature(t, repo, "api")
	repo.Checkout(head)
	repo.Write("base.txt", "base\n")
	repo.Commit("chore: base moves on")

	result, err := Sync(cfg, SyncOptions{})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if got := syncEntryFor(t, result, "feature/api").State; got != SyncMerged {
		t.Errorf("state = %q, want %q syncing from the current base branch", got, SyncMerged)
	}
}

func TestSyncWithNoTopicBranches(t *testing.T) {
	_, cfg := flowRepo(t)

	result, err := Sync(cfg, SyncOptions{Source: "develop"})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(result.Entries) != 0 {
		t.Errorf("Entries = %+v, want none when no topic branches are open", result.Entries)
	}
	if !result.OK() {
		t.Error("OK() = false with nothing to do")
	}
}

// Sync refuses to resolve a conflict, so it must never be the thing that
// decides one.
func TestSyncNeverResolvesConflicts(t *testing.T) {
	repo, cfg := flowRepo(t)

	startFeature(t, repo, "conflicted")
	repo.Checkout("develop")
	repo.Write("shared.txt", "from develop\n")
	repo.Commit("chore: develop touches shared")
	repo.Checkout("feature/conflicted")
	repo.Write("shared.txt", "from feature\n")
	repo.Commit("feat: conflicted touches shared")

	if _, err := Sync(cfg, SyncOptions{Source: "develop"}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	// The file on the feature branch must be untouched: no auto-resolution,
	// no ours/theirs pick, nothing.
	content := repo.Output("show", "feature/conflicted:shared.txt")
	if strings.TrimSpace(content) != "from feature" {
		t.Errorf("the branch's content changed to %q, want it left exactly as it was", content)
	}
}

func TestSyncErrorMessage(t *testing.T) {
	result := &SyncResult{
		Entries: []SyncEntry{
			{Branch: "feature/clean", Source: "develop", State: SyncMerged},
			{Branch: "feature/bad", Source: "develop", State: SyncConflict},
		},
		Conflicted: 1,
	}

	err := &SyncError{Result: result}
	if !strings.Contains(err.Error(), "feature/bad") {
		t.Errorf("error = %q, want it to name the branch that failed", err)
	}
	if strings.Contains(err.Error(), "feature/clean") {
		t.Errorf("error = %q, want it not to blame the branch that worked", err)
	}
}

func TestSyncErrorMessageDryRun(t *testing.T) {
	result := &SyncResult{
		DryRun: true,
		Entries: []SyncEntry{
			{Branch: "feature/bad", Source: "develop", State: SyncConflict},
		},
		Conflicted: 1,
	}

	err := (&SyncError{Result: result}).Error()
	if !strings.Contains(err, "would conflict") {
		t.Errorf("dry-run error = %q, want it to be phrased as a preview", err)
	}
	if !strings.Contains(err, "nothing was changed") {
		t.Errorf("dry-run error = %q, want it to say nothing was changed", err)
	}
}

func TestSyncEntryOK(t *testing.T) {
	tests := map[SyncState]bool{
		SyncUpToDate:   true,
		SyncMerged:     true,
		SyncWouldMerge: true,
		SyncConflict:   false,
		SyncFailed:     false,
		SyncSkipped:    false,
	}

	for state, want := range tests {
		if got := (SyncEntry{State: state}).OK(); got != want {
			t.Errorf("%q.OK() = %v, want %v", state, got, want)
		}
	}
}

// A branch skipped on purpose (a rebase or squash strategy) did not fail,
// so the error message must not name it next to the real conflict.
func TestSyncErrorMessageIgnoresSkippedBranches(t *testing.T) {
	result := &SyncResult{
		Conflicted: 1,
		Entries: []SyncEntry{
			{Branch: "feature/a", Source: "develop", State: SyncConflict},
			{Branch: "feature/b", Source: "develop", State: SyncSkipped},
			{Branch: "feature/c", Source: "develop", State: SyncMerged},
		},
	}

	msg := (&SyncError{Result: result}).Error()
	if !strings.Contains(msg, "feature/a") {
		t.Errorf("message = %q, want it to name the conflicting branch", msg)
	}
	if strings.Contains(msg, "feature/b") || strings.Contains(msg, "feature/c") {
		t.Errorf("message = %q, want only the branch that conflicted", msg)
	}
}
