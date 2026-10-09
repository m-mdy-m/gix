package flow

import (
	"errors"
	"fmt"
	"strings"

	"github.com/m-mdy-m/gix/internal/config"
	"github.com/m-mdy-m/gix/internal/git"
)

// SyncState is what happened, or would happen, to one branch.
type SyncState string

const (
	// SyncUpToDate means the branch already contains everything from its
	// source, so there is nothing to do.
	SyncUpToDate SyncState = "up to date"
	// SyncWouldMerge means the source can be merged in cleanly.
	SyncWouldMerge SyncState = "would merge"
	// SyncMerged means the merge happened.
	SyncMerged SyncState = "merged"
	// SyncConflict means the merge would conflict. Nothing is attempted.
	SyncConflict SyncState = "conflict"
	// SyncFailed means the merge was attempted and did not complete.
	SyncFailed SyncState = "failed"
	// SyncSkipped means this branch was not eligible.
	SyncSkipped SyncState = "skipped"
)

// SyncEntry is one branch's place in a sync run.
type SyncEntry struct {
	Branch string
	Source string
	State  SyncState
	Detail string
}

// OK reports whether the branch ended up or would end up in sync.
func (e SyncEntry) OK() bool {
	switch e.State {
	case SyncUpToDate, SyncMerged, SyncWouldMerge:
		return true
	default:
		return false
	}
}

// SyncResult is a whole sync run.
type SyncResult struct {
	Entries []SyncEntry
	DryRun  bool
	// Conflicted counts the branches that could not be merged, so the
	// caller can fail the command without inspecting every entry.
	Conflicted int
}

// OK reports whether every eligible branch is in sync.
func (r SyncResult) OK() bool { return r.Conflicted == 0 }

// SyncOptions configures a sync run.
type SyncOptions struct {
	// Source is the branch to bring in. Empty means the first base branch.
	Source string
	// DryRun reports what would happen and changes nothing.
	DryRun bool
	// AutoOnly restricts the run to branches marked `autosync: true` in
	// .gix/config, instead of every eligible branch.
	AutoOnly bool
}

// Sync brings a source branch into every eligible topic branch.
//
// The strategy is each kind's configured downstream_strategy, defaulting
// to merge — the same setting `gix config list` already shows, and the
// one that never rewrites history.
//
// Safety rules, in order of importance:
//   - nothing runs unless the working tree is clean and no Git operation
//     is in progress
//   - a branch that already contains the source is skipped, not re-merged
//   - a conflict is detected before any merge is attempted where that is
//     possible, and otherwise the failed merge is aborted so the next
//     branch is not blocked by leftover conflict state
//   - one branch failing does not stop the others, and the caller is told
//     so it can exit non-zero
//   - the source branch itself and base branches are never touched
func Sync(cfg *config.Config, opts SyncOptions) (*SyncResult, error) {
	// A dry run changes nothing, so it must not need the preconditions
	// that only matter for changing things. Refusing a preview because of a
	// dirty tree would hide exactly the situation worth previewing.
	if !opts.DryRun {
		if err := preflightMutation(); err != nil {
			return nil, err
		}
	}

	source, err := resolveSource(cfg, opts.Source)
	if err != nil {
		return nil, err
	}
	if !git.BranchExists(source) {
		return nil, fmt.Errorf("source branch %q does not exist — create it or pass --from with a branch that does", source)
	}

	result := &SyncResult{DryRun: opts.DryRun}

	branches, err := syncCandidates(cfg, source, opts.AutoOnly)
	if err != nil {
		return nil, err
	}
	if len(branches) == 0 {
		return result, nil
	}

	// Sync checks branches out one at a time, so put the user back where
	// they were rather than leaving them on whichever branch ran last.
	if !opts.DryRun {
		if origin, err := git.CurrentBranch(); err == nil {
			defer func() {
				if err := git.Checkout(origin); err != nil {
					log.Debugf("could not return to %s: %v", origin, err)
				}
			}()
		}
	}

	for _, target := range branches {
		entry := syncOne(cfg, source, target, opts)
		if entry.State == SyncConflict || entry.State == SyncFailed {
			result.Conflicted++
		}
		result.Entries = append(result.Entries, entry)
	}

	return result, nil
}

// syncOne brings one branch up to date, containing any failure so the
// next branch still gets its turn.
func syncOne(cfg *config.Config, source, target string, opts SyncOptions) SyncEntry {
	entry := SyncEntry{Branch: target, Source: source}

	if git.IsAncestor(source, target) {
		entry.State = SyncUpToDate
		return entry
	}

	// Only a merge is safe here. Rebasing the target onto the source
	// rewrites the target's history, and squash would discard the commits
	// being brought in, so those strategies are refused rather than
	// quietly doing something different from what the config says.
	if strategy := strategyFor(cfg, target); strategy != config.StrategyMerge {
		entry.State = SyncSkipped
		entry.Detail = fmt.Sprintf("downstream_strategy is %s, and sync only merges — merge it by hand or change the strategy", strategy)
		return entry
	}

	if _, err := git.Run("merge-base", source, target); err != nil {
		entry.State = SyncFailed
		entry.Detail = fmt.Sprintf("no common ancestor between %s and %s — they have unrelated histories", source, target)
		return entry
	}

	if opts.DryRun {
		entry.State = SyncWouldMerge
		if conflict, reason := wouldConflict(source, target); conflict {
			entry.State = SyncConflict
			entry.Detail = reason
		}
		return entry
	}

	if conflict, reason := wouldConflict(source, target); conflict {
		entry.State = SyncConflict
		entry.Detail = reason
		return entry
	}

	// Checkout from a known-good state: sync must not be affected by
	// whatever branch the user happened to be standing on.
	if err := git.Checkout(target); err != nil {
		entry.State = SyncFailed
		entry.Detail = fmt.Sprintf("cannot switch to %s: %v", target, err)
		return entry
	}

	if err := git.MergeNoFF(source, fmt.Sprintf("Merge branch '%s' into %s", source, target)); err != nil {
		abortMerge()
		entry.State = SyncFailed
		entry.Detail = err.Error()
		return entry
	}

	entry.State = SyncMerged
	return entry
}

// wouldConflict asks git whether merging source into target would
// conflict, without touching the repository.
//
// git merge-tree computes a real merge in memory, so this is the honest
// check: no temporary branch, no checkout, nothing to clean up.
func wouldConflict(source, target string) (bool, string) {
	files, err := git.MergeTreeConflicts(source, target)
	if err != nil {
		// merge-tree itself failed, so the conflict state is unknown. Say
		// so rather than guessing either way.
		return true, "could not determine whether this conflicts: " + err.Error()
	}
	if len(files) == 0 {
		return false, ""
	}
	return true, "conflicting files: " + strings.Join(files, ", ")
}

// abortMerge clears a failed merge so the next branch isn't blocked by
// leftover conflict state. A failure here is not worth reporting: the
// original error is what matters, and the user's next command will say
// an operation is in progress if something really is.
func abortMerge() {
	if _, found, err := git.InProgressOperation(); err != nil || !found {
		return
	}
	if err := git.AbortMerge(); err != nil {
		log.Debugf("could not abort the failed merge: %v", err)
	}
}

// resolveSource picks the branch to sync from: the flag, the current
// branch when it is a configured base, or the first configured base.
func resolveSource(cfg *config.Config, flag string) (string, error) {
	if flag != "" {
		return flag, nil
	}

	if current, err := git.CurrentBranch(); err == nil {
		if _, ok := IsConfiguredBase(cfg, current); ok {
			return current, nil
		}
	}

	keys := cfg.BaseKeys()
	if len(keys) == 0 {
		return "", fmt.Errorf("no base branch is configured — set one in .gix/config")
	}
	return resolveBranchName(cfg, keys[0]), nil
}

// syncCandidates lists the branches a sync run may touch.
//
// Without autoOnly, every topic branch is eligible except the source —
// the default is to bring the base branch's changes to all work in
// progress.
//
// With autoOnly, only branches the config marked `autosync: true` are
// eligible. Both roles can carry that flag: on a topic kind it means
// "take updates from this kind's parent", and on a base branch it means
// "take updates from this branch's parent" — which is how the default
// config already describes develop following main.
func syncCandidates(cfg *config.Config, source string, autoOnly bool) ([]string, error) {
	health, err := CheckBranches(cfg)
	if err != nil {
		return nil, err
	}

	var out []string
	for _, h := range health {
		if h.Name == source {
			continue
		}
		if autoOnly {
			if !autoSyncs(cfg, h) {
				continue
			}
		} else if h.Role != config.RoleTopic {
			continue
		}
		out = append(out, h.Name)
	}
	return out, nil
}

// autoSyncs reports whether a branch is marked to receive updates from its
// parent, and that the parent is a branch that exists. A base branch is
// only eligible when its own entry says so; gix never merges into one
// just because it is a base.
func autoSyncs(cfg *config.Config, h BranchHealth) bool {
	if h.AutoSync {
		return true
	}
	if h.Kind == "" {
		return false
	}
	spec, ok := cfg.Spec(h.Kind)
	return ok && spec.AutoSync
}

// strategyFor returns how a branch should take the source in, taken from
// the downstream_strategy its kind already has configured.
func strategyFor(cfg *config.Config, branch string) config.Strategy {
	kind, spec, ok := findKind(cfg, branch)
	if !ok {
		return config.StrategyMerge
	}
	log.Verbosef("syncing %s into %s via %s (%s kind)", branch, kind, spec.Downstream(), kind)
	return spec.Downstream()
}

// SyncError reports that a sync run finished with branches it could not
// merge, while making clear the other branches were handled.
type SyncError struct {
	Result *SyncResult
}

func (e *SyncError) Error() string {
	var failed []string
	for _, entry := range e.Result.Entries {
		if !entry.OK() {
			failed = append(failed, entry.Branch)
		}
	}
	if e.Result.DryRun {
		return fmt.Sprintf("%s would conflict — nothing was changed", strings.Join(failed, ", "))
	}

	source := ""
	if len(e.Result.Entries) > 0 {
		source = e.Result.Entries[0].Source
	}
	return fmt.Sprintf("%s could not take the merge from %s — the other branches were synced",
		strings.Join(failed, ", "), source)
}

// ErrSyncConflicts matches a sync that reported at least one conflict, so
// a caller can exit non-zero without inspecting the entries.
var ErrSyncConflicts = errors.New("sync could not merge every branch")

func IsSyncConflict(err error) bool { return errors.Is(err, ErrSyncConflicts) }
