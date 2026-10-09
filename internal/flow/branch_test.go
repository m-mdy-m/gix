package flow

import (
	"strings"
	"testing"

	"github.com/m-mdy-m/gix/internal/config"
	"github.com/m-mdy-m/gix/internal/git"
	"github.com/m-mdy-m/gix/internal/testutil"
)

// flowRepo is a repo with .gix/config committed and develop created, i.e.
// what `gix flow init` leaves behind. The config has to be committed,
// otherwise every mutating flow call refuses the dirty tree.
func flowRepo(t *testing.T) (*testutil.Repo, *config.Config) {
	t.Helper()

	repo := testutil.NewRepo(t)
	repo.Branch("develop")

	cfg := config.Default()
	if err := config.Save(repo.Root, cfg); err != nil {
		t.Fatalf("saving config: %v", err)
	}
	repo.Commit("chore(gix): initialize gix flow config")

	return repo, cfg
}

// startFeature commits on a fresh feature branch, leaving develop behind.
func startFeature(t *testing.T, repo *testutil.Repo, short string) string {
	t.Helper()

	full, err := Start(config.Default(), "feature", short)
	if err != nil {
		t.Fatalf("Start(feature, %s): %v", short, err)
	}
	repo.Write("feature.txt", "work\n")
	repo.Commit("feat: " + short)
	return full
}

func TestFullName(t *testing.T) {
	cfg := config.Default()

	tests := []struct {
		kind  string
		short string
		want  string
	}{
		{"feature", "api", "feature/api"},
		{"feature", "feature/api", "feature/api"},
		{"bugfix", "crash", "bugfix/crash"},
		{"hotfix", "fire", "hotfix/fire"},
	}

	for _, tc := range tests {
		t.Run(tc.kind+"/"+tc.short, func(t *testing.T) {
			got, err := FullName(cfg, tc.kind, tc.short)
			if err != nil {
				t.Fatalf("FullName: %v", err)
			}
			if got != tc.want {
				t.Errorf("FullName(%q, %q) = %q, want %q", tc.kind, tc.short, got, tc.want)
			}
		})
	}
}

func TestFullNameUnknownKind(t *testing.T) {
	for _, kind := range []string{"nope", "main", "develop"} {
		if _, err := FullName(config.Default(), kind, "x"); err == nil {
			t.Errorf("FullName(%q) = nil error, want unknown branch kind", kind)
		}
	}
}

func TestStart(t *testing.T) {
	repo, cfg := flowRepo(t)

	full, err := Start(cfg, "feature", "api")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if full != "feature/api" {
		t.Errorf("Start returned %q, want %q", full, "feature/api")
	}
	if !git.BranchExists("feature/api") {
		t.Error("feature/api does not exist after Start")
	}
	if repo.CurrentBranch() != "feature/api" {
		t.Errorf("current branch = %q, want %q", repo.CurrentBranch(), "feature/api")
	}
}

func TestStartRejects(t *testing.T) {
	tests := []struct {
		name    string
		arrange func(t *testing.T, repo *testutil.Repo)
		kind    string
		short   string
		wantErr string
	}{
		{
			name:    "unknown kind",
			kind:    "nope",
			short:   "x",
			wantErr: `unknown branch kind "nope"`,
		},
		{
			name:    "base branch as kind",
			kind:    "main",
			short:   "x",
			wantErr: `unknown branch kind "main"`,
		},
		{
			name:    "empty name",
			kind:    "feature",
			short:   "",
			wantErr: "name is required",
		},
		{
			name:    "invalid characters",
			kind:    "feature",
			short:   "bad name~1",
			wantErr: "invalid branch name",
		},
		{
			name: "already exists",
			arrange: func(t *testing.T, repo *testutil.Repo) {
				repo.Branch("feature/api")
			},
			kind:    "feature",
			short:   "api",
			wantErr: "already exists",
		},
		{
			name: "parent base is missing",
			arrange: func(t *testing.T, repo *testutil.Repo) {
				repo.Run("branch", "-D", "develop")
			},
			kind:    "feature",
			short:   "api",
			wantErr: "base branch develop does not exist",
		},
		{
			name: "dirty working tree",
			arrange: func(t *testing.T, repo *testutil.Repo) {
				repo.Write("wip.txt", "wip\n")
			},
			kind:    "feature",
			short:   "api",
			wantErr: "working tree is not clean",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo, cfg := flowRepo(t)
			if tc.arrange != nil {
				tc.arrange(t, repo)
			}

			_, err := Start(cfg, tc.kind, tc.short)
			if err == nil {
				t.Fatalf("Start(%q, %q) = nil error, want it to contain %q", tc.kind, tc.short, tc.wantErr)
			}
			if !contains(err.Error(), tc.wantErr) {
				t.Errorf("Start(%q, %q) = %q, want it to contain %q", tc.kind, tc.short, err, tc.wantErr)
			}
		})
	}
}

// A conflicted merge must block any mutating gix command, with a message
// that names the operation and how to get out of it.
func TestStartBlockedByUnfinishedOperation(t *testing.T) {
	repo, cfg := flowRepo(t)

	repo.CommitOn("feature/api", "shared.txt", "topic\n", "feat: shared")
	repo.Checkout("develop")
	repo.CommitOn("develop", "shared.txt", "develop\n", "chore: shared")
	repo.Checkout("feature/api")
	if _, err := repo.Try("merge", "develop"); err == nil {
		t.Fatal("git merge succeeded on conflicting content, want a conflict")
	}

	_, err := Start(cfg, "feature", "other")
	if err == nil {
		t.Fatal("Start succeeded during an unfinished merge, want an error")
	}
	if !contains(err.Error(), "unfinished git operation") || !contains(err.Error(), "merge") {
		t.Errorf("Start = %q, want it to report the unfinished merge", err)
	}
}

func TestFinishMerge(t *testing.T) {
	repo, cfg := flowRepo(t)
	full := startFeature(t, repo, "api")

	result, err := Finish(cfg, "feature", "api", FinishOptions{DeleteBranch: true})
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if result.Branch != full {
		t.Errorf("Branch = %q, want %q", result.Branch, full)
	}
	if len(result.MergedInto) != 1 || result.MergedInto[0] != "develop" {
		t.Errorf("MergedInto = %v, want [develop]", result.MergedInto)
	}
	if !result.BranchDeleted {
		t.Error("BranchDeleted = false, want true")
	}
	if git.BranchExists(full) {
		t.Error("the feature branch still exists after Finish deleted it")
	}
	if repo.CurrentBranch() != "develop" {
		t.Errorf("current branch = %q, want develop", repo.CurrentBranch())
	}
}

func TestFinishNoDelete(t *testing.T) {
	repo, cfg := flowRepo(t)
	full := startFeature(t, repo, "api")

	result, err := Finish(cfg, "feature", "api", FinishOptions{})
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if result.DeleteAttempted {
		t.Error("DeleteAttempted = true without DeleteBranch set")
	}
	if !git.BranchExists(full) {
		t.Error("the branch was deleted even though delete was not requested")
	}
}

func TestFinishAlreadyMergedIsANoop(t *testing.T) {
	repo, cfg := flowRepo(t)
	startFeature(t, repo, "api")

	if _, err := Finish(cfg, "feature", "api", FinishOptions{}); err != nil {
		t.Fatalf("first Finish: %v", err)
	}

	result, err := Finish(cfg, "feature", "api", FinishOptions{})
	if err != nil {
		t.Fatalf("second Finish: %v", err)
	}
	if len(result.MergedInto) != 0 {
		t.Errorf("MergedInto = %v, want nothing on a repeat run", result.MergedInto)
	}
	if len(result.AlreadyMerged) != 1 || result.AlreadyMerged[0] != "develop" {
		t.Errorf("AlreadyMerged = %v, want [develop]", result.AlreadyMerged)
	}
}

func TestFinishIntoMultipleTargets(t *testing.T) {
	repo, cfg := flowRepo(t)
	full, err := Start(cfg, "hotfix", "fire")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	repo.Write("fix.txt", "fix\n")
	repo.Commit("fix: fire")

	result, err := Finish(cfg, "hotfix", "fire", FinishOptions{DeleteBranch: true})
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if len(result.MergedInto) != 2 {
		t.Fatalf("MergedInto = %v, want main and develop", result.MergedInto)
	}
	if !result.BranchDeleted {
		t.Error("BranchDeleted = false, want true")
	}
	if git.BranchExists(full) {
		t.Error("the hotfix branch still exists after Finish deleted it")
	}
	// The branch is gone, so ancestry has to be checked through the commit
	// each target points at.
	if !fileExists(t, repo.Root, "fix.txt") {
		t.Error("fix.txt is missing from the checkout after both merges")
	}
}

func TestFinishRebaseStrategy(t *testing.T) {
	repo, cfg := flowRepo(t)
	startFeature(t, repo, "api")

	repo.Checkout("develop")
	repo.Write("develop.txt", "develop\n")
	repo.Commit("chore: develop moved on")

	result, err := Finish(cfg, "feature", "api", FinishOptions{Strategy: config.StrategyRebase})
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if result.Strategy != config.StrategyRebase {
		t.Errorf("Strategy = %q, want %q", result.Strategy, config.StrategyRebase)
	}
	if !git.IsAncestor("feature/api", "develop") {
		t.Error("feature/api is not an ancestor of develop after a rebase finish")
	}
}

func TestFinishSquashStrategy(t *testing.T) {
	repo, cfg := flowRepo(t)
	full := startFeature(t, repo, "api")

	result, err := Finish(cfg, "feature", "api", FinishOptions{Strategy: config.StrategySquash})
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if result.Strategy != config.StrategySquash {
		t.Errorf("Strategy = %q, want %q", result.Strategy, config.StrategySquash)
	}
	// A squash replays the changes as one new commit, so the branch is
	// deliberately NOT an ancestor of the target — that's the point of it.
	if git.IsAncestor(full, "develop") {
		t.Error("IsAncestor(feature/api, develop) = true after a squash, want the history to be collapsed")
	}
	if !fileExists(t, repo.Root, "feature.txt") {
		t.Error("feature.txt is missing from develop after a squash finish")
	}
}

func TestFinishTag(t *testing.T) {
	repo, cfg := flowRepo(t)
	full, err := Start(cfg, "release", "v1.0.0")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if full != "release/v1.0.0" {
		t.Errorf("Start returned %q, want %q", full, "release/v1.0.0")
	}
	repo.Write("release.txt", "notes\n")
	repo.Commit("chore: release notes")

	result, err := Finish(cfg, "release", "v1.0.0", FinishOptions{Tag: "v1.0.0", DeleteBranch: true})
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if result.Tagged != "v1.0.0" {
		t.Errorf("Tagged = %q, want %q", result.Tagged, "v1.0.0")
	}
	if result.TaggedOn != "main" {
		t.Errorf("TaggedOn = %q, want %q", result.TaggedOn, "main")
	}
	if !git.TagExists("v1.0.0") {
		t.Error("the tag does not exist after Finish created it")
	}
	if repo.CurrentBranch() != "main" {
		t.Errorf("current branch = %q, want main, since the tag is created there", repo.CurrentBranch())
	}
	tagged, err := git.Run("rev-list", "-n", "1", "v1.0.0")
	if err != nil {
		t.Fatalf("reading the tagged commit: %v", err)
	}
	if mainHead := repo.Output("rev-parse", "main"); !strings.HasPrefix(mainHead, tagged) {
		t.Errorf("tag v1.0.0 points at %s, want it on main's commit %s", tagged, mainHead)
	}
}

func TestFinishRejects(t *testing.T) {
	tests := []struct {
		name    string
		arrange func(t *testing.T, repo *testutil.Repo)
		kind    string
		short   string
		opts    FinishOptions
		wantErr string
	}{
		{
			name:    "unknown kind",
			kind:    "nope",
			short:   "x",
			wantErr: `unknown branch kind "nope"`,
		},
		{
			name:    "empty name",
			kind:    "feature",
			short:   "",
			wantErr: "name is required",
		},
		{
			name:    "branch does not exist",
			kind:    "feature",
			short:   "ghost",
			wantErr: "does not exist",
		},
		{
			name: "merge target is missing",
			arrange: func(t *testing.T, repo *testutil.Repo) {
				repo.Branch("feature/api")
				repo.Run("branch", "-D", "develop")
			},
			kind:    "feature",
			short:   "api",
			wantErr: `merge target "develop" (branch "develop") does not exist`,
		},
		{
			name: "tag on a kind that does not tag",
			arrange: func(t *testing.T, repo *testutil.Repo) {
				repo.Branch("feature/api")
			},
			kind:    "feature",
			short:   "api",
			opts:    FinishOptions{Tag: "v9.9.9"},
			wantErr: "don't support --tag",
		},
		{
			name: "invalid strategy",
			arrange: func(t *testing.T, repo *testutil.Repo) {
				repo.Branch("feature/api")
			},
			kind:    "feature",
			short:   "api",
			opts:    FinishOptions{Strategy: "cherry-pick"},
			wantErr: `invalid strategy "cherry-pick"`,
		},
		{
			name: "dirty working tree",
			arrange: func(t *testing.T, repo *testutil.Repo) {
				repo.Branch("feature/api")
				repo.Checkout("feature/api")
				repo.Write("wip.txt", "wip\n")
			},
			kind:    "feature",
			short:   "api",
			wantErr: "working tree is not clean",
		},
		{
			name: "tag already exists",
			arrange: func(t *testing.T, repo *testutil.Repo) {
				repo.Branch("release/v1.0.0")
				repo.Run("tag", "v1.0.0")
			},
			kind:    "release",
			short:   "v1.0.0",
			opts:    FinishOptions{Tag: "v1.0.0"},
			wantErr: "tag v1.0.0 already exists",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo, cfg := flowRepo(t)
			if tc.arrange != nil {
				tc.arrange(t, repo)
			}

			_, err := Finish(cfg, tc.kind, tc.short, tc.opts)
			if err == nil {
				t.Fatalf("Finish = nil error, want it to contain %q", tc.wantErr)
			}
			if !contains(err.Error(), tc.wantErr) {
				t.Errorf("Finish = %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

// A failed merge into the second target must say that the first target was
// already changed and the branch was NOT deleted — that state is not undone.
func TestFinishReportsPartialProgressOnFailure(t *testing.T) {
	repo, cfg := flowRepo(t)

	full, err := Start(cfg, "hotfix", "fire")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	repo.Write("hotfix.txt", "fix\n")
	repo.Commit("fix: fire")
	// Conflict on develop, but main can still take the merge.
	repo.Checkout("develop")
	repo.Write("shared.txt", "develop\n")
	repo.Commit("chore: develop changes shared")
	repo.Checkout("hotfix/fire")
	repo.Write("shared.txt", "hotfix\n")
	repo.Commit("fix: fire changes shared")
	repo.Checkout("main")

	_, err = Finish(cfg, "hotfix", "fire", FinishOptions{DeleteBranch: true})
	if err == nil {
		t.Fatal("Finish succeeded on conflicting content, want an error")
	}
	if !contains(err.Error(), "was already merged into main") {
		t.Errorf("Finish = %q, want it to report the merge into main already done", err)
	}
	if !contains(err.Error(), "was NOT deleted") {
		t.Errorf("Finish = %q, want it to say the branch was NOT deleted", err)
	}
	if !git.IsAncestor(full, "main") {
		t.Error("the merge into main was rolled back, want it kept")
	}
	if !git.BranchExists(full) {
		t.Error("the branch was deleted even though the second merge failed")
	}
}

func TestList(t *testing.T) {
	repo, cfg := flowRepo(t)
	startFeature(t, repo, "api")
	repo.Checkout("develop")
	startFeature(t, repo, "ui")
	if _, err := Start(cfg, "bugfix", "crash"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	features, err := List(cfg, "feature")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := join(features); got != "api,ui" {
		t.Errorf("List(feature) = %q, want %q", got, "api,ui")
	}

	bugfixes, err := List(cfg, "bugfix")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := join(bugfixes); got != "crash" {
		t.Errorf("List(bugfix) = %q, want %q", got, "crash")
	}
}

func TestListUnknownKind(t *testing.T) {
	if _, err := List(config.Default(), "nope"); err == nil {
		t.Error("List(nope) = nil error, want unknown branch kind")
	}
}
