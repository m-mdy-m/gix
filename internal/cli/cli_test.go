package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/m-mdy-m/gix/internal/logger"
	"github.com/spf13/cobra"
)

func newRoot() *cobra.Command {
	return &cobra.Command{
		Use:           "gix",
		RunE:          func(*cobra.Command, []string) error { return nil },
		SilenceUsage:  true,
		SilenceErrors: true,
	}
}

func TestBindGlobalFlags(t *testing.T) {
	root := newRoot()

	flags := BindGlobalFlags(root)

	if flags.Verbose || flags.Debug || flags.NoColor {
		t.Errorf("flags default to on: %+v, want all off", flags)
	}
	for _, name := range []string{"verbose", "debug", "no-color"} {
		if root.PersistentFlags().Lookup(name) == nil {
			t.Errorf("flag --%s is not registered", name)
		}
	}
	if root.PersistentPreRunE == nil {
		t.Fatal("PersistentPreRunE is nil, want the global flags to take effect")
	}
}

func TestGlobalPreRunSetsLogLevel(t *testing.T) {
	tests := []struct {
		name  string
		flags []string
		want  logger.Level
	}{
		{name: "default stays silent", flags: nil, want: logger.LevelSilent},
		{name: "verbose", flags: []string{"--verbose"}, want: logger.LevelVerbose},
		{name: "debug", flags: []string{"--debug"}, want: logger.LevelDebug},
		{name: "debug wins over verbose", flags: []string{"--verbose", "--debug"}, want: logger.LevelDebug},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger.SetLevel(logger.LevelSilent)
			t.Cleanup(func() { logger.SetLevel(logger.LevelSilent) })

			root := newRoot()
			BindGlobalFlags(root)
			root.SetArgs(tc.flags)
			if err := root.Execute(); err != nil {
				t.Fatalf("Execute: %v", err)
			}

			if got := logger.CurrentLevel(); got != tc.want {
				t.Errorf("log level = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSharedFlags(t *testing.T) {
	root := newRoot()

	var noDelete bool
	var strategy, tag string

	WithNoDelete(root, &noDelete)
	WithStrategy(root, &strategy)
	WithTag(root, &tag)

	for _, name := range []string{"no-delete", "strategy", "tag"} {
		if root.Flags().Lookup(name) == nil {
			t.Errorf("shared flag --%s is not registered", name)
		}
	}

	if err := root.Flags().Set("strategy", "squash"); err != nil {
		t.Fatalf("setting --strategy: %v", err)
	}
	if strategy != "squash" {
		t.Errorf("strategy = %q, want the flag to write through to %q", strategy, "squash")
	}
}

func TestRegistrableFunc(t *testing.T) {
	var called bool

	var r Registrable = RegistrableFunc(func(*cobra.Command, *Context) { called = true })
	r.Register(newRoot(), &Context{})

	if !called {
		t.Error("RegistrableFunc.Register did not call the function")
	}
}

func TestNotARepoError(t *testing.T) {
	t.Chdir(t.TempDir())

	var ctx Context
	_, err := ctx.Repo()

	if err == nil {
		t.Fatal("Repo() = nil error outside a repository")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("Repo() = %q, want the notARepo message", err)
	}
}

func TestLoggerRespectsLevel(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(&buf)

	log.Verbosef("hidden")
	if buf.Len() != 0 {
		t.Errorf("a silent logger wrote %q", buf.String())
	}

	log.SetLevel(logger.LevelVerbose)
	log.Verbosef("shown")
	if !strings.Contains(buf.String(), "shown") {
		t.Errorf("verbose output = %q, want it to contain the message", buf.String())
	}

	buf.Reset()
	log.Debugf("hidden at verbose")
	if buf.Len() != 0 {
		t.Errorf("a verbose logger wrote debug output: %q", buf.String())
	}
}

func TestLoggerPrefix(t *testing.T) {
	var buf bytes.Buffer

	log := logger.New(&buf)
	log.SetLevel(logger.LevelVerbose)

	prefixed := log.WithPrefix("git")
	prefixed.Verbosef("checkout main")

	if !strings.Contains(buf.String(), "[git]") {
		t.Errorf("prefixed output = %q, want it tagged with the prefix", buf.String())
	}
	if !strings.Contains(buf.String(), "checkout main") {
		t.Errorf("prefixed output = %q, want it to contain the message", buf.String())
	}
}
