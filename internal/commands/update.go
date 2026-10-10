package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/m-mdy-m/gix/internal/selfupdate"
	"github.com/m-mdy-m/gix/internal/ui"
	"github.com/spf13/cobra"
)

// newUpdater and executablePath are variables so tests can point the
// command at a fake release server and a throwaway binary.
var (
	newUpdater     = func() *selfupdate.Updater { return selfupdate.New("gix/" + Version) }
	executablePath = resolveExecutable
)

type updateRequest struct {
	check bool
	force bool
	tag   string
}

func newUpdateCmd() *cobra.Command {
	var req updateRequest

	cmd := &cobra.Command{
		Use:     "update",
		Aliases: []string{"upgrade"},
		Short:   "Update gix to the latest release",
		Long: `update replaces this gix with the latest published release.

It downloads the binary for this platform from the project's GitHub
releases, checks it against the published SHA-256, runs it once to make
sure it is a working gix, and only then swaps it in for the one you are
running. If anything fails, the gix you have is left exactly as it was.

It needs no repository and does not touch your repositories or
.gix/config. It never runs on its own: gix does not check for updates
in the background.

  gix update              install the latest release
  gix update --check      only say whether a newer release exists
  gix update --to v1.0.1  install a specific release, older or newer
  gix update --force      reinstall, or replace a development build

A gix built from source (make install, go install) is a development
build, not a release, and is left alone unless you pass --force.

If gix is installed somewhere your user cannot write, such as
/usr/local/bin, run it with the permissions that folder needs
(sudo gix update).`,
		Example: `  gix update --check
  gix update`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			return runUpdate(ctx, req)
		},
	}

	cmd.Flags().BoolVar(&req.check, "check", false, "only report whether a newer release exists; install nothing")
	cmd.Flags().BoolVar(&req.force, "force", false, "install even when up to date, and replace a development build")
	cmd.Flags().StringVar(&req.tag, "to", "", "install this release instead of the latest, such as v1.0.1")

	return cmd
}

func runUpdate(ctx context.Context, req updateRequest) error {
	exe, err := executablePath()
	if err != nil {
		return fmt.Errorf("cannot tell where gix is installed: %w", err)
	}
	selfupdate.CleanupOld(exe)

	res, err := newUpdater().Run(ctx, selfupdate.Options{
		Current:   Version,
		Tag:       req.tag,
		Force:     req.force,
		CheckOnly: req.check,
		ExePath:   exe,
	})
	if err != nil {
		return err
	}

	switch res.Action {
	case selfupdate.UpToDate:
		ui.Success("gix %s is up to date", res.Version)
	case selfupdate.Available:
		ui.Info("update available: %s -> %s", describeCurrent(res.Current), res.Version)
		ui.Dim("  run `gix update` to install it")
	case selfupdate.DevBuild:
		ui.Warn("this gix is a development build (%s), not a release", Version)
		ui.Info("latest release: %s", res.Version)
		ui.Dim("  run `gix update --force` to replace it with the release")
	case selfupdate.Updated:
		ui.Success("updated gix %s -> %s", describeCurrent(res.Current), res.Version)
		ui.Field("installed", "%s", exe)
	}
	return nil
}

func describeCurrent(current string) string {
	if current == "" {
		return "a development build"
	}
	return current
}

// resolveExecutable is the file to replace: the running binary, with any
// symlink followed, so updating through a link updates the real file.
func resolveExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	return exe, nil
}
