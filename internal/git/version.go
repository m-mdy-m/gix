package git

import (
	"fmt"
	"strconv"
	"strings"
)

// GitVersion is a parsed `git --version`.
type GitVersion struct {
	Major, Minor, Patch int
	Raw                 string
}

func (v GitVersion) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// AtLeast reports whether v is the same as or newer than o.
func (v GitVersion) AtLeast(o GitVersion) bool {
	if v.Major != o.Major {
		return v.Major > o.Major
	}
	if v.Minor != o.Minor {
		return v.Minor > o.Minor
	}
	return v.Patch >= o.Patch
}

// MinGitVersion is the oldest Git whose `merge-tree --write-tree` gix
// relies on to preview merges without touching the repository.
var MinGitVersion = GitVersion{Major: 2, Minor: 38}

// ParseGitVersion reads the output of `git --version`, such as
// "git version 2.43.0" or "git version 2.43.0.windows.1" or
// "git version 2.39.3 (Apple Git-146)".
func ParseGitVersion(raw string) (GitVersion, error) {
	v := GitVersion{Raw: strings.TrimSpace(raw)}

	fields := strings.Fields(v.Raw)
	if len(fields) < 3 || fields[0] != "git" || fields[1] != "version" {
		return v, fmt.Errorf("unrecognised git version output %q", v.Raw)
	}

	parts := strings.Split(fields[2], ".")
	nums := make([]int, 0, 3)
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			break // "windows", "rc1", and the like end the numeric part
		}
		nums = append(nums, n)
		if len(nums) == 3 {
			break
		}
	}
	if len(nums) < 2 {
		return v, fmt.Errorf("unrecognised git version output %q", v.Raw)
	}
	v.Major, v.Minor = nums[0], nums[1]
	if len(nums) == 3 {
		v.Patch = nums[2]
	}
	return v, nil
}

// InstalledVersion asks the git on PATH for its version.
func InstalledVersion() (GitVersion, error) {
	out, err := Run("--version")
	if err != nil {
		return GitVersion{}, err
	}
	return ParseGitVersion(out)
}

// RequireMergeTree returns an error when the installed Git is known to be
// too old for merge previews. A version that cannot be read is not an
// error here: the merge-tree call itself reports failure honestly.
func RequireMergeTree() error {
	v, err := InstalledVersion()
	if err != nil {
		return nil
	}
	if !v.AtLeast(MinGitVersion) {
		return fmt.Errorf("git %s is too old: gix needs Git %s or newer to preview merges (run `gix doctor`)", v, MinGitVersion)
	}
	return nil
}
