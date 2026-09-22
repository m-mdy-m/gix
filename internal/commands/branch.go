package commands

import (
	"fmt"

	"github.com/m-mdy-m/gix/internal/cli"
	"github.com/m-mdy-m/gix/internal/config"
	"github.com/m-mdy-m/gix/internal/flow"
	"github.com/m-mdy-m/gix/internal/ui"
	"github.com/spf13/cobra"
)

type branchKindCommands struct{}

// wellKnownKinds mirrors config.Default()'s topic kinds.
var wellKnownKinds = []string{"feature", "bugfix", "hotfix", "release"}

func (branchKindCommands) Register(root *cobra.Command, ctx *cli.Context) {
	for _, kind := range wellKnownKinds {
		root.AddCommand(newBranchKindCmd(ctx, kind))
	}
}

// this func builds the `gix feature ...` / `gix release ...` /
// etc. command group. Every kind shares the same start/finish shape;
// per-kind behavior (parent, merge targets, tagging, strategy) comes
// entirely from config at run time, not from anything decided here.
func newBranchKindCmd(ctx *cli.Context, kind string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   kind,
		Short: fmt.Sprintf("Manage %s branches", kind),
	}

	cmd.AddCommand(newStartCmd(ctx, kind))
	cmd.AddCommand(newFinishCmd(ctx, kind))

	return cmd
}

func newStartCmd(ctx *cli.Context, kind string) *cobra.Command {
	return &cobra.Command{
		Use:   "start <name>",
		Short: fmt.Sprintf("Create and switch to a new %s branch", kind),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cfg, err := ctx.Config()
			if err != nil {
				return err
			}

			full, err := flow.Start(cfg, kind, args[0])
			if err != nil {
				return err
			}

			ui.Success("switched to new branch %s", full)
			if spec, ok := cfg.Spec(kind); ok {
				ui.Dim("  from %s", resolveDisplayName(cfg, spec.Parent))
			}
			return nil
		},
	}
}

func newFinishCmd(ctx *cli.Context, kind string) *cobra.Command {
	var noDelete bool
	var tag string
	var strategy string

	cmd := &cobra.Command{
		Use:   "finish <name>",
		Short: fmt.Sprintf("Merge a %s branch back and clean it up", kind),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cfg, err := ctx.Config()
			if err != nil {
				return err
			}

			spec, ok := cfg.Spec(kind)
			if !ok {
				return fmt.Errorf("unknown branch kind %q", kind)
			}

			opts := flow.FinishOptions{
				DeleteBranch: spec.DeleteOnFinish && !noDelete,
				Tag:          tag,
				Strategy:     config.Strategy(strategy),
			}

			result, err := flow.Finish(cfg, kind, args[0], opts)
			if err != nil {
				return err
			}

			ui.Success("finished %s", result.Branch)
			ui.Dim("  strategy: %s", result.Strategy)
			for _, target := range result.MergedInto {
				ui.Step("merged into %s", target)
			}
			for _, target := range result.AlreadyMerged {
				ui.Step("%s already up to date (merged in a previous run)", target)
			}
			if result.Tagged != "" {
				ui.Step("tagged %s on %s", result.Tagged, result.TaggedOn)
			}
			if result.BranchDeleted {
				ui.Step("deleted %s", result.Branch)
			} else if result.DeleteAttempted && result.DeleteErr != nil {
				ui.Warn("could not delete %s: %s", result.Branch, result.DeleteErr)
				ui.Dim("  branch was NOT removed — merge and tag (if any) succeeded")
			}
			return nil
		},
	}

	cli.WithNoDelete(cmd, &noDelete)
	cli.WithStrategy(cmd, &strategy)
	if spec, ok := config.Default().Spec(kind); ok && spec.Tag {
		cli.WithTag(cmd, &tag)
	}

	return cmd
}

func resolveDisplayName(cfg *config.Config, key string) string {
	if spec, ok := cfg.Spec(key); ok && spec.Name != "" {
		return spec.Name
	}
	return key
}
