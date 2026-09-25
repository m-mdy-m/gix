package commands

import (
	"github.com/m-mdy-m/gix/internal/cli"
	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags (see Makefile).
var Version = "dev"

var registrables = []cli.Registrable{
	cli.RegistrableFunc(registerInit),
	flowCommands{},
	branchKindCommands{},
	cli.RegistrableFunc(registerStatus),
	cli.RegistrableFunc(registerList),
	configCommands{},
}

// New builds the root `gix` command with every subcommand registered
// and global flags bound.
func New() *cobra.Command {
	root := &cobra.Command{
		Use:     "gix",
		Short:   "A personal git-flow wrapper",
		Version: Version,
		Long: `gix is a small, config-driven wrapper around Git for a git-flow-style
branching model:

  main
   └── develop
        ├── feature/*   branched from develop, merged back into develop
        ├── bugfix/*    branched from develop, merged back into develop
        ├── hotfix/*    branched from main, merged into main + develop, taggable
        └── release/*   branched from develop, merged into main + develop, taggable

The exact branch names, prefixes, parents, and merge strategy per kind
live in .gix/config — see 'gix config list'. gix doesn't try to support
every team's workflow; it automates this one, so branch creation,
merging, and cleanup are a single command instead of five.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	ctx := &cli.Context{}
	cli.BindGlobalFlags(root)

	for _, r := range registrables {
		r.Register(root, ctx)
	}

	return root
}
