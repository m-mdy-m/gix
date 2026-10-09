package flow

import (
	"strings"
	"testing"

	"github.com/m-mdy-m/gix/internal/config"
	"github.com/m-mdy-m/gix/internal/git"
	"github.com/m-mdy-m/gix/internal/testutil"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		in      string
		want    Version
		wantErr bool
	}{
		{in: "0.0.0", want: Version{}},
		{in: "1.2.3", want: Version{Major: 1, Minor: 2, Patch: 3}},
		{in: "10.20.30", want: Version{Major: 10, Minor: 20, Patch: 30}},
		{in: "1.0.0", want: Version{Major: 1}},
		// Anything gix can't read exactly is rejected rather than guessed,
		// so a tag like these can't become the base of a release.
		{in: "1.2", wantErr: true},
		{in: "1.2.3.4", wantErr: true},
		{in: "1.2.x", wantErr: true},
		{in: "v1.2.3", wantErr: true},
		{in: "1.2.3+build", wantErr: true},
		{in: "1.2.3-rc.1+build", wantErr: true},
		{in: "1.2.3-", wantErr: true},
		{in: "1.2.3-rc..1", wantErr: true},
		{in: "1.2.3-rc.01", wantErr: true},
		{in: "1.2.3-rc.1.", wantErr: true},
		{in: "1.2.3-rc_1", wantErr: true},
		{in: "1.2.3-rc 1", wantErr: true},
		{in: "1.2-rc.1", wantErr: true},
		{in: "1.2.3-rc.1", want: Version{Major: 1, Minor: 2, Patch: 3, Pre: "rc.1"}},
		{in: "1.0.0-alpha", want: Version{Major: 1, Pre: "alpha"}},
		{in: "1.0.0-x.7.z.92", want: Version{Major: 1, Pre: "x.7.z.92"}},
		{in: "1.0.0-0.3.7", want: Version{Major: 1, Pre: "0.3.7"}},
		{in: "1.0.0-rc-1", want: Version{Major: 1, Pre: "rc-1"}},
		{in: "1.0.0-0", want: Version{Major: 1, Pre: "0"}},
		{in: "latest", wantErr: true},
		{in: "", wantErr: true},
		{in: "-1.0.0", wantErr: true},
		{in: "1.-2.3", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseVersion(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseVersion(%q) = %v, want an error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseVersion(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("ParseVersion(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestVersionBump(t *testing.T) {
	tests := []struct {
		start Version
		level Level
		want  string
	}{
		{start: Version{Major: 0, Minor: 4, Patch: 3}, level: LevelPatch, want: "0.4.4"},
		{start: Version{Major: 0, Minor: 4, Patch: 3}, level: LevelMinor, want: "0.5.0"},
		{start: Version{Major: 0, Minor: 4, Patch: 3}, level: LevelMajor, want: "1.0.0"},
		// The case that catches naive string bumping.
		{start: Version{Major: 0, Minor: 9, Patch: 4}, level: LevelMinor, want: "0.10.0"},
		{start: Version{Major: 0, Minor: 9, Patch: 9}, level: LevelPatch, want: "0.9.10"},
		{start: Version{Major: 0, Minor: 9, Patch: 9}, level: LevelMajor, want: "1.0.0"},
		{start: Version{Major: 1, Minor: 0, Patch: 0}, level: LevelPatch, want: "1.0.1"},
		{start: Version{Major: 1, Minor: 0, Patch: 0}, level: LevelMinor, want: "1.1.0"},
	}

	for _, tc := range tests {
		t.Run(tc.start.String()+" "+string(tc.level), func(t *testing.T) {
			if got := tc.start.Bump(tc.level).String(); got != tc.want {
				t.Errorf("Bump(%q) = %q, want %q", tc.start, got, tc.want)
			}
		})
	}
}

func TestVersionTag(t *testing.T) {
	if got := (Version{Major: 1, Minor: 2, Patch: 3}).Tag(); got != "v1.2.3" {
		t.Errorf("Tag() = %q, want %q", got, "v1.2.3")
	}
}

func TestLevelValid(t *testing.T) {
	for _, l := range []Level{LevelPatch, LevelMinor, LevelMajor} {
		if !l.Valid() {
			t.Errorf("Level(%q).Valid() = false, want true", l)
		}
	}
	for _, l := range []Level{"", "minorish", "PATCH"} {
		if l.Valid() {
			t.Errorf("Level(%q).Valid() = true, want false", l)
		}
	}
}

func TestLatestVersionNoTags(t *testing.T) {
	testutil.NewRepo(t)

	v, tag, found, err := LatestVersion()
	if err != nil {
		t.Fatalf("LatestVersion: %v", err)
	}
	if found {
		t.Errorf("LatestVersion found %q on an untagged repo", tag)
	}
	if v != (Version{}) {
		t.Errorf("version = %v, want the zero version when nothing is tagged", v)
	}
}

func TestLatestVersionPicksTheHighest(t *testing.T) {
	repo := testutil.NewRepo(t)
	// Deliberately unsorted, and including the 0.9 vs 0.10 case that
	// alphabetical sorting gets wrong.
	for _, tag := range []string{"v0.1.0", "v0.10.0", "v0.9.0", "v1.0.0-rc.1", "nightly"} {
		repo.Run("tag", tag)
	}

	v, tag, found, err := LatestVersion()
	if err != nil {
		t.Fatalf("LatestVersion: %v", err)
	}
	if !found {
		t.Fatal("LatestVersion found nothing")
	}
	// 1.0.0-rc.1 is a version gix can read, and it is newer than 0.10.0.
	if tag != "v1.0.0-rc.1" {
		t.Errorf("tag = %q, want v1.0.0-rc.1, the highest readable version tag", tag)
	}
	if v.String() != "1.0.0-rc.1" {
		t.Errorf("version = %q, want 1.0.0-rc.1", v)
	}
}

func TestLatestVersionIgnoresUnreadableTags(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Run("tag", "v0.5.0")
	repo.Run("tag", "not-a-version")
	repo.Run("tag", "v1.2")

	v, tag, found, err := LatestVersion()
	if err != nil {
		t.Fatalf("LatestVersion: %v", err)
	}
	if !found || tag != "v0.5.0" {
		t.Errorf("tag = %q (found = %v), want the only readable tag v0.5.0", tag, found)
	}
	if v.String() != "0.5.0" {
		t.Errorf("version = %q, want 0.5.0", v)
	}
}

// A first release has no tag to bump from, so it has to start somewhere
// defined rather than from a zero version.
func TestPlanFirstRelease(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Write("a.txt", "a\n")
	repo.Commit("feat: something")
	repo.Write("b.txt", "b\n")
	repo.Commit("feat: something else")

	plan, err := PlanBump(LevelMinor)
	if err != nil {
		t.Fatalf("PlanBump: %v", err)
	}

	if plan.Tagged {
		t.Error("Tagged = true with no tags in the repository")
	}
	if plan.Next.String() != "0.1.0" {
		t.Errorf("next = %q, want 0.1.0 as the first minor release", plan.Next)
	}
	if plan.Next.Tag() != "v0.1.0" {
		t.Errorf("next tag = %q, want v0.1.0", plan.Next.Tag())
	}
	if plan.CommitsSince != 3 {
		t.Errorf("CommitsSince = %d, want the whole history (3)", plan.CommitsSince)
	}
}

func TestPlanCountsCommitsSinceTheTag(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Run("tag", "-a", "v1.0.0", "-m", "first")
	repo.Write("a.txt", "a\n")
	repo.Commit("feat: one")
	repo.Write("b.txt", "b\n")
	repo.Commit("feat: two")

	plan, err := PlanBump(LevelPatch)
	if err != nil {
		t.Fatalf("PlanBump: %v", err)
	}

	if plan.CommitsSince != 2 {
		t.Errorf("CommitsSince = %d, want 2", plan.CommitsSince)
	}
	if plan.CurrentTag != "v1.0.0" {
		t.Errorf("CurrentTag = %q, want v1.0.0", plan.CurrentTag)
	}
	if plan.Next.String() != "1.0.1" {
		t.Errorf("next = %q, want 1.0.1", plan.Next)
	}
}

// releaseRepo is a repo with the flow set up and main checked out, with
// one release tag already on main.
func releaseRepo(t *testing.T, tag string) (*testutil.Repo, *config.Config) {
	t.Helper()

	repo, cfg := flowRepo(t)
	if tag != "" {
		repo.Checkout("main")
		repo.Run("tag", "-a", tag, "-m", "release")
	}
	return repo, cfg
}

func TestBumpCreatesTag(t *testing.T) {
	_, cfg := releaseRepo(t, "v0.2.0")

	result, err := Bump(cfg, BumpOptions{Level: LevelMinor})
	if err != nil {
		t.Fatalf("Bump: %v", err)
	}

	if !result.Tagged {
		t.Error("Tagged = false after a real bump")
	}
	if result.Plan.Next.Tag() != "v0.3.0" {
		t.Errorf("next tag = %q, want v0.3.0", result.Plan.Next.Tag())
	}
	if !git.TagExists("v0.3.0") {
		t.Error("the tag was not created")
	}
	if git.TagExists("v0.2.1") {
		t.Error("an unexpected tag was created")
	}
}

// The tag is the whole point: nothing else may be touched.
func TestBumpChangesNothingElse(t *testing.T) {
	repo, cfg := releaseRepo(t, "v0.2.0")

	head := repo.Output("rev-parse", "HEAD")
	branch := repo.CurrentBranch()
	commitsBefore := repo.Output("rev-list", "--count", "HEAD")
	filesBefore := repo.Output("status", "--porcelain")

	if _, err := Bump(cfg, BumpOptions{Level: LevelMajor}); err != nil {
		t.Fatalf("Bump: %v", err)
	}

	if got := repo.Output("rev-parse", "HEAD"); got != head {
		t.Error("Bump moved HEAD")
	}
	if got := repo.CurrentBranch(); got != branch {
		t.Errorf("current branch = %q, want %q", got, branch)
	}
	if got := repo.Output("status", "--porcelain"); got != filesBefore {
		t.Errorf("Bump changed the working tree:\nbefore:\n%s\nafter:\n%s", filesBefore, got)
	}
	if got := repo.Output("rev-list", "--count", "HEAD"); got != commitsBefore {
		t.Errorf("commit count = %s, want %s: Bump must add no commit", got, commitsBefore)
	}
}

func TestBumpDryRunCreatesNothing(t *testing.T) {
	_, cfg := releaseRepo(t, "v0.2.0")

	result, err := Bump(cfg, BumpOptions{Level: LevelMinor, DryRun: true})
	if err != nil {
		t.Fatalf("Bump: %v", err)
	}

	if result.Tagged {
		t.Error("Tagged = true on a dry run")
	}
	if result.Plan.Next.Tag() != "v0.3.0" {
		t.Errorf("next tag = %q, want v0.3.0 reported without creating it", result.Plan.Next.Tag())
	}
	if git.TagExists("v0.3.0") {
		t.Error("a dry run created the tag")
	}
}

// Tagging anything other than the main branch is refused, because that is
// where gix already puts release tags.
func TestBumpRefusesAnyBranchButMain(t *testing.T) {
	// Each branch needs its own repo: moving between them would leave the
	// working tree in a state the next case would inherit.
	for _, branch := range []string{"develop", "feature/api"} {
		t.Run(branch, func(t *testing.T) {
			repo, cfg := releaseRepo(t, "v0.2.0")

			if branch == "feature/api" {
				if _, err := Start(cfg, "feature", "api"); err != nil {
					t.Fatalf("Start: %v", err)
				}
			}
			repo.Checkout(branch)

			_, err := Bump(cfg, BumpOptions{Level: LevelMinor})
			if err == nil {
				t.Fatalf("Bump succeeded on %s, want an error", branch)
			}
			if !strings.Contains(err.Error(), "releases are cut on main") {
				t.Errorf("Bump = %q, want it to say releases are cut on main", err)
			}
			if git.TagExists("v0.3.0") {
				t.Error("a tag was created despite the refusal")
			}
		})
	}
}

func TestBumpRefusesADetachedHead(t *testing.T) {
	repo, cfg := releaseRepo(t, "v0.2.0")
	repo.Run("checkout", "--detach")

	_, err := Bump(cfg, BumpOptions{Level: LevelMinor})
	if err == nil {
		t.Fatal("Bump succeeded on a detached HEAD, want an error")
	}
	if !strings.Contains(err.Error(), "detached HEAD") {
		t.Errorf("Bump = %q, want it to mention the detached HEAD", err)
	}
}

// The "tag already exists" guard in Bump cannot be reached through tags
// alone: the plan is built from the highest existing tag, so its result
// never collides. What does stop a duplicate is git itself, which is
// checked in internal/git's TestTagRefusesADuplicate.
func TestBumpInSuccessiveRuns(t *testing.T) {
	_, cfg := releaseRepo(t, "v0.2.0")

	first, err := Bump(cfg, BumpOptions{Level: LevelMinor})
	if err != nil {
		t.Fatalf("first Bump: %v", err)
	}
	if first.Plan.Next.Tag() != "v0.3.0" {
		t.Fatalf("first bump = %q, want v0.3.0", first.Plan.Next.Tag())
	}

	second, err := Bump(cfg, BumpOptions{Level: LevelMinor})
	if err != nil {
		t.Fatalf("second Bump: %v", err)
	}
	if second.Plan.Next.Tag() != "v0.4.0" {
		t.Errorf("second bump = %q, want v0.4.0 once v0.3.0 exists", second.Plan.Next.Tag())
	}
}

func TestBumpRefusesADirtyTree(t *testing.T) {
	repo, cfg := releaseRepo(t, "v0.2.0")
	repo.Write("wip.txt", "uncommitted\n")

	_, err := Bump(cfg, BumpOptions{Level: LevelMinor})
	if err == nil {
		t.Fatal("Bump succeeded on a dirty tree, want an error")
	}
	if !strings.Contains(err.Error(), "working tree is not clean") {
		t.Errorf("Bump = %q, want it to refuse for the dirty tree", err)
	}
	if git.TagExists("v0.3.0") {
		t.Error("a tag was created despite the refusal")
	}
}

func TestBumpRefusesDuringAnUnfinishedMerge(t *testing.T) {
	repo, cfg := releaseRepo(t, "v0.2.0")
	repo.Branch("feature/api")
	repo.Checkout("feature/api")
	repo.Write("shared.txt", "feature\n")
	repo.Commit("feat: api")

	// The conflicted merge has to happen on main, because a repository
	// mid-merge cannot check out.
	repo.Checkout("main")
	repo.Write("shared.txt", "main\n")
	repo.Commit("chore: main")
	if _, err := repo.Try("merge", "feature/api"); err == nil {
		t.Fatal("git merge succeeded on conflicting content, want a conflict")
	}

	_, err := Bump(cfg, BumpOptions{Level: LevelMinor})
	if err == nil {
		t.Fatal("Bump succeeded during an unfinished merge, want an error")
	}
	if !strings.Contains(err.Error(), "unfinished git operation") {
		t.Errorf("Bump = %q, want it to refuse for the unfinished merge", err)
	}
}

func TestBumpRejectsUnknownLevel(t *testing.T) {
	_, cfg := releaseRepo(t, "v0.2.0")

	_, err := Bump(cfg, BumpOptions{Level: "minorish"})
	if err == nil {
		t.Fatal("Bump accepted an unknown level, want an error")
	}
	if !strings.Contains(err.Error(), "unknown version level") {
		t.Errorf("Bump = %q, want it to name the problem", err)
	}
}

func TestBumpDryRunNeedsNoCleanTree(t *testing.T) {
	repo, cfg := releaseRepo(t, "v0.2.0")
	repo.Write("wip.txt", "uncommitted\n")

	if _, err := Bump(cfg, BumpOptions{Level: LevelMinor, DryRun: true}); err != nil {
		t.Fatalf("a dry run on a dirty tree should work: %v", err)
	}
}

// The tag has to point at the commit being released.
func TestBumpTagsTheCurrentCommit(t *testing.T) {
	repo, cfg := releaseRepo(t, "v0.2.0")

	if _, err := Bump(cfg, BumpOptions{Level: LevelPatch}); err != nil {
		t.Fatalf("Bump: %v", err)
	}

	tagged := repo.Output("rev-list", "-n", "1", "v0.2.1")
	head := repo.Output("rev-parse", "HEAD")
	if !strings.HasPrefix(head, tagged) {
		t.Errorf("tag v0.2.1 points at %s, want it on HEAD (%s)", tagged, head)
	}
}

// The ordering example from the SemVer 2.0.0 specification, plus the
// numeric-versus-text cases a plain string sort gets wrong.
func TestVersionPrecedence(t *testing.T) {
	ascending := []string{
		"0.9.0",
		"0.10.0",
		"1.0.0-0",
		"1.0.0-1",
		"1.0.0-2",
		"1.0.0-10",
		"1.0.0-alpha",
		"1.0.0-alpha.1",
		"1.0.0-alpha.beta",
		"1.0.0-beta",
		"1.0.0-beta.2",
		"1.0.0-beta.11",
		"1.0.0-rc.1",
		"1.0.0-rc.2",
		"1.0.0-rc.10",
		"1.0.0",
		"1.0.1-rc.1",
		"1.0.1",
		"1.1.0",
		"2.0.0-alpha.1",
		"2.0.0",
	}

	versions := make([]Version, len(ascending))
	for i, in := range ascending {
		v, err := ParseVersion(in)
		if err != nil {
			t.Fatalf("ParseVersion(%q): %v", in, err)
		}
		versions[i] = v
	}

	for i, a := range versions {
		for j, b := range versions {
			want := 0
			switch {
			case i < j:
				want = -1
			case i > j:
				want = 1
			}
			if got := a.Compare(b); got != want {
				t.Errorf("%s.Compare(%s) = %d, want %d", ascending[i], ascending[j], got, want)
			}
		}
	}
}

// Numeric identifiers are compared as numbers however long they are, so a
// huge one neither overflows nor sorts as text.
func TestVersionPrecedenceNumericIdentifiers(t *testing.T) {
	low, _ := ParseVersion("1.0.0-rc.9")
	high, _ := ParseVersion("1.0.0-rc.99999999999999999999999")
	if low.Compare(high) != -1 || high.Compare(low) != 1 {
		t.Errorf("rc.9 should be lower than rc.99999999999999999999999")
	}
}

func TestPrereleaseVersionStringAndTag(t *testing.T) {
	v := Version{Major: 1, Pre: "rc.1"}
	if got := v.String(); got != "1.0.0-rc.1" {
		t.Errorf("String() = %q, want 1.0.0-rc.1", got)
	}
	if got := v.Tag(); got != "v1.0.0-rc.1" {
		t.Errorf("Tag() = %q, want v1.0.0-rc.1", got)
	}
	if got := v.Release(); got != (Version{Major: 1}) {
		t.Errorf("Release() = %v, want 1.0.0", got)
	}
}

// A level always yields a stable version from the core, so a prerelease is
// moved past rather than silently promoted.
func TestVersionBumpFromAPrerelease(t *testing.T) {
	rc := Version{Major: 1, Pre: "rc.1"}
	tests := []struct {
		level Level
		want  string
	}{
		{LevelPatch, "1.0.1"},
		{LevelMinor, "1.1.0"},
		{LevelMajor, "2.0.0"},
	}
	for _, tc := range tests {
		if got := rc.Bump(tc.level).String(); got != tc.want {
			t.Errorf("1.0.0-rc.1 %s = %q, want %q", tc.level, got, tc.want)
		}
	}
}

func TestNextVersion(t *testing.T) {
	stable := Version{Minor: 4, Patch: 3}
	rc1 := Version{Major: 1, Pre: "rc.1"}

	tests := []struct {
		name    string
		current Version
		tagged  bool
		opts    BumpOptions
		want    string
		wantErr string
	}{
		{name: "level on stable", current: stable, tagged: true, opts: BumpOptions{Level: LevelMinor}, want: "0.5.0"},
		{name: "level on prerelease", current: rc1, tagged: true, opts: BumpOptions{Level: LevelMinor}, want: "1.1.0"},
		{name: "start with minor", current: stable, tagged: true, opts: BumpOptions{Level: LevelMinor, Pre: "rc"}, want: "0.5.0-rc.1"},
		{name: "start with patch", current: stable, tagged: true, opts: BumpOptions{Level: LevelPatch, Pre: "beta"}, want: "0.4.4-beta.1"},
		{name: "start with major", current: stable, tagged: true, opts: BumpOptions{Level: LevelMajor, Pre: "alpha"}, want: "1.0.0-alpha.1"},
		{name: "start on an untagged repo", current: Version{}, opts: BumpOptions{Level: LevelMinor, Pre: "rc"}, want: "0.1.0-rc.1"},
		{name: "start from a prerelease uses the core", current: rc1, tagged: true, opts: BumpOptions{Level: LevelMinor, Pre: "rc"}, want: "1.1.0-rc.1"},
		{name: "increment", current: rc1, tagged: true, opts: BumpOptions{Pre: "rc"}, want: "1.0.0-rc.2"},
		{name: "increment past nine", current: Version{Major: 1, Pre: "rc.9"}, tagged: true, opts: BumpOptions{Pre: "rc"}, want: "1.0.0-rc.10"},
		{name: "move to a later name", current: Version{Major: 1, Pre: "beta.2"}, tagged: true, opts: BumpOptions{Pre: "rc"}, want: "1.0.0-rc.1"},
		{name: "unnumbered prerelease", current: Version{Major: 1, Pre: "rc"}, tagged: true, opts: BumpOptions{Pre: "rc"}, want: "1.0.0-rc.1"},
		{name: "promote", current: Version{Major: 1, Pre: "rc.2"}, tagged: true, opts: BumpOptions{Promote: true}, want: "1.0.0"},

		{name: "no level", current: stable, tagged: true, opts: BumpOptions{}, wantErr: "choose a level"},
		{name: "unknown level", current: stable, tagged: true, opts: BumpOptions{Level: "minorish"}, wantErr: "unknown version level"},
		{name: "unknown level with pre", current: stable, tagged: true, opts: BumpOptions{Level: "minorish", Pre: "rc"}, wantErr: "unknown version level"},
		{name: "increment a stable version", current: stable, tagged: true, opts: BumpOptions{Pre: "rc"}, wantErr: "not a prerelease"},
		{name: "increment with no tags", current: Version{}, opts: BumpOptions{Pre: "rc"}, wantErr: "not a prerelease"},
		{name: "go back to an earlier name", current: rc1, tagged: true, opts: BumpOptions{Pre: "beta"}, wantErr: "would not be higher"},
		{name: "promote a stable version", current: stable, tagged: true, opts: BumpOptions{Promote: true}, wantErr: "not a prerelease"},
		{name: "promote with no tags", current: Version{}, opts: BumpOptions{Promote: true}, wantErr: "not a prerelease"},
		{name: "promote with a level", current: rc1, tagged: true, opts: BumpOptions{Promote: true, Level: LevelMinor}, wantErr: "cannot be combined"},
		{name: "promote with pre", current: rc1, tagged: true, opts: BumpOptions{Promote: true, Pre: "rc"}, wantErr: "cannot be combined"},
		{name: "numeric name", current: stable, tagged: true, opts: BumpOptions{Level: LevelMinor, Pre: "1"}, wantErr: "invalid prerelease name"},
		{name: "dotted name", current: stable, tagged: true, opts: BumpOptions{Level: LevelMinor, Pre: "rc.1"}, wantErr: "invalid prerelease name"},
		{name: "name with a space", current: stable, tagged: true, opts: BumpOptions{Level: LevelMinor, Pre: "r c"}, wantErr: "invalid prerelease name"},
		{name: "number too large", current: Version{Major: 1, Pre: "rc.99999999999999999999"}, tagged: true, opts: BumpOptions{Pre: "rc"}, wantErr: "too large"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := nextVersion(tc.current, tc.tagged, tc.opts)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("nextVersion = %v, want an error containing %q", got, tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error = %q, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("nextVersion: %v", err)
			}
			if got.String() != tc.want {
				t.Errorf("nextVersion = %q, want %q", got, tc.want)
			}
			if tc.tagged && got.Compare(tc.current) <= 0 {
				t.Errorf("next %s is not higher than current %s", got, tc.current)
			}
		})
	}
}

// git sorts v1.0.0-rc.1 above v1.0.0 by default, so the latest version
// must come from SemVer precedence, not from git's ordering.
func TestLatestVersionRanksAReleaseAboveItsPrereleases(t *testing.T) {
	repo := testutil.NewRepo(t)
	for _, tag := range []string{"v1.0.0-alpha.1", "v1.0.0-rc.10", "v1.0.0-rc.2", "v1.0.0", "v0.9.0"} {
		repo.Run("tag", tag)
	}

	_, tag, found, err := LatestVersion()
	if err != nil || !found {
		t.Fatalf("LatestVersion: found = %v, err = %v", found, err)
	}
	if tag != "v1.0.0" {
		t.Errorf("tag = %q, want v1.0.0", tag)
	}
}

func TestLatestVersionPicksTheNewestPrerelease(t *testing.T) {
	repo := testutil.NewRepo(t)
	for _, tag := range []string{"v0.9.0", "v1.0.0-rc.2", "v1.0.0-rc.10", "v1.0.0-beta.11"} {
		repo.Run("tag", tag)
	}

	_, tag, _, err := LatestVersion()
	if err != nil {
		t.Fatalf("LatestVersion: %v", err)
	}
	if tag != "v1.0.0-rc.10" {
		t.Errorf("tag = %q, want v1.0.0-rc.10", tag)
	}
}

func TestBumpStartsAPrerelease(t *testing.T) {
	repo, cfg := releaseRepo(t, "v0.2.0")

	result, err := Bump(cfg, BumpOptions{Level: LevelMinor, Pre: "rc"})
	if err != nil {
		t.Fatalf("Bump: %v", err)
	}
	if result.Plan.Next.Tag() != "v0.3.0-rc.1" || !git.TagExists("v0.3.0-rc.1") {
		t.Errorf("want tag v0.3.0-rc.1 created, got %q (exists: %v)", result.Plan.Next.Tag(), git.TagExists("v0.3.0-rc.1"))
	}
	if kind := repo.Output("cat-file", "-t", "v0.3.0-rc.1"); kind != "tag" {
		t.Errorf("v0.3.0-rc.1 is a %s, want an annotated tag", kind)
	}
}

// Walking a release candidate through to the release, with every earlier
// tag left exactly where it was.
func TestBumpPrereleaseLifecycle(t *testing.T) {
	repo, cfg := releaseRepo(t, "v0.9.0")

	steps := []struct {
		opts BumpOptions
		want string
	}{
		{BumpOptions{Level: LevelMajor, Pre: "beta"}, "v1.0.0-beta.1"},
		{BumpOptions{Pre: "beta"}, "v1.0.0-beta.2"},
		{BumpOptions{Pre: "rc"}, "v1.0.0-rc.1"},
		{BumpOptions{Pre: "rc"}, "v1.0.0-rc.2"},
		{BumpOptions{Promote: true}, "v1.0.0"},
		{BumpOptions{Level: LevelPatch}, "v1.0.1"},
	}

	var first string
	for _, step := range steps {
		result, err := Bump(cfg, step.opts)
		if err != nil {
			t.Fatalf("Bump(%+v): %v", step.opts, err)
		}
		if got := result.Plan.Next.Tag(); got != step.want {
			t.Fatalf("Bump(%+v) = %q, want %q", step.opts, got, step.want)
		}
		if first == "" {
			first = repo.Output("rev-parse", "refs/tags/"+step.want)
		}
	}

	if got := repo.Output("rev-parse", "refs/tags/v1.0.0-beta.1"); got != first {
		t.Errorf("v1.0.0-beta.1 moved from %s to %s", first, got)
	}
}

func TestBumpDryRunOnAPrereleaseCreatesNothing(t *testing.T) {
	_, cfg := releaseRepo(t, "v1.0.0-rc.1")

	result, err := Bump(cfg, BumpOptions{Pre: "rc", DryRun: true})
	if err != nil {
		t.Fatalf("Bump: %v", err)
	}
	if result.Tagged || git.TagExists("v1.0.0-rc.2") {
		t.Error("a dry run created the tag")
	}
	if result.Plan.Next.Tag() != "v1.0.0-rc.2" {
		t.Errorf("next tag = %q, want v1.0.0-rc.2", result.Plan.Next.Tag())
	}
}

// Prereleases get the same safeguards as any release.
func TestBumpPrereleaseKeepsTheReleaseSafeguards(t *testing.T) {
	t.Run("not on main", func(t *testing.T) {
		repo, cfg := releaseRepo(t, "v0.2.0")
		repo.Checkout("develop")

		_, err := Bump(cfg, BumpOptions{Level: LevelMinor, Pre: "rc"})
		if err == nil || !strings.Contains(err.Error(), "releases are cut on main") {
			t.Fatalf("Bump = %v, want it refused off main", err)
		}
	})

	t.Run("dirty tree", func(t *testing.T) {
		repo, cfg := releaseRepo(t, "v0.2.0")
		repo.Write("wip.txt", "uncommitted\n")

		_, err := Bump(cfg, BumpOptions{Level: LevelMinor, Pre: "rc"})
		if err == nil || !strings.Contains(err.Error(), "working tree is not clean") {
			t.Fatalf("Bump = %v, want it refused on a dirty tree", err)
		}
		if git.TagExists("v0.3.0-rc.1") {
			t.Error("a tag was created despite the refusal")
		}
	})
}

func TestBumpPromoteNeedsAPrerelease(t *testing.T) {
	_, cfg := releaseRepo(t, "v1.0.0")

	_, err := Bump(cfg, BumpOptions{Promote: true})
	if err == nil || !strings.Contains(err.Error(), "not a prerelease") {
		t.Fatalf("Bump = %v, want it to say there is nothing to promote", err)
	}
}
