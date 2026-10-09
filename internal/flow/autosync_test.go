package flow

import (
	"strings"
	"testing"

	"github.com/m-mdy-m/gix/internal/config"
	"github.com/m-mdy-m/gix/internal/git"
	"github.com/m-mdy-m/gix/internal/testutil"
)

// autoSyncRepo builds a repo with one feature branch and one bugfix branch,
// so a per-kind autosync filter is actually observable.
func autoSyncRepo(t *testing.T) (*testutil.Repo, *config.Config) {
	t.Helper()

	repo, cfg := flowRepo(t)

	startFeature(t, repo, "api")

	if _, err := Start(cfg, "bugfix", "crash"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	repo.Write("crash.txt", "crash work\n")
	repo.Commit("fix: crash")
	repo.Checkout("develop")

	return repo, cfg
}

// markAuto sets or clears autosync for a kind.
func markAuto(t *testing.T, repo *testutil.Repo, cfg *config.Config, kind string, on bool) {
	t.Helper()

	spec, ok := cfg.Spec(kind)
	if !ok {
		t.Fatalf("kind %q is missing from the config", kind)
	}
	spec.AutoSync = on
	cfg.Branch[kind] = spec
	saveConfig(t, repo, cfg)
}

// moveDevelop adds a commit to develop so every topic branch is behind it.
func moveDevelop(t *testing.T, repo *testutil.Repo, file string) {
	t.Helper()

	repo.Checkout("develop")
	repo.Write(file, "develop\n")
	repo.Commit("chore: develop moves on")
}

func TestSyncAutoOnlyTouchesMarkedKinds(t *testing.T) {
	repo, cfg := autoSyncRepo(t)
	markAuto(t, repo, cfg, "feature", true)
	moveDevelop(t, repo, "develop.txt")

	result, err := Sync(cfg, SyncOptions{Source: "develop", AutoOnly: true})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if got := syncEntryFor(t, result, "feature/api").State; got != SyncMerged {
		t.Errorf("feature/api = %q, want %q", got, SyncMerged)
	}
	for _, e := range result.Entries {
		if e.Branch == "bugfix/crash" {
			t.Errorf("bugfix/crash was synced even though only the feature kind is marked: %+v", e)
		}
	}
	if git.IsAncestor("develop", "bugfix/crash") {
		t.Error("bugfix/crash contains develop, so it was synced without being marked")
	}
}

// Without --auto every topic branch is still eligible, so marking a kind
// does not change the default run.
func TestSyncWithoutAutoIgnoresTheMark(t *testing.T) {
	repo, cfg := autoSyncRepo(t)
	markAuto(t, repo, cfg, "feature", true)
	moveDevelop(t, repo, "develop.txt")

	result, err := Sync(cfg, SyncOptions{Source: "develop"})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	for _, branch := range []string{"feature/api", "bugfix/crash"} {
		if got := syncEntryFor(t, result, branch).State; got != SyncMerged {
			t.Errorf("%s = %q, want %q in a default run", branch, got, SyncMerged)
		}
	}
}

// The default config already marks develop as autosync with main as its
// parent, which is what "develop follows main" means.
func TestSyncAutoFollowsBaseBranchParent(t *testing.T) {
	repo, cfg := flowRepo(t)

	developSpec, ok := cfg.Spec("develop")
	if !ok {
		t.Fatal("develop is missing from the config")
	}
	if !developSpec.AutoSync {
		t.Fatal("the default config should mark develop autosync")
	}
	if developSpec.Parent != "main" {
		t.Fatalf("develop parent = %q, want main", developSpec.Parent)
	}

	repo.Write("main.txt", "main\n")
	repo.Commit("chore: main moves on")

	result, err := Sync(cfg, SyncOptions{Source: "main", AutoOnly: true})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if got := syncEntryFor(t, result, "develop").State; got != SyncMerged {
		t.Errorf("develop = %q, want %q for a base branch marked autosync", got, SyncMerged)
	}
	if !git.IsAncestor("main", "develop") {
		t.Error("develop does not contain main after the sync")
	}
}

// A base branch that is not marked must never be merged into, however
// central it is to the flow.
func TestSyncAutoSkipsUnmarkedBaseBranches(t *testing.T) {
	repo, cfg := flowRepo(t)
	markAuto(t, repo, cfg, "develop", false)

	developBefore := repo.Output("rev-parse", "develop")
	repo.Write("main.txt", "main\n")
	repo.Commit("chore: main moves on")

	if _, err := Sync(cfg, SyncOptions{Source: "main", AutoOnly: true}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if got := repo.Output("rev-parse", "develop"); got != developBefore {
		t.Error("develop was merged into even though it is not marked")
	}
}

func TestSyncAutoWithNothingMarked(t *testing.T) {
	repo, cfg := flowRepo(t)
	startFeature(t, repo, "api")

	markAuto(t, repo, cfg, "feature", false)
	markAuto(t, repo, cfg, "develop", false)
	moveDevelop(t, repo, "develop.txt")

	result, err := Sync(cfg, SyncOptions{Source: "develop", AutoOnly: true})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(result.Entries) != 0 {
		t.Errorf("Entries = %+v, want none when nothing is marked autosync", result.Entries)
	}
	if !result.OK() {
		t.Error("OK() = false with nothing to do")
	}
}

func TestSyncAutoNeverTouchesTheSource(t *testing.T) {
	repo, cfg := flowRepo(t)

	repo.Write("main.txt", "main\n")
	repo.Commit("chore: main moves on")
	mainBefore := repo.Output("rev-parse", "main")

	if _, err := Sync(cfg, SyncOptions{Source: "main", AutoOnly: true}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if got := repo.Output("rev-parse", "main"); got != mainBefore {
		t.Error("the source branch was modified")
	}
}

func TestSyncAutoDryRunReportsOnlyMarkedBranches(t *testing.T) {
	repo, cfg := autoSyncRepo(t)
	markAuto(t, repo, cfg, "feature", true)
	moveDevelop(t, repo, "develop.txt")

	before := repo.Output("for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/")

	result, err := Sync(cfg, SyncOptions{Source: "develop", AutoOnly: true, DryRun: true})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if got := syncEntryFor(t, result, "feature/api").State; got != SyncWouldMerge {
		t.Errorf("feature/api = %q, want %q", got, SyncWouldMerge)
	}
	for _, e := range result.Entries {
		if e.Branch == "bugfix/crash" {
			t.Errorf("bugfix/crash appears in an --auto dry run: %+v", e)
		}
	}
	if after := repo.Output("for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/"); after != before {
		t.Error("an --auto dry run changed branches")
	}
}

// Auto sync obeys the same conflict rules as a manual sync: reported, not
// resolved, and the other branches still get their turn.
func TestSyncAutoConflictIsReportedNotResolved(t *testing.T) {
	repo, cfg := autoSyncRepo(t)
	markAuto(t, repo, cfg, "feature", true)

	repo.Checkout("develop")
	repo.Write("shared.txt", "from develop\n")
	repo.Commit("chore: develop touches shared")

	repo.Checkout("feature/api")
	repo.Write("shared.txt", "from the feature\n")
	repo.Commit("feat: api touches shared")

	before := repo.Output("rev-parse", "feature/api")

	result, err := Sync(cfg, SyncOptions{Source: "develop", AutoOnly: true})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	entry := syncEntryFor(t, result, "feature/api")
	if entry.State != SyncConflict {
		t.Errorf("feature/api = %q, want %q", entry.State, SyncConflict)
	}
	if got := repo.Output("rev-parse", "feature/api"); got != before {
		t.Error("the conflicting branch was modified")
	}
	if !strings.Contains(entry.Detail, "shared.txt") {
		t.Errorf("detail = %q, want it to name the conflicting file", entry.Detail)
	}

	// The unmarked kind is not eligible, so it must be untouched too.
	if git.IsAncestor("develop", "bugfix/crash") {
		t.Error("bugfix/crash was synced even though it is not marked autosync")
	}
	if _, found, err := git.InProgressOperation(); err != nil {
		t.Fatalf("InProgressOperation: %v", err)
	} else if found {
		t.Error("an --auto run left an unfinished git operation behind")
	}
}

// autosync is a filter, not a new mechanism: the branch it reaches still
// has to contain the source, so a second run is a no-op.
func TestSyncAutoIsIdempotent(t *testing.T) {
	repo, cfg := flowRepo(t)
	startFeature(t, repo, "api")
	repo.Checkout("develop")
	repo.Write("develop.txt", "develop\n")
	repo.Commit("chore: develop moves on")
	markAuto(t, repo, cfg, "feature", true)

	if _, err := Sync(cfg, SyncOptions{Source: "develop", AutoOnly: true}); err != nil {
		t.Fatalf("first Sync: %v", err)
	}
	after := repo.Output("rev-parse", "feature/api")

	result, err := Sync(cfg, SyncOptions{Source: "develop", AutoOnly: true})
	if err != nil {
		t.Fatalf("second Sync: %v", err)
	}
	if got := syncEntryFor(t, result, "feature/api").State; got != SyncUpToDate {
		t.Errorf("state = %q, want %q on the second run", got, SyncUpToDate)
	}
	if got := repo.Output("rev-parse", "feature/api"); got != after {
		t.Error("the second run moved the branch")
	}
}

// The flag must be the only thing that changes eligibility: no hidden
// side effects, and no syncing without it.
func TestSyncAutoIsOptIn(t *testing.T) {
	repo, cfg := autoSyncRepo(t)
	markAuto(t, repo, cfg, "feature", true)
	moveDevelop(t, repo, "develop.txt")

	// With develop itself as the source and --auto off, nothing marked is
	// synced: the run needs an explicit flag.
	plain, err := Sync(cfg, SyncOptions{Source: "develop", DryRun: true})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	for _, e := range plain.Entries {
		if e.Branch != "feature/api" && e.Branch != "bugfix/crash" {
			t.Errorf("unexpected branch %s in a plain run", e.Branch)
		}
	}

	auto, err := Sync(cfg, SyncOptions{Source: "develop", DryRun: true, AutoOnly: true})
	if err != nil {
		t.Fatalf("Sync --auto: %v", err)
	}
	if len(auto.Entries) >= len(plain.Entries) {
		t.Errorf("--auto listed %d branches, a plain run listed %d: --auto should be narrower",
			len(auto.Entries), len(plain.Entries))
	}
}

// A kind marked autosync with a non-merge downstream strategy is skipped
// like any other, because skipping is what keeps history safe.
func TestSyncAutoHonoursNonMergeStrategy(t *testing.T) {
	repo, cfg := flowRepo(t)
	startFeature(t, repo, "api")
	repo.Checkout("develop")
	repo.Write("develop.txt", "develop\n")
	repo.Commit("chore: develop moves on")
	markAuto(t, repo, cfg, "feature", true)

	spec, _ := cfg.Spec("feature")
	spec.DownstreamStrategy = config.StrategySquash
	cfg.Branch["feature"] = spec
	saveConfig(t, repo, cfg)

	result, err := Sync(cfg, SyncOptions{Source: "develop", AutoOnly: true})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	entry := syncEntryFor(t, result, "feature/api")
	if entry.State != SyncSkipped {
		t.Errorf("state = %q, want %q for a squash downstream", entry.State, SyncSkipped)
	}
	if git.IsAncestor("develop", "feature/api") {
		t.Error("a squash downstream was synced anyway")
	}
}

func TestSyncCandidatesAutoOnlyIgnoresUnknownBranches(t *testing.T) {
	repo, cfg := flowRepo(t)
	repo.Branch("not-in-the-flow")

	branches, err := syncCandidates(cfg, "develop", true)
	if err != nil {
		t.Fatalf("syncCandidates: %v", err)
	}
	for _, b := range branches {
		if b == "not-in-the-flow" {
			t.Errorf("%q should not be a candidate: it is in no configured kind", b)
		}
	}
}
