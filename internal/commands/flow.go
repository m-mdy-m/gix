package commands

import (
	"fmt"

	"github.com/m-mdy-m/gix/internal/cli"
	"github.com/m-mdy-m/gix/internal/config"
	"github.com/m-mdy-m/gix/internal/flow"
	"github.com/m-mdy-m/gix/internal/ui"
	"github.com/spf13/cobra"
)

// flowCommands registers `gix flow ...`: commands about the flow setup
// itself, as opposed to individual branches within it.
type flowCommands struct{}

func (flowCommands) Register(root *cobra.Command, ctx *cli.Context) {
	flowCmd := &cobra.Command{
		Use:   "flow",
		Short: "Manage the gix branch flow itself",
	}
	flowCmd.AddCommand(newFlowInitCmd(ctx))
	root.AddCommand(flowCmd)
}

func newFlowInitCmd(ctx *cli.Context) *cobra.Command {
	var mainName, developName string
	var noCommit bool
	var authorName, authorEmail string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize gix in the current repository",
		Long: `Creates .gix/config with the branch model gix uses (main, develop,
feature/, bugfix/, hotfix/, release/ by default), commits it, and
creates the develop branch if it doesn't exist yet.

init only ever touches .gix/config and (if needed) the develop
branch — it does not require a clean working tree, since it doesn't
touch anything else.

Committing the generated config requires a Git author identity
(user.name/user.email). If none is configured, init still writes
.gix/config and creates develop, but leaves the config uncommitted
and tells you how to finish: configure your identity and re-run
'gix flow init', or pass --author-name/--author-email for one-off
use, or pass --no-commit to always leave it uncommitted.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := ctx.Repo()
			if err != nil {
				return err
			}

			if (authorName == "") != (authorEmail == "") {
				return fmt.Errorf("--author-name and --author-email must be given together")
			}

			result, err := flow.Init(root, flow.InitOptions{
				MainName:    mainName,
				DevelopName: developName,
				NoCommit:    noCommit,
				AuthorName:  authorName,
				AuthorEmail: authorEmail,
			})
			if err != nil {
				return err
			}
			cfg := result.Config

			ui.Success("gix initialized")
			ui.Field("config", "%s", config.Path(root))
			ui.Field("main", "%s", cfg.MainName())
			ui.Field("develop", "%s", cfg.DevelopName())
			ui.Field("kinds", "%s", joinKinds(cfg.Kinds()))

			switch {
			case result.Committed:
				ui.Step("committed .gix/config")
			case result.CommitSkipped:
				ui.Warn("config was written but not committed (%s)", result.SkipReason)
				if result.SkipReason == "no git author identity is configured" {
					ui.Dim("  configure your identity, then run:")
					ui.Dim("    git add %s && git commit -m \"chore(gix): initialize gix flow config\"", config.DirName+"/"+config.FileName)
					ui.Dim("  or re-run `gix flow init`")
					ui.Dim("  (git config --global user.name \"...\" && git config --global user.email \"...\")")
				}
			}

			switch {
			case result.DevelopCreated:
				ui.Step("created %s from %s", cfg.DevelopName(), cfg.MainName())
			case result.DevelopExisted:
				ui.Dim("  %s already existed, left as-is", cfg.DevelopName())
			}

			ui.Info("next: gix feature start <name>")
			return nil
		},
	}

	cmd.Flags().StringVar(&mainName, "main", "", "name of the main branch (default: main, or current branch if it's main/master/trunk)")
	cmd.Flags().StringVar(&developName, "develop", "", "name of the develop branch (default: develop)")
	cmd.Flags().BoolVar(&noCommit, "no-commit", false, "write .gix/config but don't commit it")
	cmd.Flags().StringVar(&authorName, "author-name", "", "author name to commit .gix/config with, for this run only (requires --author-email)")
	cmd.Flags().StringVar(&authorEmail, "author-email", "", "author email to commit .gix/config with, for this run only (requires --author-name)")

	return cmd
}

func joinKinds(kinds []string) string {
	out := ""
	for i, k := range kinds {
		if i > 0 {
			out += ", "
		}
		out += k
	}
	return out
}
