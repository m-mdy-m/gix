package commands

import (
	"fmt"

	"github.com/m-mdy-m/gix/internal/cli"
	"github.com/m-mdy-m/gix/internal/git"
	"github.com/m-mdy-m/gix/internal/ui"
	"github.com/spf13/cobra"
)

func registerInit(root *cobra.Command, ctx *cli.Context) {
	var initialBranch string
	var noCommit bool
	var authorName, authorEmail string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a new repository (like `git init`), ready for `gix flow init`",
		Long: `init gets a project to the point where gix flow init can run.

If the current directory is not yet a Git repository, init creates one
(equivalent to 'git init', or 'git init -b <name>' when --initial-branch
is given).

If the repository exists but has no commits yet — a brand new
repository, or one created with plain 'git init' — 'gix flow init'
has no base branch to build the flow on top of. init covers this case
too, by creating an empty initial commit, so you don't need to drop
down to 'git commit --allow-empty' yourself.

Creating that commit requires a Git author identity (user.name/
user.email). If none is configured, init still creates the repository
but leaves it commit-less and tells you how to finish: configure your
identity and re-run 'gix init', or pass --author-name/--author-email
for one-off use, or pass --no-commit to only create the repository.

init is safe to re-run: on an existing repository with commits already,
it does nothing and reports the repository is already usable.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if (authorName == "") != (authorEmail == "") {
				return fmt.Errorf("--author-name and --author-email must be given together")
			}

			createdRepo := false
			if !git.IsRepo() {
				if err := git.InitRepo(initialBranch); err != nil {
					return fmt.Errorf("initializing repository: %w", err)
				}
				createdRepo = true
			} else if initialBranch != "" {
				ui.Warn("repository already exists, ignoring --initial-branch")
			}

			repoRoot, err := ctx.Repo()
			if err != nil {
				return err
			}

			switch {
			case createdRepo:
				ui.Success("initialized empty Git repository")
				ui.Field("path", "%s", repoRoot)
			default:
				ui.Info("repository already initialized")
				ui.Field("path", "%s", repoRoot)
			}

			if git.HasCommits() {
				ui.Dim("  already has commits, nothing more to do")
				ui.Info("next: gix flow init")
				return nil
			}

			if noCommit {
				ui.Warn("no commits yet (--no-commit was passed)")
				ui.Dim("  create one yourself, e.g.:")
				ui.Dim("    git commit --allow-empty -m \"initial commit\"")
				ui.Dim("  or re-run `gix init`")
				return nil
			}

			haveOverride := authorName != "" && authorEmail != ""
			identity := git.CurrentIdentity()
			if !identity.Configured() && !haveOverride {
				ui.Warn("no commits yet, and no git author identity is configured")
				ui.Dim("  configure your identity, then run:")
				ui.Dim("    git commit --allow-empty -m \"initial commit\"")
				ui.Dim("  or re-run `gix init`")
				ui.Dim("  (git config --global user.name \"...\" && git config --global user.email \"...\")")
				return nil
			}

			var env []string
			if haveOverride {
				env = []string{
					"GIT_AUTHOR_NAME=" + authorName,
					"GIT_AUTHOR_EMAIL=" + authorEmail,
					"GIT_COMMITTER_NAME=" + authorName,
					"GIT_COMMITTER_EMAIL=" + authorEmail,
				}
			}
			if err := git.EmptyCommit("initial commit", env); err != nil {
				return fmt.Errorf("creating initial commit: %w", err)
			}
			ui.Step("created initial commit")
			ui.Info("next: gix flow init")
			return nil
		},
	}

	cmd.Flags().StringVar(&initialBranch, "initial-branch", "", "name of the initial branch when creating a new repository (passed to git init -b)")
	cmd.Flags().BoolVar(&noCommit, "no-commit", false, "only create the repository, don't create an initial commit")
	cmd.Flags().StringVar(&authorName, "author-name", "", "author name for the initial commit, for this run only (requires --author-email)")
	cmd.Flags().StringVar(&authorEmail, "author-email", "", "author email for the initial commit, for this run only (requires --author-name)")

	root.AddCommand(cmd)
}
