package commands

import (
	"github.com/m-mdy-m/gix/internal/cli"
	"github.com/m-mdy-m/gix/internal/logger"
	"github.com/spf13/cobra"
)

var log = logger.WithPrefix("commands")

// Version is set at build time via -ldflags (see Makefile).
var Version = "dev"

var registrables = []cli.Registrable{
	cli.RegistrableFunc(registerInit),
	flowCommands{},
	branchKindCommands{},
	cli.RegistrableFunc(registerStatus),
	cli.RegistrableFunc(registerList),
	configCommands{},
	doctorCommands{},
	signingCommands{},
	branchHealthCommands{},
	versionCommands{},
	cli.RegistrableFunc(registerUpdate),
}

func registerUpdate(root *cobra.Command, _ *cli.Context) {
	root.AddCommand(newUpdateCmd())
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
merging, and cleanup are a single command instead of five.

A topic kind added to .gix/config gets its own start/finish command
group, so 'gix wip start api' works with no code changes.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	ctx := &cli.Context{}
	cli.BindGlobalFlags(root)

	for _, r := range registrables {
		r.Register(root, ctx)
	}
	registerConfiguredKinds(root, ctx)

	return root
}

// registerConfiguredKinds gives every extra topic kind in .gix/config its
// own start/finish command group, so adding a kind is a config edit
// rather than a code change.
//
// Reading the config here is best-effort: outside a repository, in a
// repository without .gix/config, or with a config that doesn't validate,
// only the built-in groups exist, which is exactly how gix behaved
// before. The built-in kinds are registered unconditionally so their
// help text is the same with or without a config.
func registerConfiguredKinds(root *cobra.Command, ctx *cli.Context) {
	_, cfg, err := ctx.Config()
	if err != nil {
		log.Verbosef("no usable config, keeping only the built-in kinds: %v", err)
		return
	}

	registered := make(map[string]bool, len(wellKnownKinds))
	for _, kind := range wellKnownKinds {
		registered[kind] = true
	}
	// A config key must never be able to shadow a command gix already
	// offers: the built-in kind groups, and the base branch keys.
	for _, name := range []string{"branch", "config", "doctor", "flow", "init", "list", "signing", "status", "update", "upgrade", "version"} {
		registered[name] = true
	}
	for _, key := range cfg.BaseKeys() {
		registered[key] = true
	}

	for _, kind := range cfg.Kinds() {
		if registered[kind] {
			continue
		}
		root.AddCommand(newBranchKindCmd(ctx, kind))
		registered[kind] = true
		log.Verbosef("registered command group for configured kind %q", kind)
	}
}
