package commands

import (
	"errors"
	"fmt"

	"github.com/m-mdy-m/gix/internal/cli"
	"github.com/m-mdy-m/gix/internal/flow"
	"github.com/m-mdy-m/gix/internal/git"
	"github.com/m-mdy-m/gix/internal/ui"
	"github.com/spf13/cobra"
)

// doctorCommands registers `gix doctor`: inspect the repository and report
// what gix needs to work, plus the safe fixes it can apply.
type doctorCommands struct{}

func (doctorCommands) Register(root *cobra.Command, ctx *cli.Context) {
	root.AddCommand(newDoctorCmd(ctx))
}

func newDoctorCmd(ctx *cli.Context) *cobra.Command {
	var fix bool
	var strict bool
	var scope string
	var setName, setEmail string

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check the repository for problems gix would run into",
		Long: `doctor inspects this repository and reports what it finds: whether
the flow is set up, whether HEAD and the working tree are in a state
gix can work with, whether a remote and upstream exist, whether a Git
identity is configured, how signing is set up, and whether the base
branches .gix/config names actually exist.

It only ever reports. Nothing is rewritten, no commits are touched, and
no history is rewritten.

--fix applies only what you explicitly asked for, which today means
setting the Git identity:

  gix doctor --fix --set-user-name "Your Name" --set-user-email "you@example.com"

--scope chooses where that identity is written: local (default, this
repository only) or global (every repository for this user). gix never
guesses a name or email, and never overwrites one you didn't ask it to.

Run --fix with no name or email and it prints the exact command for
each problem it found, without changing anything.

--strict makes the command exit non-zero when anything needs attention,
so CI can fail on it. Without it doctor always exits 0: it reports, and
deciding what counts as blocking is yours.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := ctx.Repo()
			if err != nil {
				return err
			}

			report := flow.CheckRepo(repoRoot)
			printReport(report)

			if fix {
				if err := applyFixes(scope, setName, setEmail, report); err != nil {
					return err
				}
			}

			if strict && !report.OK() {
				return errNeedsAttention
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&fix, "fix", false, "apply the fixes you asked for by name, or print the exact command for each problem")
	cmd.Flags().StringVar(&scope, "scope", git.ScopeLocal, "where to write the identity: local (this repository) or global (all repositories)")
	cmd.Flags().StringVar(&setName, "set-user-name", "", "set user.name (requires --set-user-email)")
	cmd.Flags().StringVar(&setEmail, "set-user-email", "", "set user.email (requires --set-user-name)")
	cmd.Flags().BoolVar(&strict, "strict", false, "exit non-zero when anything needs attention, for use in CI")

	return cmd
}

func applyFixes(scope, setName, setEmail string, report flow.Report) error {
	if scope != git.ScopeLocal && scope != git.ScopeGlobal {
		return fmt.Errorf("invalid --scope %q (want %s or %s)", scope, git.ScopeLocal, git.ScopeGlobal)
	}

	if setName == "" && setEmail == "" {
		printFixHints(report)
		return nil
	}
	if setName == "" || setEmail == "" {
		return fmt.Errorf("--set-user-name and --set-user-email must be given together")
	}

	if err := git.SetConfig("user.name", setName, scope); err != nil {
		return err
	}
	if err := git.SetConfig("user.email", setEmail, scope); err != nil {
		return err
	}

	ui.Success("identity set in %s config", scope)
	ui.Field("name", "%s", setName)
	ui.Field("email", "%s", setEmail)
	return nil
}

func printReport(report flow.Report) {
	for _, c := range report.Checks {
		ui.Check(c.Severity == flow.OK, "%s", c.Name)

		if c.Value != "" {
			ui.Field("value", "%s", c.Value)
		}
		if c.Problem != "" {
			ui.Dim("  %s", c.Problem)
		}
	}

	problems := report.Count(flow.Fail) + report.Count(flow.Warn)
	if problems == 0 {
		ui.Info("no problems found")
		return
	}

	ui.Dim("")
	ui.Dim("%d of %d checks need attention", problems, len(report.Checks))
}

// printFixHints lists what could be fixed and the exact command to do it.
// It writes nothing, so it is safe to suggest for anything gix can't
// safely repair on its own.
// errNeedsAttention is what --strict returns, so the exit code carries the
// verdict while the report itself still prints in full.
var errNeedsAttention = errors.New("repository needs attention — see the report above")

func printFixHints(report flow.Report) {
	hinted := false

	for _, c := range report.Problems() {
		if c.Fix == "" {
			continue
		}
		if !hinted {
			ui.Info("how to fix each one")
			hinted = true
		}
		ui.Step("%s: %s", c.Name, c.Fix)
	}

	if !hinted {
		ui.Dim("nothing gix can fix automatically — see the problems above")
		return
	}
	ui.Dim("")
	ui.Dim("gix never guesses your name or email, and repairs nothing without being asked")
}
