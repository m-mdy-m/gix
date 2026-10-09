package commands

import (
	"fmt"
	"strconv"

	"github.com/m-mdy-m/gix/internal/cli"
	"github.com/m-mdy-m/gix/internal/flow"
	"github.com/m-mdy-m/gix/internal/git"
	"github.com/m-mdy-m/gix/internal/ui"
	"github.com/spf13/cobra"
)

// branchHealthCommands registers `gix branch ...`: inspecting the branches
// in the flow and what state each one is in. Read-only — nothing here
// creates, moves, or deletes anything.
type branchHealthCommands struct{}

func (branchHealthCommands) Register(root *cobra.Command, ctx *cli.Context) {
	cmd := &cobra.Command{
		Use:   "branch",
		Short: "Inspect and sync the branches in the flow",
		Long: `branch works with the branches in the flow: inspecting their state,
previewing cleanup, and bringing the base branch's changes into work in
progress.

doctor and prune only report. sync merges, and never rewrites history.

To start or finish a branch, use gix <kind> start|finish.`,
	}
	cmd.AddCommand(newBranchDoctorCmd(ctx))
	cmd.AddCommand(newBranchPruneCmd(ctx))
	cmd.AddCommand(newBranchSyncCmd(ctx))
	root.AddCommand(cmd)
}

func newBranchDoctorCmd(ctx *cli.Context) *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Show each branch's upstream, ahead/behind, and merge state",
		Long: `Reports one row per local branch: its config kind, the upstream it
tracks, how far ahead or behind it is, and whether its merge targets
already contain its work.

States:

  tracked     tracks a remote branch and matches it
  ahead       has commits its upstream doesn't — push would be needed
  behind      its upstream moved on — the commits are not merged in
  diverged    ahead and behind at once — a plain push would be rejected
  untracked   has no upstream configured
  gone        had an upstream, but that ref no longer exists locally

LAST COMMIT is how long ago the branch tip moved. gix reports the age
rather than deciding what counts as stale — only you know whether a
branch untouched for three weeks is abandoned or deliberately parked.

Nothing is changed. This is a report.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cfg, err := ctx.Config()
			if err != nil {
				return err
			}

			health, err := flow.CheckBranches(cfg)
			if err != nil {
				return err
			}
			if len(health) == 0 {
				ui.Dim("no local branches")
				return nil
			}

			current, _ := git.CurrentBranch()
			renderBranchHealth(health, current, all)
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "include branches that are in sync and healthy")

	return cmd
}

func renderBranchHealth(health []flow.BranchHealth, current string, all bool) {
	rows := [][]string{
		{"BRANCH", "KIND", "UPSTREAM", "AHEAD", "BEHIND", "LAST COMMIT", "STATE"},
	}

	shown := 0
	for _, h := range health {
		if !all && !h.NeedsWork() {
			continue
		}

		name := h.Name
		if h.Name == current {
			name += " (current)"
		}
		rows = append(rows, []string{
			name,
			branchKind(h),
			orDash(h.Upstream),
			strconv.Itoa(h.Ahead),
			strconv.Itoa(h.Behind),
			ageCell(h.AgeDays),
			string(h.State),
		})
		shown++
	}

	if shown == 0 {
		ui.Success("every branch is in sync with its upstream")
		ui.Dim("  re-run with --all to see them all")
		return
	}

	ui.Table(nil, rows)

	for _, h := range health {
		if !h.NeedsWork() {
			continue
		}
		for _, hint := range branchHints(h) {
			ui.Step("%s", hint)
		}
	}
}

// branchKind shows the config kind, so a row is traceable back to the
// setting that produced it.
func branchKind(h flow.BranchHealth) string {
	switch {
	case h.Kind != "":
		return h.Kind
	case h.Role == "base":
		return "base"
	default:
		return "-"
	}
}

func branchHints(h flow.BranchHealth) []string {
	var hints []string

	if h.Finished() {
		hints = append(hints, fmt.Sprintf("%s is already merged into %s — `gix branch prune` lists it", h.Name, joinNames(h.MergedInto)))
	}

	switch h.State {
	case flow.StateBehind:
		hints = append(hints, fmt.Sprintf("%s is %d commit(s) behind %s — merge or rebase to bring them in", h.Name, h.Behind, h.Upstream))
	case flow.StateDiverged:
		hints = append(hints, fmt.Sprintf("%s has diverged from %s — resolve that before pushing", h.Name, h.Upstream))
	case flow.StateGone:
		hints = append(hints, fmt.Sprintf("%s tracks %s, which no longer exists — the remote branch was deleted or not fetched", h.Name, h.Upstream))
	case flow.StateUntracked:
		if h.Kind != "" {
			hints = append(hints, fmt.Sprintf("%s has no upstream — `git push -u <remote> %s` to start tracking", h.Name, h.Name))
		}
	case flow.StateAhead:
		hints = append(hints, fmt.Sprintf("%s has %d unpushed commit(s)", h.Name, h.Ahead))
	}

	return hints
}

func newBranchSyncCmd(ctx *cli.Context) *cobra.Command {
	var dryRun bool
	var auto bool
	var from string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Bring a source branch into every eligible branch",
		Long: `Brings a source branch into the branches that want it, so a change
that landed on the base branch reaches the work in progress.

The source defaults to the base branch you are standing on, or the first
configured base branch.

By default every topic branch is eligible. --auto narrows the run to the
branches .gix/config marks 'autosync: true', which is how you opt into
updating one kind or one base branch instead of all of them:

  feature:
    autosync: true      # these take updates from develop
  develop:
    parent: main
    autosync: true      # develop takes updates from main

Sync is deliberately conservative:

  - it refuses to run on a dirty working tree or during an unfinished
    merge or rebase
  - branches that already contain the source are skipped
  - a branch that would conflict is reported and left alone — gix never
    resolves a conflict for you
  - one branch failing does not stop the rest, and the command exits
    non-zero so CI notices
  - the source branch is never modified
  - only merging is used, never rebase, so no history is rewritten

--dry-run shows exactly what would happen and changes nothing.

gix never syncs on its own. Nothing here runs unless you ask for it:
there are no hooks and no background process.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cfg, err := ctx.Config()
			if err != nil {
				return err
			}

			origin, _ := git.CurrentBranch()
			result, err := flow.Sync(cfg, flow.SyncOptions{Source: from, DryRun: dryRun, AutoOnly: auto})
			if err != nil {
				return err
			}

			renderSync(result, syncMode(dryRun, auto), origin)
			if result.OK() {
				return nil
			}
			return &flow.SyncError{Result: result}
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would happen without changing anything")
	cmd.Flags().BoolVar(&auto, "auto", false, "only update branches .gix/config marks autosync: true")
	cmd.Flags().StringVar(&from, "from", "", "branch to bring in (default: the base branch you are on, or the first configured base)")

	return cmd
}

func syncMode(dryRun, auto bool) string {
	switch {
	case dryRun && auto:
		return "dry run (auto) — nothing was changed"
	case dryRun:
		return "dry run — nothing was changed"
	case auto:
		return "syncing branches marked autosync"
	default:
		return "syncing"
	}
}

func renderSync(result *flow.SyncResult, heading, origin string) {
	if len(result.Entries) == 0 {
		ui.Success("nothing to sync — no branch is waiting for those changes")
		return
	}

	ui.Info("%s", heading)
	if origin != "" {
		ui.Dim("  started on %s", origin)
	}

	rows := [][]string{{"BRANCH", "WOULD"}}
	if !result.DryRun {
		rows[0][1] = "RESULT"
	}
	for _, e := range result.Entries {
		state := string(e.State)
		if result.DryRun {
			state = syncFuture(e.State)
		}
		rows = append(rows, []string{e.Branch, syncLabel(e.State, state, e.Detail)})
	}
	ui.Table(nil, rows)

	for _, e := range result.Entries {
		if e.Detail != "" && !e.OK() {
			ui.Step("%s: %s", e.Branch, e.Detail)
		}
	}

	if result.DryRun {
		ui.Dim("")
		ui.Dim("this was a preview — gix has not changed anything")
	}
}

// syncFuture turns a real outcome into what a dry run should say about it.
func syncFuture(state flow.SyncState) string {
	if state == flow.SyncMerged {
		return string(flow.SyncWouldMerge)
	}
	return string(state)
}

// syncLabel gives each outcome a marker plus a short phrase, so a wall of
// identical state names still reads at a glance.
func syncLabel(state flow.SyncState, text, detail string) string {
	switch state {
	case flow.SyncMerged:
		return "✓ " + text
	case flow.SyncConflict:
		return "✗ " + text
	case flow.SyncSkipped, flow.SyncFailed:
		return "! " + text
	default:
		return text
	}
}

func newBranchPruneCmd(ctx *cli.Context) *cobra.Command {
	var apply bool

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "List the branches that could be cleaned up, or delete them",
		Long: `Lists branches whose work is already merged away, or whose upstream
no longer exists — the branches left behind after a finished feature.

Without --apply this is a preview and deletes nothing.

--apply deletes the branches that are fully merged, using
'git branch -d'. Never -D: git refuses to delete a branch whose commits
are not reachable from its upstream, and that refusal is the safety net
this relies on. A branch whose upstream merely disappeared is not
necessarily merged, so it is listed and left alone — remove it yourself
if you are sure.

Base branches and the branch you are standing on are never candidates,
and a branch with unmerged work is never a candidate.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cfg, err := ctx.Config()
			if err != nil {
				return err
			}

			health, err := flow.CheckBranches(cfg)
			if err != nil {
				return err
			}

			current, _ := git.CurrentBranch()
			plan := flow.PlanPrune(cfg, health, current)

			if plan.Empty() {
				ui.Success("nothing to prune")
				return nil
			}

			if !apply {
				rows := [][]string{{"BRANCH", "KIND", "WHY"}}
				rows = append(rows, pruneRows(plan)...)

				ui.Info("%d branch(es) could be cleaned up", len(plan.Entries))
				ui.Table(nil, rows)
				ui.Dim("")
				ui.Dim("this is a preview — gix has not deleted anything")
				ui.Dim("re-run with --apply to delete the ones that are fully merged")
				return nil
			}

			result, err := flow.ApplyPrune(plan)
			if err != nil {
				return err
			}
			renderPrune(result)
			if len(result.Kept) > 0 {
				return fmt.Errorf("%s could not be deleted — nothing unmerged was touched", joinNames(result.Kept))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&apply, "apply", false, "delete the branches that are fully merged (never uses -D)")

	return cmd
}

func renderPrune(result *flow.PruneResult) {
	for _, o := range result.Outcomes {
		if o.Deleted {
			ui.Success("deleted %s", o.Name)
			ui.Dim("  %s", o.Reason)
			continue
		}
		ui.Warn("kept %s", o.Name)
		if o.Detail != "" {
			ui.Dim("  %s", o.Detail)
		} else {
			ui.Dim("  %s", o.Reason)
		}
	}
}

func pruneRows(plan flow.PrunePlan) [][]string {
	rows := make([][]string, 0, len(plan.Entries))
	for _, e := range plan.Entries {
		rows = append(rows, []string{e.Name, orDash(e.Kind), e.Reason})
	}
	return rows
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// ageCell formats days since the last commit. gix reports the age and
// leaves "stale" to you: only you know whether a branch untouched for
// three weeks is abandoned or deliberately parked.
func ageCell(days int) string {
	switch {
	case days < 0:
		return "no commits"
	case days == 0:
		return "today"
	case days == 1:
		return "1 day ago"
	default:
		return fmt.Sprintf("%d days ago", days)
	}
}

func joinNames(names []string) string {
	out := ""
	for i, n := range names {
		if i > 0 {
			out += " and "
		}
		out += n
	}
	return out
}
