package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m-mdy-m/gix/internal/config"
	"github.com/m-mdy-m/gix/internal/testutil"
	"github.com/m-mdy-m/gix/internal/ui"
)

func readFile(repo *testutil.Repo, name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(repo.Root, filepath.FromSlash(name)))
	return string(data), err
}

func writeFile(t *testing.T, repo *testutil.Repo, name, content string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(repo.Root, filepath.FromSlash(name)), []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
}

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

// gixRepo is a repo with the flow set up, ready for tests that need
// gix to accept mutations.
func gixRepo(t *testing.T) *testutil.Repo {
	t.Helper()

	repo := testutil.NewRepo(t)
	run(t, repo, "flow", "init")
	return repo
}

// exec runs gix with args and captures everything it printed, stdout and
// stderr together, so assertions don't depend on which stream a message
// uses. ui disables colors automatically when it isn't writing to a file.
func exec(t *testing.T, repo *testutil.Repo, args ...string) (string, error) {
	t.Helper()

	var buf bytes.Buffer
	oldOut, oldErr := ui.Out, ui.Err
	ui.Out, ui.Err = &buf, &buf
	t.Cleanup(func() { ui.Out, ui.Err = oldOut, oldErr })

	if repo != nil {
		t.Chdir(repo.Root)
	}

	root := New()
	root.SetArgs(args)
	err := root.Execute()

	return buf.String(), err
}

// run executes gix and fails the test if the command errored.
func run(t *testing.T, repo *testutil.Repo, args ...string) string {
	t.Helper()

	out, err := exec(t, repo, args...)
	if err != nil {
		t.Fatalf("gix %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

// runFails executes gix expecting an error, and returns it.
func runFails(t *testing.T, repo *testutil.Repo, args ...string) error {
	t.Helper()

	_, err := exec(t, repo, args...)
	if err == nil {
		t.Fatalf("gix %s: nil error, want a failure", strings.Join(args, " "))
	}
	return err
}

// runFailsWith executes gix expecting an error containing want.
func runFailsWith(t *testing.T, repo *testutil.Repo, want string, args ...string) {
	t.Helper()

	err := runFails(t, repo, args...)
	if !strings.Contains(err.Error(), want) {
		t.Errorf("gix %s = %q, want it to contain %q", strings.Join(args, " "), err, want)
	}
}

func TestEveryCommandHasHelp(t *testing.T) {
	root := New()
	if root.Short == "" {
		t.Error("root command has no Short description")
	}
	if root.Version == "" {
		t.Error("root command has no Version")
	}
	if root.Long == "" {
		t.Error("root command has no Long description")
	}

	// An empty Short is what a user sees in `gix --help`, and an empty
	// Long leaves `gix <cmd> --help` bare.
	wantSubcommands := []string{"init", "flow", "feature", "bugfix", "hotfix", "release", "status", "list", "config"}
	have := make(map[string]bool, len(wantSubcommands))
	for _, c := range root.Commands() {
		if c.Short == "" {
			t.Errorf("command %q has no Short description", c.Name())
		}
		for _, sub := range c.Commands() {
			if sub.Short == "" {
				t.Errorf("command %q %q has no Short description", c.Name(), sub.Name())
			}
		}
		have[c.Name()] = true
	}
	for _, name := range wantSubcommands {
		if !have[name] {
			t.Errorf("command %q is not registered", name)
		}
	}
}

func TestGlobalFlagsAreBound(t *testing.T) {
	for _, name := range []string{"verbose", "debug", "no-color"} {
		if New().PersistentFlags().Lookup(name) == nil {
			t.Errorf("global flag --%s is not registered", name)
		}
	}
}

func TestCommandsRequireARepo(t *testing.T) {
	testutil.NewDir(t)

	for _, args := range [][]string{{"status"}, {"list"}, {"config", "list"}, {"feature", "start", "api"}} {
		runFailsWith(t, nil, "not a git repository", args...)
	}
}

func TestCommandsRequireGixConfig(t *testing.T) {
	repo := testutil.NewRepo(t)

	for _, args := range [][]string{{"status"}, {"list"}, {"config", "list"}, {"feature", "start", "api"}, {"feature", "finish", "api"}} {
		runFailsWith(t, repo, "gix is not initialized", args...)
	}
}

func TestFlowInit(t *testing.T) {
	repo := testutil.NewRepo(t)

	out := run(t, repo, "flow", "init")

	for _, want := range []string{"gix initialized", "config", "main", "develop", "committed .gix/config"} {
		if !strings.Contains(out, want) {
			t.Errorf("`gix flow init` output is missing %q:\n%s", want, out)
		}
	}
	if !repo.Clean() {
		t.Error("working tree is dirty after `gix flow init`, want the config committed")
	}
	if repo.CurrentBranch() != "develop" {
		t.Errorf("current branch = %q, want develop", repo.CurrentBranch())
	}
}

func TestFlowInitTwice(t *testing.T) {
	repo := testutil.NewRepo(t)
	run(t, repo, "flow", "init")

	runFailsWith(t, repo, "already initialized", "flow", "init")
}

func TestFlowInitWithoutCommits(t *testing.T) {
	dir := testutil.NewDir(t)
	dir.Run("init", "-b", "main")

	runFailsWith(t, dir, "no commits yet", "flow", "init")
}

func TestFlowInitCustomNames(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Run("branch", "-m", "main", "trunk")

	run(t, repo, "flow", "init", "--main=trunk", "--develop=dev")

	if repo.CurrentBranch() != "dev" {
		t.Errorf("current branch = %q, want dev", repo.CurrentBranch())
	}
}

func TestFlowInitRejectsIdenticalBaseNames(t *testing.T) {
	repo := testutil.NewRepo(t)

	runFailsWith(t, repo, "must be different", "flow", "init", "--main=trunk", "--develop=trunk")
}

func TestFlowInitRejectsInvalidBaseNames(t *testing.T) {
	repo := testutil.NewRepo(t)

	runFailsWith(t, repo, "invalid branch name", "flow", "init", "--main=bad name")
	runFailsWith(t, repo, "invalid branch name", "flow", "init", "--develop=has..dots")
}

func TestFlowInitNoCommit(t *testing.T) {
	repo := testutil.NewRepo(t)

	out := run(t, repo, "flow", "init", "--no-commit")

	if !strings.Contains(out, "not committed") {
		t.Errorf("`gix flow init --no-commit` output should say the config is uncommitted:\n%s", out)
	}
	if repo.Clean() {
		t.Error("working tree is clean after --no-commit, want .gix/config left uncommitted")
	}
}

func TestFlowInitWithMissingIdentity(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Run("config", "--unset", "user.email")

	out := run(t, repo, "flow", "init")

	if !strings.Contains(out, "no git author identity is configured") {
		t.Errorf("output should explain the missing identity:\n%s", out)
	}
	if repo.Clean() {
		t.Error("working tree is clean, want .gix/config left uncommitted when no identity is configured")
	}
}

func TestInitCreatesRepository(t *testing.T) {
	dir := testutil.NewDir(t)

	out := run(t, dir, "init", "--author-name=Test User", "--author-email=test@example.invalid")

	for _, want := range []string{"initialized empty Git repository", "path"} {
		if !strings.Contains(out, want) {
			t.Errorf("`gix init` output is missing %q:\n%s", want, out)
		}
	}
	if !dir.HasCommits() {
		t.Error("repo has no commit after `gix init --author-*`")
	}
	if got := dir.CommitCount(); got != "1" {
		t.Errorf("commit count = %q, want 1", got)
	}
	if got := dir.CurrentBranch(); got != "main" {
		t.Errorf("branch = %q, want main", got)
	}
}

// `gix init` passes no --initial-branch, so the branch name comes from
// whatever git itself would choose. The README's quick start assumes main,
// so a machine defaulting to master still has to reach a usable flow.
func TestInitWithoutInitialBranchUsesGitDefault(t *testing.T) {
	dir := testutil.NewDir(t)

	run(t, dir, "init", "--author-name=Test User", "--author-email=test@example.invalid")

	if got := dir.CurrentBranch(); got != "main" {
		t.Fatalf("branch = %q, want main (the isolated config sets init.defaultBranch)", got)
	}

	run(t, dir, "flow", "init")
	if got := dir.CurrentBranch(); got != "develop" {
		t.Errorf("current branch = %q, want develop after `gix flow init`", got)
	}
}

// A repo whose main branch is called master is still usable: flow init
// should adopt it instead of creating a second base branch.
func TestFlowInitAdoptsMasterAsMain(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Run("branch", "-m", "master")

	run(t, repo, "flow", "init")

	out := run(t, repo, "config", "list")
	if !strings.Contains(out, "master") {
		t.Errorf("`gix config list` should show master as the main branch:\n%s", out)
	}
}

func TestInitCustomInitialBranch(t *testing.T) {
	dir := testutil.NewDir(t)

	run(t, dir, "init", "--initial-branch=trunk", "--author-name=Test User", "--author-email=test@example.invalid")

	if got := dir.CurrentBranch(); got != "trunk" {
		t.Errorf("branch = %q, want trunk", got)
	}
}

func TestInitWithoutIdentityLeavesRepoUncommitted(t *testing.T) {
	dir := testutil.NewDir(t)

	out := run(t, dir, "init")

	if !strings.Contains(out, "no git author identity is configured") {
		t.Errorf("output should explain the missing identity:\n%s", out)
	}
	if dir.HasCommits() {
		t.Error("repo has a commit even though no identity is configured")
	}
}

func TestInitOnExistingRepoIsIdempotent(t *testing.T) {
	repo := testutil.NewRepo(t)

	out := run(t, repo, "init")

	if !strings.Contains(out, "repository already initialized") {
		t.Errorf("`gix init` on a repo with commits should say it's already initialized:\n%s", out)
	}
	if !strings.Contains(out, "next: gix flow init") {
		t.Errorf("output should point at `gix flow init`:\n%s", out)
	}
	if got := repo.CommitCount(); got != "1" {
		t.Errorf("commit count = %q, want the existing commit left alone", got)
	}
}

func TestInitIgnoresInitialBranchOnExistingRepo(t *testing.T) {
	repo := testutil.NewRepo(t)

	out := run(t, repo, "init", "--initial-branch=trunk")

	if !strings.Contains(out, "ignoring --initial-branch") {
		t.Errorf("`gix init --initial-branch` on an existing repo should say it was ignored:\n%s", out)
	}
}

func TestInitNoCommit(t *testing.T) {
	dir := testutil.NewDir(t)

	out := run(t, dir, "init", "--no-commit")

	if !strings.Contains(out, "--no-commit was passed") {
		t.Errorf("`gix init --no-commit` should say it skipped the commit:\n%s", out)
	}
	if dir.HasCommits() {
		t.Error("repo has a commit even though --no-commit was passed")
	}
}

func TestInitAuthorFlagsMustBePaired(t *testing.T) {
	repo := testutil.NewRepo(t)

	runFailsWith(t, repo, "must be given together", "init", "--author-name=Only Name")
	runFailsWith(t, repo, "must be given together", "init", "--author-email=only@example.invalid")
}

func TestStatus(t *testing.T) {
	repo := gixRepo(t)

	out := run(t, repo, "status")

	for _, want := range []string{"on develop", "the develop branch", "working tree clean"} {
		if !strings.Contains(out, want) {
			t.Errorf("`gix status` output is missing %q:\n%s", want, out)
		}
	}
}

func TestStatusOnMain(t *testing.T) {
	repo := gixRepo(t)
	repo.Checkout("main")

	out := run(t, repo, "status")

	if !strings.Contains(out, "the main branch") {
		t.Errorf("`gix status` on main should say it is the main branch:\n%s", out)
	}
}

func TestStatusOnTopicBranch(t *testing.T) {
	repo := gixRepo(t)

	run(t, repo, "feature", "start", "api")
	repo.Write("api.txt", "work\n")
	repo.Commit("feat: api")

	out := run(t, repo, "status")

	if !strings.Contains(out, "a feature branch, based on develop") {
		t.Errorf("`gix status` should place the branch in the flow:\n%s", out)
	}
	if !strings.Contains(out, "1 ahead") {
		t.Errorf("`gix status` should report being ahead of develop:\n%s", out)
	}
}

func TestStatusDirtyTree(t *testing.T) {
	repo := gixRepo(t)
	repo.Write("wip.txt", "wip\n")

	out := run(t, repo, "status")

	if !strings.Contains(out, "uncommitted changes") {
		t.Errorf("`gix status` should report a dirty tree:\n%s", out)
	}
}

func TestStatusDetachedHead(t *testing.T) {
	repo := gixRepo(t)
	repo.Run("checkout", "--detach")

	out := run(t, repo, "status")

	if !strings.Contains(out, "detached HEAD") {
		t.Errorf("`gix status` on a detached HEAD should say so:\n%s", out)
	}
}

func TestStatusWarnsAboutMissingBase(t *testing.T) {
	repo := gixRepo(t)
	repo.Checkout("main")
	repo.Run("branch", "-D", "develop")

	out := run(t, repo, "status")

	if !strings.Contains(out, "does not exist in this repository") {
		t.Errorf("`gix status` should warn about the missing base branch:\n%s", out)
	}
}

func TestList(t *testing.T) {
	repo := gixRepo(t)

	run(t, repo, "feature", "start", "api")
	repo.Write("api.txt", "work\n")
	repo.Commit("feat: api")

	out := run(t, repo, "list")

	if !strings.Contains(out, "feature") || !strings.Contains(out, "api") {
		t.Errorf("`gix list` should show the open feature branch:\n%s", out)
	}
}

func TestListWithNothingOpen(t *testing.T) {
	repo := gixRepo(t)

	out := run(t, repo, "list")

	if !strings.Contains(out, "no feature/bugfix/hotfix/release branches yet") {
		t.Errorf("`gix list` on a fresh flow should say there's nothing open:\n%s", out)
	}
}

func TestListTree(t *testing.T) {
	repo := gixRepo(t)

	run(t, repo, "feature", "start", "api")
	run(t, repo, "bugfix", "start", "crash")
	repo.Checkout("develop")

	out := run(t, repo, "list", "--tree")

	for _, want := range []string{"main", "develop", "feature/", "bugfix/", "(current)"} {
		if !strings.Contains(out, want) {
			t.Errorf("`gix list --tree` output is missing %q:\n%s", want, out)
		}
	}
}

func TestFeatureStartFinish(t *testing.T) {
	repo := gixRepo(t)

	out := run(t, repo, "feature", "start", "api")
	if !strings.Contains(out, "switched to new branch feature/api") {
		t.Errorf("`gix feature start` output is unexpected:\n%s", out)
	}
	if got := repo.CurrentBranch(); got != "feature/api" {
		t.Errorf("current branch = %q, want feature/api", got)
	}

	repo.Write("api.txt", "work\n")
	repo.Commit("feat: api")

	out = run(t, repo, "feature", "finish", "api")
	if !strings.Contains(out, "finished feature/api") {
		t.Errorf("`gix feature finish` output is unexpected:\n%s", out)
	}
	if !strings.Contains(out, "merged into develop") {
		t.Errorf("`gix feature finish` should report the merge:\n%s", out)
	}
	if got := repo.CurrentBranch(); got != "develop" {
		t.Errorf("current branch = %q, want develop", got)
	}
}

func TestFeatureFinishNoDelete(t *testing.T) {
	repo := gixRepo(t)

	run(t, repo, "feature", "start", "api")
	repo.Write("api.txt", "work\n")
	repo.Commit("feat: api")
	run(t, repo, "feature", "finish", "api", "--no-delete")

	if got := repo.CurrentBranch(); got != "develop" {
		t.Fatalf("current branch = %q, want develop", got)
	}
	if _, err := repo.Try("show-ref", "--verify", "--quiet", "refs/heads/feature/api"); err != nil {
		t.Error("--no-delete still removed the branch")
	}
}

func TestFeatureStartRejectsExistingBranch(t *testing.T) {
	repo := gixRepo(t)

	run(t, repo, "feature", "start", "api")
	run(t, repo, "feature", "finish", "api", "--no-delete")

	runFailsWith(t, repo, "already exists", "feature", "start", "api")
}

func TestFinishUnknownBranch(t *testing.T) {
	repo := gixRepo(t)

	runFailsWith(t, repo, "does not exist", "feature", "finish", "ghost")
}

func TestStartAndFinishRefuseDirtyTree(t *testing.T) {
	repo := gixRepo(t)

	repo.Write("wip.txt", "wip\n")
	runFailsWith(t, repo, "working tree is not clean", "feature", "start", "api")

	repo.Remove("wip.txt")
	run(t, repo, "feature", "start", "api")
	repo.Write("api.txt", "work\n")
	repo.Commit("feat: api")
	repo.Write("wip.txt", "wip\n")
	runFailsWith(t, repo, "working tree is not clean", "feature", "finish", "api")
}

func TestReleaseBranchLifecycle(t *testing.T) {
	repo := gixRepo(t)

	run(t, repo, "release", "start", "v1.0.0")
	repo.Write("notes.md", "release notes\n")
	repo.Commit("chore: release notes")

	out := run(t, repo, "release", "finish", "v1.0.0", "--tag=v1.0.0")

	if !strings.Contains(out, "tagged v1.0.0 on main") {
		t.Errorf("`gix release finish --tag` should report the tag:\n%s", out)
	}
	if got := repo.CurrentBranch(); got != "main" {
		t.Errorf("current branch = %q, want main", got)
	}
	if _, err := repo.Try("rev-parse", "--verify", "refs/tags/v1.0.0"); err != nil {
		t.Error("the tag does not exist after `gix release finish --tag`")
	}
}

func TestReleaseRefusesTagOnKindWithoutTagSupport(t *testing.T) {
	repo := gixRepo(t)

	run(t, repo, "feature", "start", "api")
	repo.Write("api.txt", "work\n")
	repo.Commit("feat: api")

	run(t, repo, "feature", "finish", "api")
}

func TestReleaseStartRejectsDuplicateTag(t *testing.T) {
	repo := gixRepo(t)
	repo.Run("tag", "v1.0.0")

	run(t, repo, "release", "start", "v1.0.0")
	repo.Write("notes.md", "notes\n")
	repo.Commit("chore: notes")

	runFailsWith(t, repo, "already exists", "release", "finish", "v1.0.0", "--tag=v1.0.0")
}

func TestConfigList(t *testing.T) {
	repo := gixRepo(t)

	out := run(t, repo, "config", "list")

	for _, want := range []string{"remote", "origin", "main", "develop", "prefix", "feature/", "parent"} {
		if !strings.Contains(out, want) {
			t.Errorf("`gix config list` output is missing %q:\n%s", want, out)
		}
	}
}

func TestConfigSet(t *testing.T) {
	repo := gixRepo(t)

	out := run(t, repo, "config", "set", "feature", "--prefix=feat/", "--upstream-strategy=squash", "--delete-on-finish=false")
	if !strings.Contains(out, "updated feature") {
		t.Errorf("`gix config set` should confirm the update:\n%s", out)
	}

	out = run(t, repo, "config", "list")
	if !strings.Contains(out, "feat/") {
		t.Errorf("`gix config list` should show the new prefix:\n%s", out)
	}

	// config set leaves the edit in the working tree, so commit it before
	// asking gix to mutate anything.
	repo.Commit("chore(gix): update flow config")

	run(t, repo, "feature", "start", "api")
	if got := repo.CurrentBranch(); got != "feat/api" {
		t.Errorf("current branch = %q, want the new prefix to be used", got)
	}
}

func TestConfigSetTagAndDeleteFlags(t *testing.T) {
	repo := gixRepo(t)

	run(t, repo, "config", "set", "hotfix", "--tag=false", "--delete-on-finish=false")

	out := run(t, repo, "config", "list")
	if !strings.Contains(out, "false") {
		t.Errorf("`gix config list` should reflect the new booleans:\n%s", out)
	}
}

func TestConfigSetValidation(t *testing.T) {
	repo := gixRepo(t)

	runFailsWith(t, repo, "invalid --upstream-strategy", "config", "set", "feature", "--upstream-strategy=nope")
	runFailsWith(t, repo, "invalid --downstream-strategy", "config", "set", "feature", "--downstream-strategy=nope")
	runFailsWith(t, repo, "want true or false", "config", "set", "feature", "--tag=maybe")
	runFailsWith(t, repo, "unknown branch", "config", "set", "nope", "--tag=true")
	runFailsWith(t, repo, "no changes given", "config", "set", "feature")
}

// Topic-kind settings must not be applied to a base branch.
func TestConfigSetRejectsTopicFlagsOnABaseBranch(t *testing.T) {
	repo := gixRepo(t)

	for _, args := range [][]string{
		{"config", "set", "main", "--tag=true"},
		{"config", "set", "develop", "--prefix=feat/"},
		{"config", "set", "develop", "--upstream-strategy=rebase"},
		{"config", "set", "develop", "--delete-on-finish=false"},
	} {
		runFailsWith(t, repo, "is a base branch", args...)
	}

	// Nothing may have been written by those attempts.
	out := run(t, repo, "config", "list")
	if !strings.Contains(out, "feature/") {
		t.Errorf("a rejected base-branch edit changed the config:\n%s", out)
	}
}

func TestConfigSetAutosyncOnABaseBranch(t *testing.T) {
	repo := gixRepo(t)

	run(t, repo, "config", "set", "develop", "--autosync=false")
	repo.Commit("chore(gix): unmark develop")

	if !strings.Contains(run(t, repo, "branch", "sync", "--from=main", "--auto"), "nothing to sync") {
		t.Error("develop still receives autosync after being unmarked")
	}

	run(t, repo, "config", "set", "develop", "--autosync=true")
	repo.Commit("chore(gix): mark develop")
}

// An invalid prefix must be refused before anything is written, leaving the
// config on disk loadable.
func TestConfigSetRejectsInvalidPrefix(t *testing.T) {
	repo := gixRepo(t)

	runFailsWith(t, repo, "not a valid branch name component", "config", "set", "feature", "--prefix=bad prefix/")

	out := run(t, repo, "config", "list")
	if !strings.Contains(out, "feature/") {
		t.Errorf("the rejected prefix must not have been written:\n%s", out)
	}
}

func TestHelp(t *testing.T) {
	repo := testutil.NewRepo(t)

	for _, args := range [][]string{
		{"--help"},
		{"init", "--help"},
		{"flow", "--help"},
		{"flow", "init", "--help"},
		{"feature", "--help"},
		{"feature", "start", "--help"},
		{"feature", "finish", "--help"},
		{"status", "--help"},
		{"list", "--help"},
		{"config", "--help"},
		{"config", "list", "--help"},
		{"config", "set", "--help"},
	} {
		run(t, repo, args...)
	}
}

func TestUnknownCommand(t *testing.T) {
	repo := testutil.NewRepo(t)

	runFails(t, repo, "definitely-not-a-command")
}
