package commands

import (
	"fmt"

	"github.com/m-mdy-m/gix/internal/cli"
	"github.com/m-mdy-m/gix/internal/config"
	"github.com/m-mdy-m/gix/internal/ui"
	"github.com/spf13/cobra"
)

// configCommands registers `gix config ...`: inspecting and editing
// .gix/config from the command line instead of by hand.
type configCommands struct{}

func (configCommands) Register(root *cobra.Command, ctx *cli.Context) {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect or edit the gix flow config",
	}
	configCmd.AddCommand(newConfigListCmd(ctx))
	configCmd.AddCommand(newConfigSetCmd(ctx))
	root.AddCommand(configCmd)
}

func newConfigListCmd(ctx *cli.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Print the current flow config",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, cfg, err := ctx.Config()
			if err != nil {
				return err
			}

			ui.Info("config: %s", config.Path(root))
			ui.Field("remote", "%s", cfg.Remote)
			ui.Field("main", "%s", cfg.MainName())
			ui.Field("develop", "%s", cfg.DevelopName())

			for _, kind := range cfg.Kinds() {
				spec, ok := cfg.Spec(kind)
				if !ok {
					continue
				}
				fmt.Println()
				ui.Info("%s", kind)
				ui.Field("prefix", "%s", spec.Prefix)
				ui.Field("parent", "%s", spec.Parent)
				ui.Field("merge-into", "%s", joinKinds(spec.MergeTargets()))
				ui.Field("upstream", "%s", spec.Upstream())
				ui.Field("downstream", "%s", spec.Downstream())
				ui.Field("tag", "%t", spec.Tag)
				ui.Field("delete", "%t", spec.DeleteOnFinish)
			}
			return nil
		},
	}
}

func newConfigSetCmd(ctx *cli.Context) *cobra.Command {
	var prefix, upstream, downstream string
	var tag, deleteOnFinish string // tri-state: "", "true", "false" — see parseTriState

	cmd := &cobra.Command{
		Use:   "set <kind>",
		Short: "Change one branch kind's settings",
		Long: `Changes settings for a single topic branch kind (feature, bugfix, hotfix,
release, or any kind you've added). Only the flags you pass are
changed; everything else in .gix/config is left as-is.

Examples:
  gix config set release --upstream-strategy=squash
  gix config set feature --prefix=feat/
  gix config set hotfix --tag=false`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, cfg, err := ctx.Config()
			if err != nil {
				return err
			}

			kind := args[0]
			spec, ok := cfg.Spec(kind)
			if !ok || spec.Role != config.RoleTopic {
				return fmt.Errorf("unknown topic branch kind %q (see `gix config list`)", kind)
			}

			changed := false

			if prefix != "" {
				spec.Prefix = prefix
				changed = true
			}
			if upstream != "" {
				s := config.Strategy(upstream)
				if !s.Valid() {
					return fmt.Errorf("invalid --upstream-strategy %q (want merge, rebase, or squash)", upstream)
				}
				spec.UpstreamStrategy = s
				changed = true
			}
			if downstream != "" {
				s := config.Strategy(downstream)
				if !s.Valid() {
					return fmt.Errorf("invalid --downstream-strategy %q (want merge, rebase, or squash)", downstream)
				}
				spec.DownstreamStrategy = s
				changed = true
			}
			if tag != "" {
				v, err := parseTriState(tag)
				if err != nil {
					return fmt.Errorf("--tag: %w", err)
				}
				spec.Tag = v
				changed = true
			}
			if deleteOnFinish != "" {
				v, err := parseTriState(deleteOnFinish)
				if err != nil {
					return fmt.Errorf("--delete-on-finish: %w", err)
				}
				spec.DeleteOnFinish = v
				changed = true
			}

			if !changed {
				return fmt.Errorf("no changes given — pass at least one of --prefix, --upstream-strategy, --downstream-strategy, --tag, --delete-on-finish")
			}

			cfg.Branch[kind] = spec
			if err := config.Save(root, cfg); err != nil {
				return err
			}

			ui.Success("updated %s", kind)
			return nil
		},
	}

	cmd.Flags().StringVar(&prefix, "prefix", "", "branch name prefix, e.g. feature/")
	cmd.Flags().StringVar(&upstream, "upstream-strategy", "", "merge, rebase, or squash")
	cmd.Flags().StringVar(&downstream, "downstream-strategy", "", "merge or rebase")
	cmd.Flags().StringVar(&tag, "tag", "", "true or false: whether finish accepts --tag")
	cmd.Flags().StringVar(&deleteOnFinish, "delete-on-finish", "", "true or false: whether finish deletes the branch by default")

	return cmd
}

func parseTriState(s string) (bool, error) {
	switch s {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("want true or false, got %q", s)
	}
}
