package commands

import (
	"github.com/m-mdy-m/gix/internal/cli"
	"github.com/m-mdy-m/gix/internal/config"
	"github.com/m-mdy-m/gix/internal/flow"
	"github.com/m-mdy-m/gix/internal/git"
	"github.com/m-mdy-m/gix/internal/ui"
	"github.com/spf13/cobra"
)

func registerStatus(root *cobra.Command, ctx *cli.Context) {
	root.AddCommand(newStatusCmd(ctx))
}

func newStatusCmd(ctx *cli.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the current branch and where it sits in the flow",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cfg, err := ctx.Config()
			if err != nil {
				return err
			}

			developName := cfg.DevelopName()
			st, err := git.GetStatus(developName)
			if err != nil {
				return err
			}

			health := flow.CheckHealth(cfg)

			switch st.Head.Kind {
			case git.HeadDetached:
				ui.Info("detached HEAD at %s", st.Head.Commit)
				ui.Dim("  not on any branch — flow role: none")
			case git.HeadUnborn:
				ui.Info("on %s", st.Head.Branch)
				ui.Dim("  no commits yet")
			default:
				ui.Info("on %s", st.Branch)
				describeBranchRole(cfg, st.Branch, developName, health)
			}

			if st.Clean {
				ui.Dim("  working tree clean")
			} else {
				ui.Dim("  working tree has uncommitted changes")
			}

			if st.HasBase {
				ui.Dim("  %d ahead, %d behind %s", st.Ahead, st.Behind, developName)
			}

			if op, found, opErr := git.InProgressOperation(); opErr == nil && found {
				ui.Warn("a %s is in progress — resolve or abort it before running gix commands", op)
			}

			if !health.OK() {
				for _, mb := range health.MissingBases {
					ui.Warn("configured %s branch %q does not exist in this repository", mb.Key, mb.Name)
				}
				ui.Dim("  if it was renamed, update .gix/config (or `gix config set`) to match")
			}

			return nil
		},
	}
}

// describeBranchRole prints the "the main branch" / "a feature branch,
// based on develop" / etc. line for a branch gix knows how to place in
// the flow, and falls back to an explicit "not part of the configured
// flow" for anything else — including a branch matching a configured
// base/parent key whose target doesn't actually exist in the
// repository, which is called out by name rather than silently
// treated the same as "unknown branch". health is used only for that
// fallback: if nothing else claims this branch AND a configured base
// branch is missing, the two facts together are worth surfacing side
// by side, since the most common way to end up here is a manual
// `git branch -m` renaming exactly the branch you're standing on.
func describeBranchRole(cfg *config.Config, branch, developName string, health flow.Health) {
	switch {
	case branch == cfg.MainName():
		ui.Dim("  the main branch")
	case branch == developName:
		ui.Dim("  the develop branch")
	default:
		if kind, spec, ok := findKindForBranch(cfg, branch); ok {
			parentName := resolveDisplayName(cfg, spec.Parent)
			if spec.Parent != "" && !git.BranchExists(parentName) {
				ui.Dim("  a %s branch — configured parent %q does not exist in this repository", kind, parentName)
			} else {
				ui.Dim("  a %s branch, based on %s", kind, parentName)
			}
			return
		}
		ui.Dim("  not part of the configured flow (no kind prefix matches)")
		if !health.OK() {
			ui.Dim("  note: %s", missingBaseHint(health))
		}
	}
}

func missingBaseHint(health flow.Health) string {
	if len(health.MissingBases) == 1 {
		mb := health.MissingBases[0]
		return "configured " + mb.Key + " branch (" + mb.Name + ") is missing — if this branch is what used to be " + mb.Name + ", update .gix/config to match"
	}
	return "one or more configured base branches are missing — see the warning below"
}

func findKindForBranch(cfg *config.Config, branch string) (string, config.BranchSpec, bool) {
	for _, kind := range cfg.Kinds() {
		spec, ok := cfg.Spec(kind)
		if !ok || spec.Prefix == "" {
			continue
		}
		if len(branch) > len(spec.Prefix) && branch[:len(spec.Prefix)] == spec.Prefix {
			return kind, spec, true
		}
	}
	return "", config.BranchSpec{}, false
}
