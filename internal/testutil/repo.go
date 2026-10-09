// Package testutil sets up throwaway Git repositories for tests.
//
// gix shells out to the real `git` binary and reads config from the
// process environment, so tests must control both: a temp directory to
// work in, and an isolated Git config that ignores whatever the
// developer's machine (or CI runner) happens to have configured.
package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Repo is a throwaway Git repository with one commit on `main`.
type Repo struct {
	t    *testing.T
	Root string
}

// NewDir switches the test into an empty directory that is not a Git
// repository yet, with Git's config isolated the same way NewRepo does,
// and returns a handle to it.
func NewDir(t *testing.T) *Repo {
	t.Helper()

	requireGit(t)

	dir := t.TempDir()
	isolateEnv(t)
	t.Chdir(dir)

	return &Repo{t: t, Root: dir}
}

// requireGit skips rather than fails: gix shells out to the real git
// binary, so there is nothing these tests can check without it.
func requireGit(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
}

// NewRepo creates an isolated repo and switches the test into it.
// The caller's environment is left as it was when the test ends.
func NewRepo(t *testing.T) *Repo {
	t.Helper()

	r := NewDir(t)
	r.Run("init", "-b", "main")
	r.Run("config", "user.name", "gix test")
	r.Run("config", "user.email", "gix-test@example.invalid")
	r.Run("config", "commit.gpgsign", "false")
	r.Run("config", "tag.gpgsign", "false")

	r.Write("README.md", "gix test repo\n")
	r.Commit("initial commit")

	// Temp paths can be symlinked (/var on macOS), so trust git's answer
	// over the path we asked for.
	r.Root = r.Output("rev-parse", "--show-toplevel")

	return r
}

// isolateEnv points Git at a config that belongs to this test alone,
// leaving system and global config unreadable so a developer's own
// user.name, hooksPath, or signing setup cannot change test results.
func isolateEnv(t *testing.T) {
	t.Helper()

	// Keep the global config outside the working directory: a file inside
	// the repo would get committed along with the test's own changes.
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, []byte("[init]\n\tdefaultBranch = main\n"), 0o644); err != nil {
		t.Fatalf("writing isolated git config: %v", err)
	}

	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_TERMINAL_PROMPT", "0")

	// Setting these to "" is not the same as clearing them: git reads an
	// empty GIT_DIR as a path and fails with "the empty string is not a
	// valid path". Remove them for the duration of the test instead.
	for _, name := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE"} {
		unsetenv(t, name)
	}
}

// unsetenv removes a variable for the duration of the test, restoring
// whatever the developer or CI runner had set afterwards.
func unsetenv(t *testing.T, name string) {
	t.Helper()

	old, had := os.LookupEnv(name)
	if err := os.Unsetenv(name); err != nil {
		t.Fatalf("unsetting %s: %v", name, err)
	}
	t.Cleanup(func() {
		if had {
			os.Setenv(name, old)
			return
		}
		os.Unsetenv(name)
	})
}

// Run executes a git command in the repo and fails the test if it errors.
func (r *Repo) Run(args ...string) {
	r.t.Helper()
	if out, err := r.try(args...); err != nil {
		r.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// Output executes a git command and returns its trimmed stdout.
func (r *Repo) Output(args ...string) string {
	r.t.Helper()
	out, err := r.try(args...)
	if err != nil {
		r.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

// Try executes a git command and returns stdout plus any error, for
// tests that assert on Git refusing to do something.
func (r *Repo) Try(args ...string) (string, error) {
	r.t.Helper()
	return r.try(args...)
}

func (r *Repo) try(args ...string) (string, error) {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return strings.TrimSpace(string(out) + "\n" + stderr.String()), err
	}
	return strings.TrimSpace(string(out)), nil
}

// Write creates or replaces a file in the working tree.
func (r *Repo) Write(name, content string) {
	r.t.Helper()
	full := filepath.Join(r.Root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		r.t.Fatalf("creating dir for %s: %v", name, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		r.t.Fatalf("writing %s: %v", name, err)
	}
}

// Remove deletes a file from the working tree, staged or not.
func (r *Repo) Remove(name string) {
	r.t.Helper()
	if err := os.Remove(filepath.Join(r.Root, filepath.FromSlash(name))); err != nil && !os.IsNotExist(err) {
		r.t.Fatalf("removing %s: %v", name, err)
	}
}

// Commit stages everything and records a commit.
func (r *Repo) Commit(message string) {
	r.t.Helper()
	r.Run("add", "-A")
	r.Run("commit", "-m", message)
}

// Branch creates a branch at HEAD without switching to it.
func (r *Repo) Branch(name string) {
	r.t.Helper()
	r.Run("branch", name)
}

// Checkout switches to an existing branch.
func (r *Repo) Checkout(name string) {
	r.t.Helper()
	r.Run("checkout", name)
}

// CurrentBranch returns the checked-out branch name.
func (r *Repo) CurrentBranch() string {
	r.t.Helper()
	return r.Output("rev-parse", "--abbrev-ref", "HEAD")
}

// Clean reports whether the working tree has no uncommitted changes.
func (r *Repo) Clean() bool {
	r.t.Helper()
	out, err := r.Try("status", "--porcelain")
	return err == nil && out == ""
}

// CommitOn commits a change to the given file while on the given branch,
// creating the branch from HEAD first if it doesn't exist.
func (r *Repo) CommitOn(branch, file, content, message string) {
	r.t.Helper()
	if r.CurrentBranch() != branch {
		if !r.hasBranch(branch) {
			r.Branch(branch)
		}
		r.Checkout(branch)
	}
	r.Write(file, content)
	r.Commit(message)
}

// CommitCount returns how many commits HEAD has.
func (r *Repo) CommitCount() string {
	r.t.Helper()
	return r.Output("rev-list", "--count", "HEAD")
}

// HasCommits reports whether HEAD resolves to a commit yet.
func (r *Repo) HasCommits() bool {
	r.t.Helper()
	_, err := r.Try("rev-parse", "--verify", "-q", "HEAD")
	return err == nil
}

func (r *Repo) hasBranch(name string) bool {
	_, err := r.Try("show-ref", "--verify", "--quiet", "refs/heads/"+name)
	return err == nil
}
