package cli

import (
	"github.com/m-mdy-m/gix/internal/logger"
	"github.com/m-mdy-m/gix/internal/ui"
	"github.com/spf13/cobra"
)

type GlobalFlags struct {
	Verbose bool
	Debug   bool
	NoColor bool
}

func BindGlobalFlags(root *cobra.Command) *GlobalFlags {
	flags := &GlobalFlags{}

	root.PersistentFlags().BoolVar(&flags.Verbose, "verbose", false, "print each git command gix runs")
	root.PersistentFlags().BoolVar(&flags.Debug, "debug", false, "print verbose output plus raw git output and timing")
	root.PersistentFlags().BoolVar(&flags.NoColor, "no-color", false, "disable colored output")

	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		switch {
		case flags.Debug:
			logger.SetLevel(logger.LevelDebug)
		case flags.Verbose:
			logger.SetLevel(logger.LevelVerbose)
		}
		if flags.NoColor {
			ui.DisableColor()
		}
		return nil
	}

	return flags
}

func WithNoDelete(cmd *cobra.Command, dst *bool) {
	cmd.Flags().BoolVar(dst, "no-delete", false, "keep the branch after merging instead of deleting it")
}

func WithTag(cmd *cobra.Command, dst *string) {
	cmd.Flags().StringVar(dst, "tag", "", "create an annotated tag with this name after merging")
}

func WithStrategy(cmd *cobra.Command, dst *string) {
	cmd.Flags().StringVar(dst, "strategy", "", "override the merge strategy for this run: merge, rebase, or squash")
}
