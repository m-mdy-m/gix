package commands

import (
	"strings"
	"testing"

	"github.com/m-mdy-m/gix/internal/testutil"
)

func TestDoctorOnAFreshRepo(t *testing.T) {
	repo := testutil.NewRepo(t)

	out := run(t, repo, "doctor")

	// Every check has to be named, so a user can tell what was inspected.
	for _, want := range []string{
		"repository", "gix config", "HEAD", "working tree",
		"in-progress operation", "identity", "remote", "upstream",
		"signing", "flow",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("`gix doctor` output is missing the %q check:\n%s", want, out)
		}
	}
}

// doctor has to work before gix is set up — finding out that you haven't
// run `gix flow init` is one of the main reasons to run it.
func TestDoctorWithoutGixConfig(t *testing.T) {
	repo := testutil.NewRepo(t)

	out := run(t, repo, "doctor")

	if !strings.Contains(out, "gix is not initialized") {
		t.Errorf("`gix doctor` should notice gix isn't set up:\n%s", out)
	}
	if !strings.Contains(out, "gix flow init") {
		t.Errorf("`gix doctor` should say how to fix it:\n%s", out)
	}
	if !strings.Contains(out, "repository") {
		t.Errorf("`gix doctor` should still check the repository itself:\n%s", out)
	}
}

func TestDoctorOnAHealthyRepo(t *testing.T) {
	repo := gixRepo(t)
	// flow init leaves HEAD on develop, so that's the branch that needs
	// an upstream for the check to be satisfied.
	repo.Run("remote", "add", "origin", "https://example.invalid/gix.git")
	repo.Run("update-ref", "refs/remotes/origin/develop", "develop")
	repo.Run("config", "branch.develop.remote", "origin")
	repo.Run("config", "branch.develop.merge", "refs/heads/develop")

	out := run(t, repo, "doctor")

	if !strings.Contains(out, "no problems found") {
		t.Errorf("`gix doctor` on a healthy repo should say so:\n%s", out)
	}
}

func TestDoctorReportsDirtyTree(t *testing.T) {
	repo := gixRepo(t)
	repo.Write("wip.txt", "wip\n")

	out := run(t, repo, "doctor")

	if !strings.Contains(out, "uncommitted changes") {
		t.Errorf("`gix doctor` should report the dirty tree:\n%s", out)
	}
	if !strings.Contains(out, "checks need attention") {
		t.Errorf("`gix doctor` should summarize what needs attention:\n%s", out)
	}
}

func TestDoctorReportsMissingIdentity(t *testing.T) {
	repo := gixRepo(t)
	repo.Run("config", "--unset", "user.name")
	repo.Run("config", "--unset", "user.email")

	out := run(t, repo, "doctor")

	if !strings.Contains(out, "user.name and user.email are not set") {
		t.Errorf("`gix doctor` should report the missing identity:\n%s", out)
	}
}

func TestDoctorFixWithoutFlagsOnlyPrints(t *testing.T) {
	repo := gixRepo(t)
	repo.Run("config", "--unset", "user.name")
	repo.Run("config", "--unset", "user.email")

	before := repo.Output("config", "--list")
	out := run(t, repo, "doctor", "--fix")

	if before != repo.Output("config", "--list") {
		t.Error("`gix doctor --fix` changed config even with no identity flags given")
	}
	if !strings.Contains(out, "how to fix each one") {
		t.Errorf("`gix doctor --fix` should print how to fix each problem:\n%s", out)
	}
	if !strings.Contains(out, "--set-user-name") {
		t.Errorf("`gix doctor --fix` should give the exact command:\n%s", out)
	}
}

func TestDoctorFixSetsIdentityLocally(t *testing.T) {
	repo := gixRepo(t)
	repo.Run("config", "--unset", "user.name")
	repo.Run("config", "--unset", "user.email")

	out := run(t, repo, "doctor", "--fix", "--set-user-name=Test User", "--set-user-email=test@example.invalid")

	if !strings.Contains(out, "identity set in local config") {
		t.Errorf("`gix doctor --fix` should confirm what it wrote:\n%s", out)
	}
	if got := repo.Output("config", "--local", "user.name"); got != "Test User" {
		t.Errorf("local user.name = %q, want %q", got, "Test User")
	}
	if got := repo.Output("config", "--local", "user.email"); got != "test@example.invalid" {
		t.Errorf("local user.email = %q, want it written locally", got)
	}

	// And the problem must be gone afterwards.
	if after := run(t, repo, "doctor"); strings.Contains(after, "user.name and user.email are not set") {
		t.Errorf("`gix doctor` still reports the identity problem after --fix:\n%s", after)
	}
}

func TestDoctorFixScopeGlobal(t *testing.T) {
	repo := gixRepo(t)
	repo.Run("config", "--unset", "user.name")
	repo.Run("config", "--unset", "user.email")

	out := run(t, repo, "doctor", "--fix", "--scope=global", "--set-user-name=Global User", "--set-user-email=global@example.invalid")

	if !strings.Contains(out, "identity set in global config") {
		t.Errorf("`gix doctor --fix --scope=global` should say where it wrote:\n%s", out)
	}
	if _, err := repo.Try("config", "--local", "user.name"); err == nil {
		t.Error("--scope=global wrote a local value as well")
	}
}

func TestDoctorFixRequiresBothIdentityFlags(t *testing.T) {
	repo := gixRepo(t)
	repo.Run("config", "--unset", "user.name")
	repo.Run("config", "--unset", "user.email")

	runFailsWith(t, repo, "must be given together", "doctor", "--fix", "--set-user-name=Only Name")
	runFailsWith(t, repo, "must be given together", "doctor", "--fix", "--set-user-email=only@example.invalid")
}

// --fix must never invent an identity or write anything it wasn't told to.
func TestDoctorFixWritesNothingElse(t *testing.T) {
	repo := gixRepo(t)
	repo.Run("config", "--unset", "user.name")
	repo.Run("config", "--unset", "user.email")

	before := repo.Output("config", "--list")
	run(t, repo, "doctor", "--fix", "--set-user-name=Test User", "--set-user-email=test@example.invalid")

	after := strings.Split(repo.Output("config", "--list"), "\n")
	want := strings.Split(before, "\n")
	for _, line := range want {
		if line == "" {
			continue
		}
		if !containsLine(after, line) {
			t.Errorf("`gix doctor --fix` removed or changed %q", line)
		}
	}
	if len(after) != len(want)+2 {
		t.Errorf("config has %d keys, want only the 2 identity keys added (%d before)", len(after), len(want))
	}
}

func TestDoctorFixRejectsUnknownScope(t *testing.T) {
	repo := gixRepo(t)

	runFailsWith(t, repo, "invalid --scope", "doctor", "--fix", "--scope=system",
		"--set-user-name=Test User", "--set-user-email=test@example.invalid")
}

func TestDoctorRejectsExtraArgs(t *testing.T) {
	repo := gixRepo(t)

	runFails(t, repo, "doctor", "unexpected")
}

func TestSigningStatus(t *testing.T) {
	repo := gixRepo(t)

	out := run(t, repo, "signing", "status")

	for _, want := range []string{"signing", "format", "openpgp", "commits", "tags", "no signing key configured"} {
		if !strings.Contains(out, want) {
			t.Errorf("`gix signing status` output is missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "signing is off") {
		t.Errorf("`gix signing status` should say signing is off by default:\n%s", out)
	}
}

func TestSigningStatusConfigured(t *testing.T) {
	repo := gixRepo(t)
	repo.Run("config", "gpg.format", "ssh")
	repo.Run("config", "user.signingkey", "~/.ssh/id_ed25519.pub")
	repo.Run("config", "commit.gpgsign", "true")

	out := run(t, repo, "signing", "status")

	if !strings.Contains(out, "ssh") {
		t.Errorf("`gix signing status` should report the ssh format:\n%s", out)
	}
	if !strings.Contains(out, "~/.ssh/id_ed25519.pub") {
		t.Errorf("`gix signing status` should report the configured key:\n%s", out)
	}
	if !strings.Contains(out, "ssh-keygen") {
		t.Errorf("`gix signing status` should name the program git needs:\n%s", out)
	}
	if !strings.Contains(out, "local config") {
		t.Errorf("`gix signing status` should say where the key is configured:\n%s", out)
	}
}

func TestSigningStatusUnavailableProgram(t *testing.T) {
	repo := gixRepo(t)
	repo.Run("config", "gpg.format", "openpgp")
	repo.Run("config", "gpg.program", "gix-nonexistent-signer")
	repo.Run("config", "user.signingkey", "ABCDEF0123456789")
	repo.Run("config", "commit.gpgsign", "true")

	out := run(t, repo, "signing", "status")

	if !strings.Contains(out, "git cannot sign") {
		t.Errorf("`gix signing status` should warn that git cannot sign:\n%s", out)
	}
	if !strings.Contains(out, "gix-nonexistent-signer") {
		t.Errorf("`gix signing status` should name the missing program:\n%s", out)
	}
}

func TestSigningStatusX509(t *testing.T) {
	repo := gixRepo(t)
	repo.Run("config", "gpg.format", "x509")

	out := run(t, repo, "signing", "status")

	if !strings.Contains(out, "built into git") {
		t.Errorf("`gix signing status` should say x509 needs no program:\n%s", out)
	}
}

// Neither command may write anything: both are meant to be safe to run
// while you work out what's wrong.
func TestDoctorAndSigningAreReadOnly(t *testing.T) {
	repo := gixRepo(t)

	before := repo.Output("config", "--list")
	head := repo.Output("rev-parse", "HEAD")

	run(t, repo, "doctor")
	run(t, repo, "signing", "status")

	if after := repo.Output("config", "--list"); after != before {
		t.Error("inspecting changed git config")
	}
	if after := repo.Output("rev-parse", "HEAD"); after != head {
		t.Error("inspecting moved HEAD")
	}
	if !repo.Clean() {
		t.Error("inspecting left the working tree dirty")
	}
}

func TestDoctorAndSigningNeedARepo(t *testing.T) {
	testutil.NewDir(t)

	runFailsWith(t, nil, "not a git repository", "doctor")
	runFailsWith(t, nil, "not a git repository", "signing", "status")
}

func TestNewCommandsAreRegistered(t *testing.T) {
	root := New()

	have := map[string]bool{}
	for _, c := range root.Commands() {
		have[c.Name()] = true
		if c.Short == "" {
			t.Errorf("command %q has no Short description", c.Name())
		}
	}
	for _, name := range []string{"doctor", "signing"} {
		if !have[name] {
			t.Errorf("command %q is not registered", name)
		}
	}
}

func TestNewCommandsHaveHelp(t *testing.T) {
	repo := testutil.NewRepo(t)

	for _, args := range [][]string{
		{"doctor", "--help"},
		{"signing", "--help"},
		{"signing", "status", "--help"},
	} {
		run(t, repo, args...)
	}
}

func containsLine(lines []string, line string) bool {
	for _, l := range lines {
		if l == line {
			return true
		}
	}
	return false
}
