package commands

import (
	"strings"
	"testing"

	"github.com/m-mdy-m/gix/internal/testutil"
)

// taggedRepo is a repo with the flow set up, main checked out, and one
// release tag already made.
func taggedRepo(t *testing.T, tag string) *testutil.Repo {
	t.Helper()

	repo := gixRepo(t)
	if tag != "" {
		repo.Checkout("main")
		repo.Run("tag", "-a", tag, "-m", "release")
	}
	return repo
}

func TestVersionShow(t *testing.T) {
	repo := taggedRepo(t, "v0.2.0")

	out := run(t, repo, "version", "show")

	for _, want := range []string{"version", "current", "0.2.0", "v0.2.0"} {
		if !strings.Contains(out, want) {
			t.Errorf("`gix version show` output is missing %q:\n%s", want, out)
		}
	}
}

func TestVersionShowWithNoTags(t *testing.T) {
	repo := gixRepo(t)

	out := run(t, repo, "version", "show")

	if !strings.Contains(out, "no release tags yet") {
		t.Errorf("`gix version show` should say there is no release yet:\n%s", out)
	}
	if !strings.Contains(out, "v0.1.0") {
		t.Errorf("`gix version show` should say what the first release would be:\n%s", out)
	}
}

func TestVersionBumpDryRun(t *testing.T) {
	repo := taggedRepo(t, "v0.2.0")

	before := repo.Output("for-each-ref", "--format=%(refname)", "refs/tags/")

	out := run(t, repo, "version", "bump", "minor", "--dry-run")

	for _, want := range []string{"dry run", "0.2.0", "0.3.0", "v0.3.0", "main"} {
		if !strings.Contains(out, want) {
			t.Errorf("`gix version bump minor --dry-run` output is missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "nothing was created") {
		t.Errorf("a dry run should say it created nothing:\n%s", out)
	}

	after := repo.Output("for-each-ref", "--format=%(refname)", "refs/tags/")
	if after != before {
		t.Errorf("a dry run created tags:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestVersionBumpLevels(t *testing.T) {
	tests := []struct {
		level string
		want  string
	}{
		{level: "patch", want: "v0.2.1"},
		{level: "minor", want: "v0.3.0"},
		{level: "major", want: "v1.0.0"},
	}

	for _, tc := range tests {
		t.Run(tc.level, func(t *testing.T) {
			repo := taggedRepo(t, "v0.2.0")

			out := run(t, repo, "version", "bump", tc.level, "--dry-run")

			if !strings.Contains(out, tc.want) {
				t.Errorf("`gix version bump %s` should produce %s:\n%s", tc.level, tc.want, out)
			}
		})
	}
}

// The 0.9 → 0.10 case is where naive bumping gets it wrong.
func TestVersionBumpCrossesDoubleDigits(t *testing.T) {
	repo := taggedRepo(t, "v0.9.4")

	out := run(t, repo, "version", "bump", "minor", "--dry-run")

	if !strings.Contains(out, "v0.10.0") {
		t.Errorf("0.9.4 minor should be 0.10.0, not 0.9.5:\n%s", out)
	}
}

func TestVersionBumpCreatesTag(t *testing.T) {
	repo := taggedRepo(t, "v0.2.0")

	head := repo.Output("rev-parse", "HEAD")
	commits := repo.Output("rev-list", "--count", "HEAD")

	out := run(t, repo, "version", "bump", "minor")

	if !strings.Contains(out, "created v0.3.0") {
		t.Errorf("`gix version bump minor` should confirm the tag:\n%s", out)
	}
	if !strings.Contains(out, "git push origin v0.3.0") {
		t.Errorf("output should say how to push the tag:\n%s", out)
	}

	if _, err := repo.Try("rev-parse", "--verify", "refs/tags/v0.3.0"); err != nil {
		t.Error("the tag was not created")
	}
	if got := repo.Output("rev-parse", "HEAD"); got != head {
		t.Error("bump moved HEAD")
	}
	if got := repo.Output("rev-list", "--count", "HEAD"); got != commits {
		t.Errorf("commit count = %s, want %s: bump must not commit", got, commits)
	}
}

func TestVersionBumpIsAnAnnotatedTag(t *testing.T) {
	repo := taggedRepo(t, "v0.2.0")

	run(t, repo, "version", "bump", "patch")

	kind := repo.Output("cat-file", "-t", "v0.2.1")
	if kind != "tag" {
		t.Errorf("v0.2.1 is a %s, want an annotated tag", kind)
	}
	if msg := repo.Output("tag", "-l", "--format=%(contents)", "v0.2.1"); msg == "" {
		t.Error("the annotated tag has no message")
	}
}

func TestVersionBumpFirstRelease(t *testing.T) {
	repo := gixRepo(t)
	repo.Checkout("main")

	out := run(t, repo, "version", "bump", "minor", "--dry-run")

	if !strings.Contains(out, "first release") {
		t.Errorf("with no tags, output should say it is the first release:\n%s", out)
	}
	if !strings.Contains(out, "v0.1.0") {
		t.Errorf("the first minor release should be v0.1.0:\n%s", out)
	}
}

func TestVersionBumpRejectsUnknownLevel(t *testing.T) {
	repo := taggedRepo(t, "v0.2.0")

	runFailsWith(t, repo, "unknown version level", "version", "bump", "minorish")
}

func TestVersionBumpRejectsExtraArgs(t *testing.T) {
	repo := taggedRepo(t, "v0.2.0")

	runFails(t, repo, "version", "bump", "minor", "extra")
	runFails(t, repo, "version", "bump")
}

// gix puts release tags on main, so releasing from anywhere else is
// refused rather than warned about.
func TestVersionBumpRefusesANonMainBranch(t *testing.T) {
	repo := taggedRepo(t, "v0.2.0")
	run(t, repo, "feature", "start", "api")
	repo.Write("api.txt", "api\n")
	repo.Commit("feat: api")

	runFailsWith(t, repo, "releases are cut on main", "version", "bump", "minor")

	if _, err := repo.Try("rev-parse", "--verify", "refs/tags/v0.3.0"); err == nil {
		t.Error("a tag was created despite the refusal")
	}
}

func TestVersionBumpRefusesADirtyTree(t *testing.T) {
	repo := taggedRepo(t, "v0.2.0")
	repo.Write("wip.txt", "uncommitted\n")

	runFailsWith(t, repo, "working tree is not clean", "version", "bump", "minor")
}

func TestVersionBumpRefusesDetachedHead(t *testing.T) {
	repo := taggedRepo(t, "v0.2.0")
	repo.Run("checkout", "--detach")

	runFailsWith(t, repo, "detached HEAD", "version", "bump", "minor")
}

// Nothing may be pushed, committed, or written by gix. The user decides.
func TestVersionBumpDoesNothingElse(t *testing.T) {
	repo := taggedRepo(t, "v0.2.0")

	filesBefore := repo.Output("status", "--porcelain")
	branch := repo.CurrentBranch()

	run(t, repo, "version", "bump", "patch")

	if got := repo.Output("status", "--porcelain"); got != filesBefore {
		t.Errorf("bump changed the working tree:\nbefore:\n%s\nafter:\n%s", filesBefore, got)
	}
	if got := repo.CurrentBranch(); got != branch {
		t.Errorf("current branch = %q, want %q", got, branch)
	}
	if _, err := repo.Try("rev-parse", "--verify", "refs/remotes/origin/main"); err == nil {
		t.Error("a remote-tracking ref appeared, so something pushed")
	}
}

func TestVersionBumpDoesNotEditTheChangelog(t *testing.T) {
	repo := taggedRepo(t, "v0.2.0")

	writeFile(t, repo, "CHANGELOG.md", "# Changelog\n\n## [0.2.0]\n\nOld.\n")
	repo.Commit("docs: changelog")

	run(t, repo, "version", "bump", "minor")

	after, err := readFile(repo, "CHANGELOG.md")
	if err != nil {
		t.Fatalf("reading the changelog: %v", err)
	}
	if strings.Contains(after, "0.3.0") {
		t.Errorf("bump edited the changelog, which is the user's job:\n%s", after)
	}
}

func TestVersionCommandsNeedARepo(t *testing.T) {
	testutil.NewDir(t)

	runFailsWith(t, nil, "not a git repository", "version", "show")
	runFailsWith(t, nil, "not a git repository", "version", "bump", "minor")
}

func TestVersionCommandsNeedGixConfig(t *testing.T) {
	repo := testutil.NewRepo(t)

	runFailsWith(t, repo, "gix is not initialized", "version", "bump", "minor")
}

func TestVersionGroupIsRegistered(t *testing.T) {
	root := New()

	for _, c := range root.Commands() {
		if c.Name() != "version" {
			continue
		}
		want := map[string]bool{"show": false, "bump": false}
		for _, sub := range c.Commands() {
			if _, ok := want[sub.Name()]; ok {
				want[sub.Name()] = true
			}
			if sub.Short == "" {
				t.Errorf("`gix version %s` has no Short description", sub.Name())
			}
		}
		for name, found := range want {
			if !found {
				t.Errorf("`gix version %s` is not registered", name)
			}
		}
		return
	}
	t.Error("`gix version` is not registered")
}

func TestVersionHelp(t *testing.T) {
	repo := taggedRepo(t, "v0.2.0")

	for _, args := range [][]string{
		{"version", "--help"},
		{"version", "show", "--help"},
		{"version", "bump", "--help"},
	} {
		run(t, repo, args...)
	}
}

// `gix version bump` must not promise anything it does not do.
func TestVersionBumpHelpIsHonest(t *testing.T) {
	root := New()

	for _, c := range root.Commands() {
		if c.Name() != "version" {
			continue
		}
		for _, sub := range c.Commands() {
			if sub.Name() != "bump" {
				continue
			}
			lower := strings.ToLower(sub.Long)
			for _, claim := range []string{
				"updates the changelog",
				"writes a commit",
				"pushes",
				"signs the tag",
			} {
				if strings.Contains(lower, claim) {
					t.Errorf("`gix version bump` help claims it %q, which it does not do", claim)
				}
			}
			for _, honest := range []string{"does not push", "does not edit changelog", "dry-run"} {
				if !strings.Contains(lower, strings.ToLower(honest)) {
					t.Errorf("`gix version bump` help should state it %s", honest)
				}
			}
			return
		}
	}
}

// A configured kind must not be able to shadow `gix version`.
func TestConfiguredKindCannotShadowVersion(t *testing.T) {
	repo := gixRepo(t)

	raw, err := readFile(repo, ".gix/config")
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	entry := "  version:\n    type: topic\n    prefix: versionbranch/\n    parent: develop\n"
	writeFile(t, repo, ".gix/config", strings.Replace(raw, "branch:\n", "branch:\n"+entry, 1))
	repo.Commit("chore(gix): add a colliding kind")

	for _, c := range New().Commands() {
		if c.Name() != "version" {
			continue
		}
		for _, sub := range c.Commands() {
			if sub.Name() == "start" || sub.Name() == "finish" {
				t.Fatal("`gix version` was replaced by a branch kind group")
			}
		}
		return
	}
	t.Error("`gix version` disappeared")
}

func TestVersionBumpStartsAPrerelease(t *testing.T) {
	repo := taggedRepo(t, "v0.2.0")

	out := run(t, repo, "version", "bump", "minor", "--pre", "rc", "--dry-run")

	for _, want := range []string{"v0.3.0-rc.1", "prerelease of 0.3.0"} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
	if _, err := repo.Try("rev-parse", "--verify", "refs/tags/v0.3.0-rc.1"); err == nil {
		t.Error("a dry run created the tag")
	}
}

func TestVersionBumpPrereleaseLifecycle(t *testing.T) {
	repo := taggedRepo(t, "v0.9.0")

	for _, step := range []struct {
		args []string
		want string
	}{
		{[]string{"major", "--pre", "rc"}, "created v1.0.0-rc.1"},
		{[]string{"--pre", "rc"}, "created v1.0.0-rc.2"},
		{[]string{"--release"}, "created v1.0.0"},
	} {
		out := run(t, repo, append([]string{"version", "bump"}, step.args...)...)
		if !strings.Contains(out, step.want) {
			t.Errorf("`gix version bump %v` should print %q:\n%s", step.args, step.want, out)
		}
	}
}

func TestVersionBumpMinorOnAPrereleaseDoesNotPromote(t *testing.T) {
	repo := taggedRepo(t, "v1.0.0-rc.1")

	out := run(t, repo, "version", "bump", "minor", "--dry-run")

	if !strings.Contains(out, "v1.1.0") {
		t.Errorf("1.0.0-rc.1 minor should be v1.1.0:\n%s", out)
	}
}

func TestVersionShowReportsAPrerelease(t *testing.T) {
	repo := taggedRepo(t, "v1.0.0-rc.1")

	out := run(t, repo, "version", "show")

	if !strings.Contains(out, "1.0.0-rc.1") {
		t.Errorf("`gix version show` should report the prerelease:\n%s", out)
	}
}

func TestVersionBumpPrereleaseMisuse(t *testing.T) {
	repo := taggedRepo(t, "v0.2.0")

	runFailsWith(t, repo, "not a prerelease", "version", "bump", "--pre", "rc")
	runFailsWith(t, repo, "not a prerelease", "version", "bump", "--release")
	runFailsWith(t, repo, "invalid prerelease name", "version", "bump", "minor", "--pre", "1")
	runFails(t, repo, "version", "bump", "minor", "--pre", "rc", "--release")
	runFails(t, repo, "version", "bump", "--release", "--pre", "rc")
}

func TestVersionBumpRefusesGoingBackwards(t *testing.T) {
	repo := taggedRepo(t, "v1.0.0-rc.1")

	runFailsWith(t, repo, "would not be higher", "version", "bump", "--pre", "beta")
}
