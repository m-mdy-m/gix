package flow

import (
	"strings"
	"testing"

	"github.com/m-mdy-m/gix/internal/config"
	"github.com/m-mdy-m/gix/internal/testutil"
)

func healthFor(t *testing.T, health []BranchHealth, name string) BranchHealth {
	t.Helper()

	for _, h := range health {
		if h.Name == name {
			return h
		}
	}
	var names []string
	for _, h := range health {
		names = append(names, h.Name)
	}
	t.Fatalf("no %q branch in the health report, got %v", name, names)
	return BranchHealth{}
}

func TestCheckBranchesPlacement(t *testing.T) {
	repo, cfg := flowRepo(t)
	startFeature(t, repo, "api")
	repo.Checkout("develop")

	health, err := CheckBranches(cfg)
	if err != nil {
		t.Fatalf("CheckBranches: %v", err)
	}

	main := healthFor(t, health, "main")
	if main.Role != "base" || main.Kind != "" {
		t.Errorf("main = %+v, want a base branch with no kind", main)
	}
	if main.State != StateUntracked {
		t.Errorf("main state = %q, want %q", main.State, StateUntracked)
	}

	feature := healthFor(t, health, "feature/api")
	if feature.Kind != "feature" || feature.Role != "topic" {
		t.Errorf("feature/api = %+v, want kind feature and role topic", feature)
	}
	if feature.Parent != "develop" {
		t.Errorf("feature/api parent = %q, want develop", feature.Parent)
	}
}

func TestCheckBranchesIsDeterministic(t *testing.T) {
	repo, cfg := flowRepo(t)
	startFeature(t, repo, "api")
	startFeature(t, repo, "ui")
	repo.Checkout("develop")

	first, err := CheckBranches(cfg)
	if err != nil {
		t.Fatalf("CheckBranches: %v", err)
	}
	for i := 0; i < 5; i++ {
		again, err := CheckBranches(cfg)
		if err != nil {
			t.Fatalf("CheckBranches: %v", err)
		}
		for j := range first {
			if first[j].Name != again[j].Name {
				t.Fatalf("branch %d changed between runs: %q then %q", j, first[j].Name, again[j].Name)
			}
		}
	}
}

func TestCheckBranchesStates(t *testing.T) {
	tests := []struct {
		name  string
		track bool
		setup func(t *testing.T, repo *testutil.Repo)
		want  BranchState
	}{
		{
			name: "untracked",
			want: StateUntracked,
		},
		{
			name:  "behind",
			track: true,
			setup: func(t *testing.T, repo *testutil.Repo) {
				repo.Checkout("main")
				repo.Write("m.txt", "m\n")
				repo.Commit("chore: main")
				repo.Run("update-ref", "refs/remotes/origin/topic", "main")
				repo.Checkout("topic")
			},
			want: StateBehind,
		},
		{
			name:  "ahead",
			track: true,
			setup: func(t *testing.T, repo *testutil.Repo) {
				repo.Checkout("topic")
				repo.Write("t.txt", "t\n")
				repo.Commit("feat: topic")
			},
			want: StateAhead,
		},
		{
			name:  "tracked",
			track: true,
			want:  StateTracked,
		},
		{
			name:  "gone",
			track: true,
			setup: func(t *testing.T, repo *testutil.Repo) {
				repo.Run("update-ref", "-d", "refs/remotes/origin/topic")
			},
			want: StateGone,
		},
	}

	// Each state gets its own repo: the setups are not independent of
	// each other, and sharing one would make the order part of the test.
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo, cfg := flowRepo(t)
			repo.Branch("topic")
			if tc.track {
				trackBranch(t, repo, "topic", "origin", "topic")
			}
			if tc.setup != nil {
				tc.setup(t, repo)
			}

			health, err := CheckBranches(cfg)
			if err != nil {
				t.Fatalf("CheckBranches: %v", err)
			}
			if got := healthFor(t, health, "topic").State; got != tc.want {
				t.Errorf("topic state = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCheckBranchesDiverged(t *testing.T) {
	repo, cfg := flowRepo(t)
	repo.Branch("topic")
	trackBranch(t, repo, "topic", "origin", "topic")

	repo.Checkout("topic")
	repo.Write("t.txt", "t\n")
	repo.Commit("feat: topic")

	repo.Checkout("main")
	repo.Write("m.txt", "m\n")
	repo.Commit("chore: main")
	repo.Run("update-ref", "refs/remotes/origin/topic", "main")
	repo.Checkout("topic")

	health, err := CheckBranches(cfg)
	if err != nil {
		t.Fatalf("CheckBranches: %v", err)
	}

	h := healthFor(t, health, "topic")
	if h.State != StateDiverged {
		t.Errorf("topic state = %q, want %q", h.State, StateDiverged)
	}
	if h.Ahead != 1 || h.Behind != 1 {
		t.Errorf("ahead/behind = %d/%d, want 1/1", h.Ahead, h.Behind)
	}
	if !h.NeedsWork() {
		t.Error("NeedsWork() = false for a diverged branch")
	}
}

func TestCheckBranchesMergeState(t *testing.T) {
	repo, cfg := flowRepo(t)
	full := startFeature(t, repo, "api")

	health, err := CheckBranches(cfg)
	if err != nil {
		t.Fatalf("CheckBranches: %v", err)
	}
	if h := healthFor(t, health, full); h.AlreadyMerged() {
		t.Errorf("%s = %+v, want not merged before finishing", full, h)
	}

	if _, err := Finish(cfg, "feature", "api", FinishOptions{DeleteBranch: false}); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	health, err = CheckBranches(cfg)
	if err != nil {
		t.Fatalf("CheckBranches: %v", err)
	}
	h := healthFor(t, health, full)
	if !h.AlreadyMerged() {
		t.Fatalf("%s = %+v, want merged after finishing", full, h)
	}
	if strings.Join(h.MergedInto, ",") != "develop" {
		t.Errorf("MergedInto = %v, want [develop]", h.MergedInto)
	}
}

func TestCheckBranchesMultiTargetMergeState(t *testing.T) {
	repo, cfg := flowRepo(t)

	full, err := Start(cfg, "hotfix", "fire")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	repo.Write("fix.txt", "fix\n")
	repo.Commit("fix: fire")

	health, err := CheckBranches(cfg)
	if err != nil {
		t.Fatalf("CheckBranches: %v", err)
	}
	if h := healthFor(t, health, full); h.AlreadyMerged() {
		t.Errorf("%s = %+v, want not merged before finishing", full, h)
	}

	if _, err := Finish(cfg, "hotfix", "fire", FinishOptions{DeleteBranch: false}); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	health, err = CheckBranches(cfg)
	if err != nil {
		t.Fatalf("CheckBranches: %v", err)
	}

	h := healthFor(t, health, full)
	if !h.AlreadyMerged() {
		t.Fatalf("%s = %+v, want merged after finishing", full, h)
	}
	if len(h.MergedInto) != 2 {
		t.Errorf("MergedInto = %v, want both main and develop", h.MergedInto)
	}
}

// A branch whose merge target does not exist can't be judged merged, and
// must not be reported as though it were.
func TestCheckBranchesMissingTarget(t *testing.T) {
	repo, cfg := flowRepo(t)
	startFeature(t, repo, "api")
	repo.Checkout("main")
	repo.Run("branch", "-D", "develop")

	health, err := CheckBranches(cfg)
	if err != nil {
		t.Fatalf("CheckBranches: %v", err)
	}

	h := healthFor(t, health, "feature/api")
	if len(h.MergedInto) != 0 {
		t.Errorf("MergedInto = %v, want nothing when the target is missing", h.MergedInto)
	}
	if h.AlreadyMerged() {
		t.Error("AlreadyMerged() = true with no reachable merge target")
	}
}

// The longest configured prefix has to win, or a narrower kind would be
// invisible.
func TestCheckBranchesLongestPrefixWins(t *testing.T) {
	repo, cfg := flowRepo(t)
	repo.Branch("feature/deep/api")
	repo.Branch("feature/shallow")

	cfg.Branch["featuredeep"] = config.BranchSpec{
		Role:   config.RoleTopic,
		Prefix: "feature/deep/",
		Parent: "develop",
	}
	saveConfig(t, repo, cfg)

	health, err := CheckBranches(cfg)
	if err != nil {
		t.Fatalf("CheckBranches: %v", err)
	}

	if got := healthFor(t, health, "feature/deep/api").Kind; got != "featuredeep" {
		t.Errorf("kind = %q, want featuredeep (the longer prefix)", got)
	}
	if got := healthFor(t, health, "feature/shallow").Kind; got != "feature" {
		t.Errorf("kind = %q, want feature", got)
	}
}

// A bare prefix isn't a branch name, so it belongs to no kind. Git won't
// even create one, which is why this checks the matcher directly.
func TestFindKindIgnoresBarePrefix(t *testing.T) {
	cfg := config.Default()

	if kind, _, ok := findKind(cfg, "feature/"); ok {
		t.Errorf("findKind(feature/) = %q, want no match", kind)
	}
	if kind, _, ok := findKind(cfg, "feature/api"); !ok || kind != "feature" {
		t.Errorf("findKind(feature/api) = %q, %v; want feature, true", kind, ok)
	}
	if kind, _, ok := findKind(cfg, "main"); ok {
		t.Errorf("findKind(main) = %q, want no match: main is a base branch", kind)
	}
}

func TestCheckBranchesOutsideAConfiguredKind(t *testing.T) {
	repo, cfg := flowRepo(t)
	repo.Branch("someone-elses-work")

	health, err := CheckBranches(cfg)
	if err != nil {
		t.Fatalf("CheckBranches: %v", err)
	}

	h := healthFor(t, health, "someone-elses-work")
	if h.Kind != "" || h.Role != "" {
		t.Errorf("%+v, want no kind and no role", h)
	}
}

func TestPlanPruneMergedBranch(t *testing.T) {
	repo, cfg := flowRepo(t)
	full := startFeature(t, repo, "api")
	if _, err := Finish(cfg, "feature", "api", FinishOptions{DeleteBranch: false}); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	repo.Checkout("develop")

	health, err := CheckBranches(cfg)
	if err != nil {
		t.Fatalf("CheckBranches: %v", err)
	}

	plan := PlanPrune(cfg, health, "develop")
	if plan.Empty() {
		t.Fatalf("PlanPrune = empty, want %s listed", full)
	}
	if len(plan.Entries) != 1 || plan.Entries[0].Name != full {
		t.Fatalf("Entries = %+v, want just %s", plan.Entries, full)
	}
	if !strings.Contains(plan.Entries[0].Reason, "develop") {
		t.Errorf("reason = %q, want it to name the merge target", plan.Entries[0].Reason)
	}
	if plan.Entries[0].Kind != "feature" {
		t.Errorf("kind = %q, want feature", plan.Entries[0].Kind)
	}
}

func TestPlanPruneGoneUpstream(t *testing.T) {
	repo, cfg := flowRepo(t)
	repo.Branch("feature/fire")
	trackBranch(t, repo, "feature/fire", "origin", "feature/fire")
	repo.Run("update-ref", "-d", "refs/remotes/origin/feature/fire")

	health, err := CheckBranches(cfg)
	if err != nil {
		t.Fatalf("CheckBranches: %v", err)
	}

	plan := PlanPrune(cfg, health, "main")
	if len(plan.Entries) != 1 || plan.Entries[0].Name != "feature/fire" {
		t.Fatalf("Entries = %+v, want the branch with the gone upstream", plan.Entries)
	}
	if !strings.Contains(plan.Entries[0].Reason, "no longer exists") {
		t.Errorf("reason = %q, want it to explain the upstream is gone", plan.Entries[0].Reason)
	}
}

// Unmerged work is never a prune candidate, however stale it is.
func TestPlanPruneSkipsUnmergedBranches(t *testing.T) {
	repo, cfg := flowRepo(t)
	startFeature(t, repo, "api")
	repo.Checkout("develop")
	// A branch with no commits of its own is trivially "already merged",
	// so this one needs work that exists nowhere else.
	repo.Branch("feature/abandoned")
	repo.Checkout("feature/abandoned")
	repo.Write("abandoned.txt", "work in progress\n")
	repo.Commit("feat: abandoned")
	repo.Checkout("develop")

	health, err := CheckBranches(cfg)
	if err != nil {
		t.Fatalf("CheckBranches: %v", err)
	}

	plan := PlanPrune(cfg, health, "develop")
	for _, e := range plan.Entries {
		if e.Name == "feature/abandoned" {
			t.Errorf("%+v, want an unmerged branch left alone", e)
		}
	}
}

func TestPlanPruneNeverListsBaseOrCurrentBranch(t *testing.T) {
	repo, cfg := flowRepo(t)
	full := startFeature(t, repo, "api")
	if _, err := Finish(cfg, "feature", "api", FinishOptions{DeleteBranch: false}); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	// Stay on the merged branch itself.
	repo.Checkout(full)

	health, err := CheckBranches(cfg)
	if err != nil {
		t.Fatalf("CheckBranches: %v", err)
	}

	plan := PlanPrune(cfg, health, full)

	for _, e := range plan.Entries {
		if e.Name == full {
			t.Errorf("%+v, want the branch you are standing on never listed", e)
		}
		if e.Name == "main" || e.Name == "develop" {
			t.Errorf("%+v, want base branches never listed", e)
		}
	}
	if !containsName(plan.Protected, full) {
		t.Errorf("Protected = %v, want it to list %s", plan.Protected, full)
	}
	if !containsName(plan.Protected, "develop") {
		t.Errorf("Protected = %v, want develop listed as protected", plan.Protected)
	}
}

func TestPlanPruneEmptyPlan(t *testing.T) {
	_, cfg := flowRepo(t)

	health, err := CheckBranches(cfg)
	if err != nil {
		t.Fatalf("CheckBranches: %v", err)
	}

	if plan := PlanPrune(cfg, health, "main"); !plan.Empty() {
		t.Errorf("PlanPrune = %+v, want empty on a fresh flow", plan)
	}
}

func TestBranchStateAttention(t *testing.T) {
	states := map[BranchState]bool{
		StateTracked:   false,
		StateAhead:     true,
		StateBehind:    true,
		StateDiverged:  true,
		StateUntracked: true,
		StateGone:      true,
	}

	for state, want := range states {
		if got := state.Attention(); got != want {
			t.Errorf("%q.Attention() = %v, want %v", state, got, want)
		}
	}
}

// Nothing in branch health may be able to delete a branch by itself.
func TestNoPruneEntryWithoutAReason(t *testing.T) {
	repo, cfg := flowRepo(t)
	startFeature(t, repo, "api")
	if _, err := Finish(cfg, "feature", "api", FinishOptions{DeleteBranch: false}); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	repo.Checkout("develop")

	health, err := CheckBranches(cfg)
	if err != nil {
		t.Fatalf("CheckBranches: %v", err)
	}

	for _, e := range PlanPrune(cfg, health, "develop").Entries {
		if e.Reason == "" {
			t.Errorf("%s is listed with no reason", e.Name)
		}
	}
}

func trackBranch(t *testing.T, repo *testutil.Repo, branch, remote, remoteBranch string) {
	t.Helper()

	if _, err := repo.Try("remote", "add", remote, "https://example.invalid/"+remote+".git"); err != nil {
		out, outErr := repo.Try("remote")
		if outErr != nil || !hasRemote(out, remote) {
			t.Fatalf("adding remote %s: %v\n%s", remote, outErr, out)
		}
	}
	repo.Run("update-ref", "refs/remotes/"+remote+"/"+remoteBranch, remoteBranch)
	repo.Run("config", "branch."+branch+".remote", remote)
	repo.Run("config", "branch."+branch+".merge", "refs/heads/"+remoteBranch)
}

func hasRemote(out, name string) bool {
	for _, line := range strings.Split(out, "\n") {
		if line == name {
			return true
		}
	}
	return false
}

func containsName(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}
