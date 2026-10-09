package flow

import (
	"strings"
	"time"

	"github.com/m-mdy-m/gix/internal/config"
	"github.com/m-mdy-m/gix/internal/git"
)

// BranchState is how one local branch sits relative to its upstream and
// to the flow. Derived entirely from git, never guessed.
type BranchState string

const (
	// StateTracked is a branch that tracks a remote branch and matches it.
	StateTracked BranchState = "tracked"
	// StateAhead is ahead of its upstream, so a push would be needed.
	StateAhead BranchState = "ahead"
	// StateBehind is behind its upstream, so its remote moved on.
	StateBehind BranchState = "behind"
	// StateDiverged is ahead and behind at once — a push would be rejected.
	StateDiverged BranchState = "diverged"
	// StateUntracked has no upstream configured.
	StateUntracked BranchState = "untracked"
	// StateGone had an upstream, but that ref no longer exists — usually
	// because the branch was deleted on the remote.
	StateGone BranchState = "gone"
)

// Attention reports whether this state needs the user to do something.
func (s BranchState) Attention() bool {
	switch s {
	case StateTracked:
		return false
	default:
		return true
	}
}

// BranchHealth is one local branch, placed in the flow and compared to its
// upstream.
type BranchHealth struct {
	Name     string
	Kind     string // config kind the branch belongs to, "" when none matches
	Role     config.Role
	Parent   string // configured parent, for topic kinds
	State    BranchState
	Upstream string
	Ahead    int
	Behind   int

	// AutoSync is the branch's own `autosync: true` setting, or that of
	// its kind for topic branches. It marks the branch as one that should
	// receive updates from its parent.
	AutoSync bool

	// MergedInto names the configured merge targets that already contain
	// this branch's commits — empty for a base branch.
	MergedInto []string

	// AgeDays is how long ago this branch's tip commit was, or -1 when the
	// branch has no commit yet.
	AgeDays int
}

// NeedsWork reports whether the branch is behind, ahead, untracked, or
// has lost its upstream.
func (h BranchHealth) NeedsWork() bool { return h.State.Attention() }

// AlreadyMerged reports whether every merge target already contains this
// branch, which is what makes it safe to delete.
func (h BranchHealth) AlreadyMerged() bool {
	return len(h.MergedInto) > 0
}

// CheckBranches inspects every local branch: where it sits in the
// configured flow, how it compares to its upstream, how long since it
// last moved, and which merge targets already contain it. Branches come
// back in git's refname order, so output is stable across runs and
// machines.
func CheckBranches(cfg *config.Config) ([]BranchHealth, error) {
	refs, err := git.ListBranchTracking()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	ages := commitAges(now)

	out := make([]BranchHealth, 0, len(refs))
	for _, ref := range refs {
		h := branchHealth(cfg, ref)
		h.AgeDays = ages[ref.Name]
		out = append(out, h)
	}
	return out, nil
}

// commitAges maps branch name to days since its last commit. A branch
// with no commit gets -1, which is distinct from "committed today".
func commitAges(now time.Time) map[string]int {
	commits, err := git.ListBranchCommits()
	if err != nil {
		log.Debugf("commit dates unavailable: %v", err)
		return nil
	}

	ages := make(map[string]int, len(commits))
	for _, c := range commits {
		if !c.Valid {
			ages[c.Name] = -1
			continue
		}
		ages[c.Name] = c.DaysSince(now)
	}
	return ages
}

func branchHealth(cfg *config.Config, ref git.BranchTracking) BranchHealth {
	h := BranchHealth{
		Name:     ref.Name,
		Upstream: ref.Upstream,
		Ahead:    ref.Track.Ahead(),
		Behind:   ref.Track.Behind(),
	}

	switch {
	case ref.Track.Gone():
		h.State = StateGone
	case ref.Upstream == "":
		h.State = StateUntracked
	case h.Ahead > 0 && h.Behind > 0:
		h.State = StateDiverged
	case h.Ahead > 0:
		h.State = StateAhead
	case h.Behind > 0:
		h.State = StateBehind
	default:
		h.State = StateTracked
	}

	kind, spec, ok := findKind(cfg, ref.Name)
	switch {
	case ok:
		h.Kind = kind
		h.Role = spec.Role
		h.Parent = resolveBranchName(cfg, firstTarget(cfg, spec))
		h.AutoSync = spec.AutoSync
	case isBase(cfg, ref.Name):
		h.Role = config.RoleBase
		// A base branch can also ask for updates: that is how develop
		// follows main in the default config.
		if base, found := cfg.Spec(ref.Name); found {
			h.AutoSync = base.AutoSync
			if base.Parent != "" {
				h.Parent = resolveBranchName(cfg, base.Parent)
			}
		}
	}

	if h.Role != config.RoleTopic {
		return h
	}
	h.MergedInto = mergedTargets(cfg, spec, h.Name)
	return h
}

// findKind returns the topic kind a branch name belongs to. Longest
// prefix wins, so a kind configured as "feature/x/" isn't shadowed by a
// broader "feature/".
func findKind(cfg *config.Config, branch string) (string, config.BranchSpec, bool) {
	best := ""
	var bestSpec config.BranchSpec

	for _, kind := range cfg.Kinds() {
		spec, ok := cfg.Spec(kind)
		if !ok || spec.Prefix == "" || !strings.HasPrefix(branch, spec.Prefix) || branch == spec.Prefix {
			continue
		}
		if len(spec.Prefix) > len(best) {
			best, bestSpec = kind, spec
		}
	}
	if best == "" {
		return "", config.BranchSpec{}, false
	}
	return best, bestSpec, true
}

func isBase(cfg *config.Config, branch string) bool {
	_, ok := IsConfiguredBase(cfg, branch)
	return ok
}

func firstTarget(cfg *config.Config, spec config.BranchSpec) string {
	targets := spec.MergeTargets()
	if len(targets) == 0 {
		return ""
	}
	return targets[0]
}

// mergedTargets lists the merge targets that already contain the branch.
// A missing target is skipped rather than reported: not being able to
// tell is different from knowing it isn't merged.
func mergedTargets(cfg *config.Config, spec config.BranchSpec, branch string) []string {
	var out []string
	for _, key := range spec.MergeTargets() {
		name := resolveBranchName(cfg, key)
		if !git.BranchExists(name) {
			continue
		}
		if git.IsAncestor(branch, name) {
			out = append(out, name)
		}
	}
	return out
}

// PruneEntry is one branch that would be safe to delete, and why.
type PruneEntry struct {
	Name   string
	Kind   string
	Reason string

	// Merged is true only when the branch's configured merge targets
	// already contain its work. A branch whose upstream merely vanished is
	// not necessarily merged, so it can be listed but never deleted.
	Merged bool
}

// PrunePlan is what `gix branch prune` would do. It is a preview only:
// nothing here is deleted, and gix never decides to delete on its own.
type PrunePlan struct {
	Entries []PruneEntry
	// Protected names the branches deliberately left out, so the reason a
	// merged base branch is still there is visible rather than implied.
	Protected []string
}

// Empty reports whether there is nothing to prune.
func (p PrunePlan) Empty() bool { return len(p.Entries) == 0 }

// PlanPrune lists branches whose work is already merged away, or whose
// upstream is gone, so `gix branch prune` can show what cleanup would
// mean.
//
// Base branches and the branch you are standing on are never candidates,
// whatever their state: they are the flow, not leftovers.
func PlanPrune(cfg *config.Config, health []BranchHealth, current string) PrunePlan {
	var plan PrunePlan

	for _, h := range health {
		if h.Role != config.RoleTopic {
			plan.Protected = append(plan.Protected, h.Name)
			continue
		}
		if h.Name == current {
			plan.Protected = append(plan.Protected, h.Name)
			continue
		}

		switch {
		case h.AlreadyMerged():
			plan.Entries = append(plan.Entries, PruneEntry{
				Name:   h.Name,
				Kind:   h.Kind,
				Reason: "already merged into " + strings.Join(h.MergedInto, " and "),
				Merged: true,
			})
		case h.State == StateGone:
			plan.Entries = append(plan.Entries, PruneEntry{
				Name:   h.Name,
				Kind:   h.Kind,
				Reason: "upstream " + h.Upstream + " no longer exists",
				Merged: false,
			})
		}
	}

	return plan
}

// PruneOutcome is what a prune run did to one branch.
type PruneOutcome struct {
	Name string
	// Deleted is true only when the branch was really removed.
	Deleted bool
	// Reason explains why it was listed; Detail explains what happened.
	Reason string
	Detail string
}

// PruneResult is a whole prune run.
type PruneResult struct {
	Outcomes []PruneOutcome
	// Kept names the listed branches that were not deleted, so the report
	// is complete rather than silent about what was left behind.
	Kept []string
}

// ApplyPrune deletes the branches a plan lists as fully merged, and
// nothing else.
//
// The deletion itself is `git branch -d`, never `-D`: git refuses to
// delete a branch whose commits are not reachable from its upstream, and
// that refusal is exactly the safety net this needs. A branch whose
// upstream merely disappeared is not necessarily merged, so it is listed
// and left alone for you to judge.
func ApplyPrune(plan PrunePlan) (*PruneResult, error) {
	if err := preflightMutation(); err != nil {
		return nil, err
	}

	result := &PruneResult{}
	for _, entry := range plan.Entries {
		outcome := PruneOutcome{Name: entry.Name, Reason: entry.Reason}

		if !entry.Merged {
			outcome.Detail = "not deleted: its upstream is gone, which does not mean its work is merged — remove it yourself if you are sure"
			result.Outcomes = append(result.Outcomes, outcome)
			result.Kept = append(result.Kept, entry.Name)
			continue
		}

		if err := git.DeleteBranch(entry.Name, false); err != nil {
			outcome.Detail = err.Error()
			result.Kept = append(result.Kept, entry.Name)
		} else {
			outcome.Deleted = true
		}
		result.Outcomes = append(result.Outcomes, outcome)
	}

	return result, nil
}
