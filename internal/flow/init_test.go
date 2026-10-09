package flow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m-mdy-m/gix/internal/config"
	"github.com/m-mdy-m/gix/internal/git"
	"github.com/m-mdy-m/gix/internal/testutil"
)

func TestInit(t *testing.T) {
	repo := testutil.NewRepo(t)

	result, err := Init(repo.Root, InitOptions{})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if !result.ConfigWritten {
		t.Error("ConfigWritten = false, want .gix/config written")
	}
	if !result.Committed {
		t.Error("Committed = false, want the config committed")
	}
	if result.CommitSkipped {
		t.Error("CommitSkipped = true despite a usable identity")
	}
	if !result.DevelopCreated {
		t.Error("DevelopCreated = false, want develop created")
	}
	if result.DevelopExisted {
		t.Error("DevelopExisted = true on a repo that had no develop")
	}
	if !repo.Clean() {
		t.Error("working tree is dirty after Init, want the config committed")
	}
	if got := repo.CurrentBranch(); got != "develop" {
		t.Errorf("current branch = %q, want develop", got)
	}
}

func TestInitIsIdempotent(t *testing.T) {
	repo := testutil.NewRepo(t)

	if _, err := Init(repo.Root, InitOptions{}); err != nil {
		t.Fatalf("first Init: %v", err)
	}

	_, err := Init(repo.Root, InitOptions{})
	if err == nil {
		t.Fatal("second Init = nil error, want it to report gix is already initialized")
	}
	if !strings.Contains(err.Error(), "already initialized") {
		t.Errorf("second Init = %q, want it to say already initialized", err)
	}
}

// A config written by hand but never committed is resumable: init should
// finish the job instead of refusing.
func TestInitResumesUncommittedConfig(t *testing.T) {
	repo := testutil.NewRepo(t)

	cfg := config.Default()
	if err := config.Save(repo.Root, cfg); err != nil {
		t.Fatalf("saving config: %v", err)
	}

	result, err := Init(repo.Root, InitOptions{})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if result.ConfigWritten {
		t.Error("ConfigWritten = true, want the existing config reused")
	}
	if !result.Committed {
		t.Error("Committed = false, want init to finish committing the config")
	}
}

func TestInitNoCommit(t *testing.T) {
	repo := testutil.NewRepo(t)

	result, err := Init(repo.Root, InitOptions{NoCommit: true})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if result.Committed {
		t.Error("Committed = true despite --no-commit")
	}
	if !result.CommitSkipped {
		t.Error("CommitSkipped = false despite --no-commit")
	}
	if result.SkipReason == "" {
		t.Error("SkipReason is empty, want it to explain why")
	}
	if !result.DevelopCreated {
		t.Error("DevelopCreated = false, want develop created even with --no-commit")
	}
}

func TestInitAuthorOverride(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Run("config", "--unset", "user.name")
	repo.Run("config", "--unset", "user.email")

	result, err := Init(repo.Root, InitOptions{AuthorName: "Someone", AuthorEmail: "someone@example.invalid"})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if !result.Committed {
		t.Fatalf("Committed = false, want the author override to be enough (skip reason: %q)", result.SkipReason)
	}

	author := repo.Output("log", "-1", "--format=%an <%ae>")
	if !strings.Contains(author, "someone@example.invalid") {
		t.Errorf("author = %q, want the override to be recorded", author)
	}
}

func TestInitSkipsCommitWithoutIdentity(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Run("config", "--unset", "user.name")
	repo.Run("config", "--unset", "user.email")

	result, err := Init(repo.Root, InitOptions{})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if result.Committed {
		t.Error("Committed = true with no identity configured")
	}
	if !result.CommitSkipped {
		t.Error("CommitSkipped = false with no identity configured")
	}
	if result.SkipReason != "no git author identity is configured" {
		t.Errorf("SkipReason = %q, want it to name the missing identity", result.SkipReason)
	}
}

func TestInitCustomNames(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Run("branch", "-m", "main", "trunk")

	result, err := Init(repo.Root, InitOptions{MainName: "trunk", DevelopName: "dev"})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if result.Config.MainName() != "trunk" {
		t.Errorf("MainName() = %q, want trunk", result.Config.MainName())
	}
	if result.Config.DevelopName() != "dev" {
		t.Errorf("DevelopName() = %q, want dev", result.Config.DevelopName())
	}
	if got := repo.CurrentBranch(); got != "dev" {
		t.Errorf("current branch = %q, want dev", got)
	}
}

// With no --main, init should notice a repo whose main branch is named
// master or trunk and use it rather than creating a duplicate main.
func TestInfersMainFromCurrentBranch(t *testing.T) {
	for _, name := range []string{"master", "trunk"} {
		t.Run(name, func(t *testing.T) {
			repo := testutil.NewRepo(t)
			repo.Run("branch", "-m", name)

			result, err := Init(repo.Root, InitOptions{})
			if err != nil {
				t.Fatalf("Init: %v", err)
			}
			if result.Config.MainName() != name {
				t.Errorf("MainName() = %q, want it to adopt %q", result.Config.MainName(), name)
			}
		})
	}
}

func TestInitLeavesUnrelatedBranchAlone(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Branch("somebody-elses-work")

	if _, err := Init(repo.Root, InitOptions{}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if !git.BranchExists("somebody-elses-work") {
		t.Error("Init removed an unrelated local branch")
	}
}

func TestInitOnTopicBranchDoesNotAdoptIt(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Run("checkout", "-b", "topic")

	result, err := Init(repo.Root, InitOptions{})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if result.Config.MainName() != "main" {
		t.Errorf("MainName() = %q, want main, since topic is not a base branch name", result.Config.MainName())
	}
}

func TestInitRejectsMissingMainBranch(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Run("checkout", "-b", "only")

	_, err := Init(repo.Root, InitOptions{MainName: "nonexistent"})
	if err == nil {
		t.Fatal("Init = nil error for a main branch that doesn't exist")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Init = %q, want it to report the missing main branch", err)
	}
}

func TestInitRejects(t *testing.T) {
	tests := []struct {
		name    string
		opts    InitOptions
		wantErr string
	}{
		{name: "empty main name", opts: InitOptions{MainName: "  "}, wantErr: "branch name cannot be empty"},
		{name: "main with dots", opts: InitOptions{MainName: "a..b"}, wantErr: `cannot contain ".."`},
		{name: "main with slash edges", opts: InitOptions{MainName: "/main"}, wantErr: `cannot start or end with "/"`},
		{name: "main with ref characters", opts: InitOptions{MainName: "main~1"}, wantErr: "doesn't allow in refs"},
		{name: "same base twice", opts: InitOptions{MainName: "x", DevelopName: "x"}, wantErr: "must be different"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := testutil.NewRepo(t)

			_, err := Init(repo.Root, tc.opts)
			if err == nil {
				t.Fatalf("Init = nil error, want it to contain %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("Init = %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestInitRejectsWhenConfigIsUnusable(t *testing.T) {
	repo := testutil.NewRepo(t)

	broken := "gix:\n  version: \"1\"\nbranch: {}\n"
	writeRawConfig(t, repo.Root, broken)

	_, err := Init(repo.Root, InitOptions{})
	if err == nil {
		t.Fatal("Init = nil error for an invalid uncommitted config, want an error")
	}
	if !strings.Contains(err.Error(), "is invalid") {
		t.Errorf("Init = %q, want it to explain the config is invalid", err)
	}
}

// writeRawConfig puts arbitrary text in .gix/config, for cases the typed
// config API can't express.
func writeRawConfig(t *testing.T, repoRoot, content string) {
	t.Helper()

	dir := filepath.Join(repoRoot, config.DirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(content), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
}
