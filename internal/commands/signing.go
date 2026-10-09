package commands

import (
	"github.com/m-mdy-m/gix/internal/cli"
	"github.com/m-mdy-m/gix/internal/flow"
	"github.com/m-mdy-m/gix/internal/git"
	"github.com/m-mdy-m/gix/internal/ui"
	"github.com/spf13/cobra"
)

// signingCommands registers `gix signing ...`: what the repository says
// about commit and tag signing, and whether git can actually do it.
type signingCommands struct{}

func (signingCommands) Register(root *cobra.Command, ctx *cli.Context) {
	cmd := &cobra.Command{
		Use:   "signing",
		Short: "Inspect Git commit and tag signing",
		Long: `signing reports how this repository is set up to sign commits and
tags: the format (openpgp, ssh, or x509), the configured key, whether
commits and tags are signed at all, and whether git has what it needs
to do it.

gix only reads configuration and looks for the signing program on PATH.
It never signs anything, never creates keys, and never prints key
material.`,
	}
	cmd.AddCommand(newSigningStatusCmd(ctx))
	root.AddCommand(cmd)
}

func newSigningStatusCmd(ctx *cli.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show how signing is configured",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := ctx.Repo(); err != nil {
				return err
			}

			cfg, err := git.ReadConfig()
			if err != nil {
				return err
			}
			signing := flow.InspectSigning(cfg)

			ui.Info("signing")
			ui.Field("format", "%s", signing.Format)
			ui.Field("commits", "%t", signing.SigningEnabled())
			ui.Field("tags", "%t", signing.TagSign)

			switch {
			case signing.Key == "":
				ui.Dim("  no signing key configured (user.signingkey is unset)")
			default:
				ui.Field("key", "%s", signing.Key)
				ui.Dim("  key configured in %s config", signing.KeyScope)
			}

			switch {
			case signing.Program == "":
				ui.Field("program", "%s", "built into git")
			default:
				ui.Field("program", "%s", signing.Program)
			}

			if !signing.SigningEnabled() && signing.Key == "" {
				ui.Dim("")
				ui.Dim("signing is off — git will create unsigned commits and tags")
				return nil
			}

			if signing.Available {
				ui.Success("git can sign with this setup")
				return nil
			}

			ui.Warn("git cannot sign with this setup")
			if signing.Program != "" {
				ui.Dim("  %s was not found on PATH", signing.Program)
				ui.Dim("  install it, or point git elsewhere with `git config gpg.program <path>`")
			}
			ui.Dim("  check `gpg.format` — it must be openpgp, ssh, or x509")
			return nil
		},
	}
}
