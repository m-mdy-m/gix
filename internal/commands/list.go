package commands

import (
	"fmt"

	"github.com/m-mdy-m/gix/internal/cli"
	"github.com/m-mdy-m/gix/internal/config"
	"github.com/m-mdy-m/gix/internal/flow"
	"github.com/m-mdy-m/gix/internal/git"
	"github.com/m-mdy-m/gix/internal/ui"
	"github.com/spf13/cobra"
)

func registerList(root *cobra.Command, ctx *cli.Context) {
	root.AddCommand(newListCmd(ctx))
}

func newListCmd(ctx *cli.Context) *cobra.Command {
	var tree bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List branches across every configured kind",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cfg, err := ctx.Config()
			if err != nil {
				return err
			}

			head, err := git.GetHeadState()
			if err != nil {
				return err
			}
			if head.Kind == git.HeadDetached {
				ui.Dim("(detached HEAD at %s)", head.Commit)
			}
			current := head.Branch // "" for detached HEAD, which matches no branch below — fine

			if tree {
				return renderTree(cfg, current)
			}
			return renderFlat(cfg, current)
		},
	}

	cmd.Flags().BoolVar(&tree, "tree", false, "render the flow as a git-flow-style tree instead of a flat list")

	return cmd
}

func renderFlat(cfg *config.Config, current string) error {
	any := false
	for _, kind := range cfg.Kinds() {
		names, err := flow.List(cfg, kind)
		if err != nil {
			return err
		}
		if len(names) == 0 {
			continue
		}
		any = true
		ui.Info("%s", kind)
		for _, name := range names {
			full, err := flow.FullName(cfg, kind, name)
			if err != nil {
				return err
			}
			marker := "  "
			if full == current {
				marker = "* "
			}
			ui.Dim("%s%s", marker, name)
		}
	}
	if !any {
		ui.Dim("no feature/bugfix/hotfix/release branches yet")
	}
	return nil
}

func renderTree(cfg *config.Config, current string) error {
	mainName := cfg.MainName()
	developName := cfg.DevelopName()
	health := flow.CheckHealth(cfg)
	missing := make(map[string]bool, len(health.MissingBases))
	for _, mb := range health.MissingBases {
		missing[mb.Name] = true
	}

	developNode := ui.TreeNode{Label: developName, Current: developName == current, Missing: missing[developName]}
	mainChildren := []ui.TreeNode{}

	for _, kind := range cfg.Kinds() {
		spec, ok := cfg.Spec(kind)
		if !ok {
			continue
		}
		names, err := flow.List(cfg, kind)
		if err != nil {
			return err
		}

		var children []ui.TreeNode
		for _, name := range names {
			full, err := flow.FullName(cfg, kind, name)
			if err != nil {
				return err
			}
			children = append(children, ui.TreeNode{Label: name, Current: full == current})
		}
		if len(children) == 0 {
			continue
		}

		group := ui.TreeNode{Label: kind + "/", Children: children}
		if spec.Parent == "develop" {
			developNode.Children = append(developNode.Children, group)
		} else {
			mainChildren = append(mainChildren, group)
		}
	}

	mainNode := ui.TreeNode{
		Label:    mainName,
		Current:  mainName == current,
		Missing:  missing[mainName],
		Children: append([]ui.TreeNode{developNode}, mainChildren...),
	}

	ui.Tree([]ui.TreeNode{mainNode})

	if !health.OK() {
		fmt.Println()
		for _, mb := range health.MissingBases {
			ui.Warn("configured %s branch %q does not exist in this repository", mb.Key, mb.Name)
		}
	}
	return nil
}
