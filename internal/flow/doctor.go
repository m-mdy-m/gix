package flow

import (
	"fmt"
	"strings"

	"github.com/m-mdy-m/gix/internal/config"
	"github.com/m-mdy-m/gix/internal/git"
)

// Severity is how much one doctor check matters.
type Severity int

const (
	// OK means the check passed; the value is just context.
	OK Severity = iota
	// Warn means something is off but gix can still work.
	Warn
	// Fail means gix cannot work correctly until it's dealt with.
	Fail
)

// Check is one line of doctor output: what was inspected, what was found,
// and what to do about it when it isn't fine.
type Check struct {
	Name     string
	Severity Severity
	Value    string
	Problem  string
	Fix      string
}

// Report is the full result of inspecting a repository, in a fixed order
// so repeated runs print the same thing.
type Report struct {
	Checks []Check
}

// OK reports whether nothing came back at Warn or above.
func (r Report) OK() bool { return r.Count(Warn) == 0 && r.Count(Fail) == 0 }

// Count returns how many checks have exactly this severity.
func (r Report) Count(s Severity) int {
	n := 0
	for _, c := range r.Checks {
		if c.Severity == s {
			n++
		}
	}
	return n
}

// Problems returns the checks worth acting on, in report order.
func (r Report) Problems() []Check {
	var out []Check
	for _, c := range r.Checks {
		if c.Severity != OK {
			out = append(out, c)
		}
	}
	return out
}

// CheckRepo inspects a repository the way gix depends on it and reports
// what it finds. It deliberately runs without .gix/config: "gix isn't set
// up here" is one of the things a user needs to find out.
//
// Only safe, reversible fixes are ever suggested. Nothing in here
// rewrites history, discards work, or writes config.
func CheckRepo(repoRoot string) Report {
	var r Report

	r.Checks = append(r.Checks, checkGitVersion())
	r.Checks = append(r.Checks, checkRepository(repoRoot))
	r.Checks = append(r.Checks, checkConfig(repoRoot))
	r.Checks = append(r.Checks, checkHead())
	r.Checks = append(r.Checks, checkWorkingTree())
	r.Checks = append(r.Checks, checkInProgress())
	r.Checks = append(r.Checks, checkIdentity())
	r.Checks = append(r.Checks, checkRemote(repoRoot))
	r.Checks = append(r.Checks, checkUpstream())
	r.Checks = append(r.Checks, checkSigning())
	r.Checks = append(r.Checks, checkFlowHealth(repoRoot))

	return r
}

// checkGitVersion reports the installed Git, and warns when it is too old
// for `gix branch sync` to preview merges.
func checkGitVersion() Check {
	c := Check{Name: "git"}

	v, err := git.InstalledVersion()
	if err != nil {
		c.Severity = Warn
		c.Problem = "could not read the Git version: " + err.Error()
		c.Fix = "make sure `git --version` works in this shell"
		return c
	}

	c.Value = v.String()
	if !v.AtLeast(git.MinGitVersion) {
		c.Severity = Warn
		c.Problem = fmt.Sprintf("Git %s is older than %s, which `gix branch sync` needs to preview merges", v, git.MinGitVersion)
		c.Fix = "upgrade Git to " + git.MinGitVersion.String() + " or newer"
	}
	return c
}

func checkRepository(repoRoot string) Check {
	c := Check{Name: "repository"}

	if !git.IsRepo() {
		c.Severity = Fail
		c.Problem = "this directory is not inside a Git repository"
		c.Fix = "run `gix init` to create one here"
		return c
	}

	c.Value = repoRoot
	return c
}

func checkConfig(repoRoot string) Check {
	c := Check{Name: "gix config"}

	_, err := config.Load(repoRoot)
	switch {
	case err != nil && !config.Exists(repoRoot):
		c.Severity = Warn
		c.Problem = "gix is not initialized in this repository"
		c.Fix = "run `gix flow init`"
		return c
	case err != nil:
		c.Severity = Fail
		c.Problem = err.Error()
		c.Fix = "fix or delete " + config.Path(repoRoot)
		return c
	}

	c.Value = config.Path(repoRoot)
	return c
}

func checkHead() Check {
	c := Check{Name: "HEAD"}

	head, err := git.GetHeadState()
	if err != nil {
		c.Severity = Fail
		c.Problem = err.Error()
		c.Fix = "run `git status` to see what state the repository is in"
		return c
	}

	switch head.Kind {
	case git.HeadDetached:
		c.Severity = Warn
		c.Value = head.Commit
		c.Problem = "HEAD is detached, so there is no branch to sync or merge into"
		c.Fix = "run `git checkout <branch>` to get back on a branch"
	case git.HeadUnborn:
		c.Severity = Warn
		c.Value = head.Branch
		c.Problem = "no commits yet, so there is no base branch to build the flow on"
		c.Fix = "make an initial commit, then run `gix flow init`"
	default:
		c.Value = head.Branch
	}

	return c
}

func checkWorkingTree() Check {
	c := Check{Name: "working tree"}

	clean, err := git.IsClean()
	if err != nil {
		c.Severity = Fail
		c.Problem = err.Error()
		return c
	}

	if clean {
		c.Value = "clean"
		return c
	}

	c.Severity = Warn
	c.Value = "uncommitted changes"
	c.Problem = "gix refuses to move branches or merge while changes are uncommitted"
	c.Fix = "commit or stash them first"
	return c
}

func checkInProgress() Check {
	c := Check{Name: "in-progress operation"}

	op, found, err := git.InProgressOperation()
	if err != nil {
		c.Severity = Warn
		c.Problem = err.Error()
		return c
	}
	if !found {
		c.Value = "none"
		return c
	}

	c.Severity = Warn
	c.Value = string(op)
	c.Problem = fmt.Sprintf("a %s is unfinished, so gix will refuse to run", op)
	c.Fix = fmt.Sprintf("finish it with `git %s --continue`, or drop it with `git %s --abort`", op, op)
	return c
}

func checkIdentity() Check {
	c := Check{Name: "identity"}

	cfg, err := git.ReadConfig()
	if err != nil {
		c.Severity = Warn
		c.Problem = err.Error()
		return c
	}

	name, hasName := cfg.Get("user.name")
	email, hasEmail := cfg.Get("user.email")

	switch {
	case !hasName && !hasEmail:
		c.Severity = Fail
		c.Problem = "user.name and user.email are not set, so commits cannot be attributed"
		c.Fix = "set them with `gix doctor --fix --set-user-name <name> --set-user-email <email>`"
	case !hasEmail:
		c.Severity = Fail
		c.Value = name.Value
		c.Problem = "user.email is not set"
		c.Fix = fmt.Sprintf("set it with `gix doctor --fix --set-user-email <email> --set-user-name %q`", name.Value)
	case !hasName:
		c.Severity = Fail
		c.Value = email.Value
		c.Problem = "user.name is not set"
		c.Fix = fmt.Sprintf("set it with `gix doctor --fix --set-user-name <name> --set-user-email %q`", email.Value)
	default:
		c.Value = fmt.Sprintf("%s <%s>", name.Value, email.Value)
		if name.Scope != email.Scope {
			c.Severity = Warn
			c.Problem = fmt.Sprintf("name is set in %s config but email in %s config", name.Scope, email.Scope)
			c.Fix = fmt.Sprintf("set both in one scope: `gix doctor --fix --scope %s --set-user-name ... --set-user-email ...`", email.Scope)
			return c
		}
		c.Value = fmt.Sprintf("%s <%s> (%s)", name.Value, email.Value, name.Scope)
	}

	return c
}

func checkRemote(repoRoot string) Check {
	c := Check{Name: "remote"}

	name := "origin"
	if cfg, err := config.Load(repoRoot); err == nil && cfg.Remote != "" {
		name = cfg.Remote
	}

	if git.HasRemote(name) {
		c.Value = name
		return c
	}

	remotes, err := git.Remotes()
	if err != nil {
		c.Severity = Warn
		c.Problem = err.Error()
		return c
	}
	if len(remotes) > 0 {
		c.Value = strings.Join(remotes, ", ")
		c.Severity = Warn
		c.Problem = fmt.Sprintf("configured remote %q does not exist", name)
		c.Fix = fmt.Sprintf("run `git remote rename %s %s`, or set `remote: %s` in .gix/config", remotes[0], name, remotes[0])
		return c
	}

	c.Severity = Warn
	c.Problem = "no remotes configured, so branches can't be pushed or pulled"
	c.Fix = fmt.Sprintf("run `git remote add %s <url>`", name)
	return c
}

func checkUpstream() Check {
	c := Check{Name: "upstream"}

	branch, err := git.CurrentBranch()
	if err != nil {
		// Detached HEAD or an unborn branch: there is no upstream to have.
		c.Value = "n/a"
		return c
	}

	upstream, err := git.UpstreamOf(branch)
	if err != nil {
		c.Severity = Warn
		c.Problem = err.Error()
		return c
	}
	if upstream == "" {
		c.Severity = Warn
		c.Value = branch
		c.Problem = fmt.Sprintf("%s does not track a remote branch", branch)
		c.Fix = fmt.Sprintf("run `git push -u <remote> %s` to start tracking one", branch)
		return c
	}

	c.Value = fmt.Sprintf("%s -> %s", branch, upstream)
	return c
}

func checkSigning() Check {
	c := Check{Name: "signing"}

	cfg, err := git.ReadConfig()
	if err != nil {
		c.Severity = Warn
		c.Problem = err.Error()
		return c
	}

	signing := InspectSigning(cfg)

	switch {
	case !signing.SigningEnabled() && signing.Key == "":
		c.Value = "off"
		return c
	case signing.Available:
		c.Value = signingSummary(signing)
		return c
	}

	c.Severity = Warn
	c.Value = signingSummary(signing)
	if signing.Program == "" {
		c.Problem = fmt.Sprintf("%s signing is configured but git cannot use it", signing.Format)
		c.Fix = "check `gpg.format` — it must be openpgp, ssh, or x509"
		return c
	}
	c.Problem = fmt.Sprintf("%s signing is configured but %s was not found on PATH", signing.Format, signing.Program)
	c.Fix = fmt.Sprintf("install %s, or unset the key with `git config --unset user.signingkey`", signing.Program)
	return c
}

func signingSummary(s SigningInfo) string {
	var parts []string
	parts = append(parts, s.Format)
	if s.Key != "" {
		parts = append(parts, "key "+s.Key)
	}
	if s.CommitSign {
		parts = append(parts, "commits signed")
	} else {
		parts = append(parts, "commits unsigned")
	}
	if s.TagSign {
		parts = append(parts, "tags signed")
	}
	return strings.Join(parts, ", ")
}

func checkFlowHealth(repoRoot string) Check {
	c := Check{Name: "flow"}

	cfg, err := config.Load(repoRoot)
	if err != nil {
		c.Severity = Warn
		c.Problem = err.Error()
		return c
	}

	health := CheckHealth(cfg)
	if health.OK() {
		c.Value = strings.Join(cfg.BaseKeys(), ", ")
		return c
	}

	var missing []string
	for _, mb := range health.MissingBases {
		missing = append(missing, mb.Name)
	}
	c.Severity = Fail
	c.Value = strings.Join(missing, ", ")
	c.Problem = "configured base branch(es) do not exist in this repository"
	c.Fix = "if they were renamed, update .gix/config (`gix config list` shows the names gix expects)"
	return c
}
