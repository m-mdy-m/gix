package commands

import (
	"fmt"

	"github.com/m-mdy-m/gix/internal/cli"
	"github.com/m-mdy-m/gix/internal/flow"
	"github.com/m-mdy-m/gix/internal/ui"
	"github.com/spf13/cobra"
)

// versionCommands registers `gix version ...`: what the repository's
// release version is, and how to move it forward.
type versionCommands struct{}

func (versionCommands) Register(root *cobra.Command, ctx *cli.Context) {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show and bump the release version",
		Long: `version reads the repository's release version from its tags — v0.9.4
means a tag called v0.9.4 — and can create the next one.

There is no version file to keep in step with: the tags are the record.
A tag gix cannot read as an exact SemVer version — major.minor.patch with
an optional prerelease such as -rc.1 — is ignored rather than guessed at.`,
	}
	cmd.AddCommand(newVersionShowCmd(ctx))
	cmd.AddCommand(newVersionBumpCmd(ctx))
	root.AddCommand(cmd)
}

func newVersionShowCmd(ctx *cli.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show the current release version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := ctx.Repo(); err != nil {
				return err
			}

			plan, err := flow.PlanBump(flow.LevelPatch)
			if err != nil {
				return err
			}

			if !plan.Tagged {
				ui.Info("no release tags yet")
				ui.Field("commits", "%d", plan.CommitsSince)
				ui.Dim("  the first release would be %s, with `gix version bump minor`", flow.Version{}.Bump(flow.LevelMinor).Tag())
				return nil
			}

			ui.Info("version")
			ui.Field("current", "%s", plan.Current)
			ui.Field("tag", "%s", plan.CurrentTag)
			ui.Field("commits", "%d since %s", plan.CommitsSince, plan.CurrentTag)
			return nil
		},
	}
}

func newVersionBumpCmd(ctx *cli.Context) *cobra.Command {
	var (
		dryRun  bool
		pre     string
		promote bool
	)

	cmd := &cobra.Command{
		Use:   "bump [patch|minor|major] [--pre <name> | --release]",
		Short: "Create the next release tag",
		Long: `Calculates the next version from the highest existing v* tag and
creates it:

  patch   0.4.3 → 0.4.4
  minor   0.4.3 → 0.5.0
  major   0.4.3 → 1.0.0

With no tags yet, the first release is 0.1.0.

A level always makes a stable version from the core of the current one,
so on a prerelease it moves past it rather than promoting it:

  minor   1.0.0-rc.1 → 1.1.0
  patch   1.0.0-rc.1 → 1.0.1
  major   1.0.0-rc.1 → 2.0.0

Prereleases follow SemVer 2.0.0 (alpha.1 < beta.1 < rc.1 < rc.2 < rc.10 < 1.0.0):

  minor --pre rc   0.4.3       → 0.5.0-rc.1   start a prerelease
  --pre rc         1.0.0-rc.1  → 1.0.0-rc.2   next in the series
  --pre rc         1.0.0-beta.2 → 1.0.0-rc.1  move to a later name
  --release        1.0.0-rc.2  → 1.0.0        promote it as it is

A prerelease can only move forward: --pre beta on 1.0.0-rc.1 is refused.
--release takes no level and no --pre.

What bump does, and nothing more:

  - checks out nothing, changes no file, writes no commit
  - creates one annotated tag on the current commit
  - does not edit CHANGELOG.md, does not sign, does not push

Those are yours to decide. Make your release commit first, then tag it,
then push the tag. gix only chooses the number.

Releases are cut from the main branch, which is where gix already puts
release tags. Running it elsewhere is refused rather than warned about,
so you cannot tag a develop or feature commit by accident.

--dry-run prints what would happen and creates nothing.`,
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{"patch", "minor", "major"},
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cfg, err := ctx.Config()
			if err != nil {
				return err
			}

			var level flow.Level
			if len(args) == 1 {
				level = flow.Level(args[0])
				if !level.Valid() {
					return fmt.Errorf("unknown version level %q (want patch, minor, or major)", args[0])
				}
			}

			result, err := flow.Bump(cfg, flow.BumpOptions{Level: level, Pre: pre, Promote: promote, DryRun: dryRun})
			if err != nil {
				return err
			}

			renderBump(result, dryRun)
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would happen without creating anything")
	cmd.Flags().StringVar(&pre, "pre", "", "start or continue a prerelease with this name (alpha, beta, rc, ...)")
	cmd.Flags().BoolVar(&promote, "release", false, "promote the current prerelease to its stable release")
	cmd.MarkFlagsMutuallyExclusive("pre", "release")

	return cmd
}

func renderBump(result *flow.BumpResult, dryRun bool) {
	plan := result.Plan

	if dryRun {
		ui.Info("dry run — nothing was created")
	} else {
		ui.Info("release")
	}

	ui.Field("current", "%s", currentOrNone(plan))
	ui.Field("next", "%s", plan.Next)
	ui.Field("tag", "%s", plan.Next.Tag())
	if plan.Commit != "" {
		ui.Field("commit", "%s (%s)", plan.Commit, plan.Branch)
	}
	ui.Field("since", "%s", commitsSince(plan))
	if plan.Next.Pre != "" {
		ui.Field("kind", "prerelease of %s", plan.Next.Release())
	}

	switch {
	case dryRun:
		ui.Dim("")
		ui.Dim("gix would create the tag %s and nothing else", plan.Next.Tag())
	case result.Tagged:
		ui.Success("created %s", plan.Next.Tag())
		ui.Dim("  push it with: git push origin %s", plan.Next.Tag())
	default:
		ui.Dim("  nothing was created")
	}
}

func currentOrNone(plan *flow.BumpPlan) string {
	if !plan.Tagged {
		return "none — this would be the first release"
	}
	return fmt.Sprintf("%s (%s)", plan.Current, plan.CurrentTag)
}

func commitsSince(plan *flow.BumpPlan) string {
	if !plan.Tagged {
		return fmt.Sprintf("%d commit(s), no tags yet", plan.CommitsSince)
	}
	return fmt.Sprintf("%d commit(s)", plan.CommitsSince)
}
