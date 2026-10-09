package flow

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/m-mdy-m/gix/internal/config"
	"github.com/m-mdy-m/gix/internal/git"
)

// Level is which part of a version number to increase.
type Level string

const (
	LevelPatch Level = "patch"
	LevelMinor Level = "minor"
	LevelMajor Level = "major"
)

// Valid reports whether this is a level gix knows how to bump.
func (l Level) Valid() bool {
	switch l {
	case LevelPatch, LevelMinor, LevelMajor:
		return true
	default:
		return false
	}
}

// TagPrefix is the tag naming convention gix reads and writes. It matches
// what the project's release workflow already triggers on.
const TagPrefix = "v"

// Version is a SemVer 2.0.0 release number: major.minor.patch with an
// optional prerelease suffix such as rc.1.
//
// Pre is kept as the dot-joined string so a Version stays comparable with
// ==. Build metadata (+build) is not modelled: it never affects precedence
// and gix has no use for it, so a tag carrying it is not a release tag.
type Version struct {
	Major int
	Minor int
	Patch int
	Pre   string
}

func (v Version) String() string {
	core := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Pre == "" {
		return core
	}
	return core + "-" + v.Pre
}

// Tag returns the tag name for this version.
func (v Version) Tag() string { return TagPrefix + v.String() }

// Release is the stable version a prerelease leads up to: 1.0.0-rc.2 is
// the run-up to 1.0.0.
func (v Version) Release() Version {
	v.Pre = ""
	return v
}

// Bump increases the given part of the version and resets everything below
// it, which is what makes 0.9.4 minor become 0.10.0 rather than 0.9.5.
//
// The result is always a stable version, and a prerelease is bumped from
// its core: 1.0.0-rc.1 minor is 1.1.0, not an implicit promotion to 1.0.0.
func (v Version) Bump(level Level) Version {
	switch level {
	case LevelMajor:
		return Version{Major: v.Major + 1}
	case LevelMinor:
		return Version{Major: v.Major, Minor: v.Minor + 1}
	case LevelPatch:
		return Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch + 1}
	default:
		return v
	}
}

// Compare orders two versions by SemVer 2.0.0 precedence: -1, 0, or 1.
func (v Version) Compare(o Version) int {
	for _, d := range [3]int{v.Major - o.Major, v.Minor - o.Minor, v.Patch - o.Patch} {
		if d != 0 {
			return sign(d)
		}
	}
	switch {
	case v.Pre == o.Pre:
		return 0
	case v.Pre == "":
		return 1 // a release outranks every prerelease of itself
	case o.Pre == "":
		return -1
	}
	return comparePre(v.Pre, o.Pre)
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}

// comparePre compares two non-empty prerelease strings identifier by
// identifier: numeric ones numerically, the rest as ASCII, and a numeric
// identifier below an alphanumeric one. When one is a prefix of the other,
// the shorter is lower — which is why rc.2 < rc.10 and rc < rc.1.
func comparePre(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		if c := compareIdentifier(as[i], bs[i]); c != 0 {
			return c
		}
	}
	return sign(len(as) - len(bs))
}

func compareIdentifier(a, b string) int {
	aNum, bNum := isNumeric(a), isNumeric(b)
	switch {
	case aNum && bNum:
		// Identifiers are validated without leading zeros, so length
		// then text is numeric order and cannot overflow.
		if len(a) != len(b) {
			return sign(len(a) - len(b))
		}
		return sign(strings.Compare(a, b))
	case aNum:
		return -1
	case bNum:
		return 1
	}
	return sign(strings.Compare(a, b))
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// validIdentifier reports whether s is one SemVer prerelease identifier:
// ASCII alphanumerics and hyphens, with no leading zero on a number.
func validIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '-':
		default:
			return false
		}
	}
	return !(isNumeric(s) && len(s) > 1 && s[0] == '0')
}

func validPrerelease(s string) bool {
	for _, id := range strings.Split(s, ".") {
		if !validIdentifier(id) {
			return false
		}
	}
	return true
}

// ParseVersion reads a bare major.minor.patch number, optionally followed
// by a -prerelease suffix.
//
// It is strict on purpose: a tag gix can't read exactly is skipped rather
// than guessed at, so a tag like "v1.2" or "release-3" can't be mistaken
// for a release and bumped from the wrong base.
func ParseVersion(s string) (Version, error) {
	core, pre, hasPre := strings.Cut(s, "-")

	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("not a major.minor.patch version: %q", s)
	}

	var out [3]int
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return Version{}, fmt.Errorf("not a major.minor.patch version: %q", s)
		}
		out[i] = n
	}

	v := Version{Major: out[0], Minor: out[1], Patch: out[2]}
	if hasPre {
		if !validPrerelease(pre) {
			return Version{}, fmt.Errorf("not a valid prerelease suffix: %q", s)
		}
		v.Pre = pre
	}
	return v, nil
}

// LatestVersion returns the highest release tag in the repository, along
// with the tag name it came from. Nothing tagged yet is not an error: it
// just means this is the first release.
//
// "Highest" is SemVer precedence, worked out here rather than taken from
// git's tag ordering: by default git sorts v1.0.0-rc.1 above v1.0.0.
// A prerelease can be the highest, when it is newer than every release.
func LatestVersion() (Version, string, bool, error) {
	tags, err := git.ListTags()
	if err != nil {
		return Version{}, "", false, err
	}

	var (
		best    Version
		bestTag string
		found   bool
	)
	for _, tag := range tags {
		if !strings.HasPrefix(tag, TagPrefix) {
			continue
		}
		v, err := ParseVersion(strings.TrimPrefix(tag, TagPrefix))
		if err != nil {
			continue
		}
		if !found || v.Compare(best) > 0 {
			best, bestTag, found = v, tag, true
		}
	}
	return best, bestTag, found, nil
}

// BumpPlan is what a release would do, computed without changing
// anything. It is what --dry-run prints.
type BumpPlan struct {
	Level Level

	// Current is the highest existing release, and CurrentTag its name.
	// CurrentTag is "" and Current is 0.0.0 when nothing is tagged.
	Current    Version
	CurrentTag string
	Tagged     bool

	// Next is the version this bump would create.
	Next Version

	// Branch and Commit say where the tag would point.
	Branch string
	Commit string

	// CommitsSince is how many commits have landed since CurrentTag, or
	// the total when there is no tag yet.
	CommitsSince int
}

// BumpResult is a completed bump.
type BumpResult struct {
	Plan   *BumpPlan
	Tagged bool
}

// BumpOptions configures a release bump.
//
// Level alone makes the next stable version. Pre starts a prerelease
// (with a Level) or moves the current one along (without). Promote turns
// the current prerelease into its stable release. Pre and Promote are
// separate on purpose, so that incrementing and promoting are never
// confused, and neither combines with the other.
type BumpOptions struct {
	Level   Level
	Pre     string
	Promote bool
	DryRun  bool
}

// Bump plans and, unless DryRun is set, creates the next release tag.
//
// What bump deliberately does not do, because the repository has no
// convention for any of it: write a CHANGELOG entry, make a release
// commit, sign the tag, or push. It calculates a version and creates a
// tag. Anything else is your call and your commit.
//
// The tag is created on the main branch, which is where gix already puts
// release tags.
func Bump(cfg *config.Config, opts BumpOptions) (*BumpResult, error) {
	branch, err := git.CurrentBranch()
	if err != nil {
		return nil, fmt.Errorf("cannot release from a detached HEAD — check out the branch you are releasing")
	}

	// gix already creates tags on the main branch (see Finish), so allowing
	// a release from anywhere else would tag a commit the rest of gix
	// would never have tagged.
	mainName := cfg.MainName()
	if branch != mainName {
		return nil, fmt.Errorf("cannot release from %s: releases are cut on %s — check it out first",
			branch, mainName)
	}

	plan, err := planBump(opts)
	if err != nil {
		return nil, err
	}
	plan.Branch = branch

	commit, err := git.Run("rev-parse", "--short", "HEAD")
	if err != nil {
		return nil, err
	}
	plan.Commit = commit

	// Defensive: the plan comes from the highest existing tag, so its
	// result should not exist yet. This catches a tag created between
	// planning and tagging, and keeps the error legible if git's ordering
	// ever differs from the assumption.
	if git.TagExists(plan.Next.Tag()) {
		return nil, fmt.Errorf("tag %s already exists — nothing to do", plan.Next.Tag())
	}

	if opts.DryRun {
		return &BumpResult{Plan: plan}, nil
	}

	if err := preflightMutation(); err != nil {
		return nil, err
	}

	message := fmt.Sprintf("%s %s", plan.Next.Tag(), plan.Next.String())
	if err := git.Tag(plan.Next.Tag(), message); err != nil {
		return nil, fmt.Errorf("creating tag %s: %w", plan.Next.Tag(), err)
	}

	return &BumpResult{Plan: plan, Tagged: true}, nil
}

// PlanBump works out what the next stable version would be, touching
// nothing.
func PlanBump(level Level) (*BumpPlan, error) {
	return planBump(BumpOptions{Level: level})
}

func planBump(opts BumpOptions) (*BumpPlan, error) {
	current, currentTag, tagged, err := LatestVersion()
	if err != nil {
		return nil, err
	}

	next, err := nextVersion(current, tagged, opts)
	if err != nil {
		return nil, err
	}

	plan := &BumpPlan{
		Level:      opts.Level,
		Current:    current,
		CurrentTag: currentTag,
		Tagged:     tagged,
		Next:       next,
	}

	commits, err := commitsSince(currentTag)
	if err != nil {
		return nil, err
	}
	plan.CommitsSince = commits

	return plan, nil
}

// nextVersion applies the bump rules to the highest existing version.
//
//	Level            stable version from the core: 1.0.0-rc.1 minor → 1.1.0
//	Level + Pre      start a prerelease:           0.4.3 minor rc → 0.5.0-rc.1
//	Pre              next in the series:           1.0.0-rc.1 rc → 1.0.0-rc.2
//	Promote          drop the suffix:              1.0.0-rc.2 → 1.0.0
func nextVersion(current Version, tagged bool, opts BumpOptions) (Version, error) {
	if opts.Promote {
		if opts.Level != "" || opts.Pre != "" {
			return Version{}, fmt.Errorf("--release promotes the current prerelease as it is, and cannot be combined with a level or --pre")
		}
		if !tagged || current.Pre == "" {
			return Version{}, fmt.Errorf("nothing to promote: %s is not a prerelease", describeCurrent(current, tagged))
		}
		return current.Release(), nil
	}

	if opts.Pre == "" {
		if !opts.Level.Valid() {
			if opts.Level == "" {
				return Version{}, fmt.Errorf("choose a level (%s, %s, or %s), or use --pre or --release", LevelPatch, LevelMinor, LevelMajor)
			}
			return Version{}, fmt.Errorf("unknown version level %q (want %s, %s, or %s)",
				opts.Level, LevelPatch, LevelMinor, LevelMajor)
		}
		return current.Bump(opts.Level), nil
	}

	if !validIdentifier(opts.Pre) || isNumeric(opts.Pre) {
		return Version{}, fmt.Errorf("invalid prerelease name %q: use letters, digits, and hyphens, not only digits (for example alpha, beta, or rc)", opts.Pre)
	}

	if opts.Level != "" {
		if !opts.Level.Valid() {
			return Version{}, fmt.Errorf("unknown version level %q (want %s, %s, or %s)",
				opts.Level, LevelPatch, LevelMinor, LevelMajor)
		}
		next := current.Bump(opts.Level)
		next.Pre = opts.Pre + ".1"
		return next, nil
	}

	if !tagged || current.Pre == "" {
		return Version{}, fmt.Errorf("%s is not a prerelease: start one with a level, for example `gix version bump minor --pre %s`",
			describeCurrent(current, tagged), opts.Pre)
	}
	pre, err := continuePrerelease(current.Pre, opts.Pre)
	if err != nil {
		return Version{}, err
	}
	next := current
	next.Pre = pre
	if next.Compare(current) <= 0 {
		return Version{}, fmt.Errorf("%s would not be higher than %s — a prerelease can only move forward (alpha, beta, rc)", next, current)
	}
	return next, nil
}

func describeCurrent(current Version, tagged bool) string {
	if !tagged {
		return "the repository (no release tags yet)"
	}
	return current.String()
}

// continuePrerelease numbers the next prerelease for the given name. In
// the same series the trailing number goes up (rc.1 → rc.2); with a
// different name the series starts again at .1, and the caller checks
// that this really is a step forward.
func continuePrerelease(current, name string) (string, error) {
	ids := strings.Split(current, ".")
	series, last := ids[:len(ids)-1], ids[len(ids)-1]

	// "rc" with no number yet is its own series, so it also lands on rc.1.
	if !isNumeric(last) || strings.Join(series, ".") != name {
		return name + ".1", nil
	}
	n, err := strconv.Atoi(last)
	if err != nil {
		return "", fmt.Errorf("prerelease number %q is too large to increment", last)
	}
	return fmt.Sprintf("%s.%d", name, n+1), nil
}

// commitsSince counts the commits since a tag, or the whole history when
// there is no tag yet.
// A repository whose HEAD has no commit yet has no history to count, so
// that is reported as zero rather than passed on as git's "unknown
// revision" error.
func commitsSince(tag string) (int, error) {
	rev := "HEAD"
	if tag != "" {
		rev = tag + "..HEAD"
	}
	out, err := git.Run("rev-list", "--count", rev)
	if err != nil {
		if !git.HasCommits() {
			return 0, nil
		}
		return 0, fmt.Errorf("counting commits: %w", err)
	}
	n, err := strconv.Atoi(out)
	if err != nil {
		return 0, fmt.Errorf("unexpected commit count %q: %w", out, err)
	}
	return n, nil
}
