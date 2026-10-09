package flow

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/m-mdy-m/gix/internal/git"
	"github.com/m-mdy-m/gix/internal/testutil"
)

func TestInspectSigningDefaults(t *testing.T) {
	signing := InspectSigning(git.Config{})

	// Git's own default is openpgp even when gpg.format is unset.
	if signing.Format != SigningOpenPGP {
		t.Errorf("Format = %q, want %q for an unset gpg.format", signing.Format, SigningOpenPGP)
	}
	if signing.Key != "" {
		t.Errorf("Key = %q, want empty when user.signingkey is unset", signing.Key)
	}
	if signing.SigningEnabled() {
		t.Error("SigningEnabled() = true with commit.gpgsign unset")
	}
	if signing.TagSign {
		t.Error("TagSign = true with tag.gpgsign unset")
	}
}

func TestInspectSigningReadsConfig(t *testing.T) {
	cfg := git.Config{
		"gpg.format":      {Value: SigningSSH, Scope: git.ScopeLocal},
		"user.signingkey": {Value: "~/.ssh/id_ed25519.pub", Scope: git.ScopeLocal},
		"commit.gpgsign":  {Value: "true", Scope: git.ScopeLocal},
		"tag.gpgsign":     {Value: "true", Scope: git.ScopeLocal},
	}

	signing := InspectSigning(cfg)

	if signing.Format != SigningSSH {
		t.Errorf("Format = %q, want %q", signing.Format, SigningSSH)
	}
	if signing.Key != "~/.ssh/id_ed25519.pub" {
		t.Errorf("Key = %q, want the configured key path", signing.Key)
	}
	if signing.KeyScope != git.ScopeLocal {
		t.Errorf("KeyScope = %q, want %q", signing.KeyScope, git.ScopeLocal)
	}
	if !signing.SigningEnabled() {
		t.Error("SigningEnabled() = false with commit.gpgsign = true")
	}
	if !signing.TagSign {
		t.Error("TagSign = false with tag.gpgsign = true")
	}
}

// Git accepts several spellings of true for its booleans; gix must agree
// with git rather than only understanding "true".
func TestInspectSigningBooleanSpellings(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{value: "true", want: true},
		{value: "yes", want: true},
		{value: "on", want: true},
		{value: "1", want: true},
		{value: "false", want: false},
		{value: "no", want: false},
		{value: "", want: false},
		{value: "nonsense", want: false},
	}

	for _, tc := range tests {
		t.Run("commit.gpgsign="+tc.value, func(t *testing.T) {
			cfg := git.Config{"commit.gpgsign": {Value: tc.value, Scope: git.ScopeLocal}}

			if got := InspectSigning(cfg).SigningEnabled(); got != tc.want {
				t.Errorf("SigningEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestInspectSigningX509NeedsNoProgram(t *testing.T) {
	cfg := git.Config{
		"gpg.format": {Value: SigningX509, Scope: git.ScopeLocal},
	}

	signing := InspectSigning(cfg)

	if signing.Program != "" {
		t.Errorf("Program = %q, want empty for x509, which git handles itself", signing.Program)
	}
	if !signing.Available {
		t.Error("Available = false for x509, which needs no external program")
	}
}

func TestInspectSigningOpenPGPUsesConfiguredProgram(t *testing.T) {
	cfg := git.Config{
		"user.signingkey": {Value: "KEY", Scope: git.ScopeLocal},
		"gpg.program":     {Value: "gix-custom-signer", Scope: git.ScopeLocal},
	}

	signing := InspectSigning(cfg)

	if signing.Program != "gix-custom-signer" {
		t.Errorf("Program = %q, want the configured gpg.program", signing.Program)
	}
	if signing.Available {
		t.Error("Available = true for a program that doesn't exist")
	}
}

func TestInspectSigningOpenPGPDefaultsToGpg(t *testing.T) {
	signing := InspectSigning(git.Config{
		"gpg.format": {Value: SigningOpenPGP, Scope: git.ScopeLocal},
	})

	if signing.Program != "gpg" {
		t.Errorf("Program = %q, want gpg by default", signing.Program)
	}
	// Availability depends on the machine, so compare against git's own view.
	if _, err := exec.LookPath("gpg"); err != nil && signing.Available {
		t.Error("Available = true but gpg is not on PATH")
	}
}

func TestInspectSigningUnknownFormat(t *testing.T) {
	cfg := git.Config{
		"gpg.format":     {Value: "carrier-pigeon", Scope: git.ScopeLocal},
		"commit.gpgsign": {Value: "true", Scope: git.ScopeLocal},
	}

	signing := InspectSigning(cfg)

	if signing.Available {
		t.Error("Available = true for a format gix doesn't know")
	}
	if signing.Program != "" {
		t.Errorf("Program = %q, want empty for an unknown format", signing.Program)
	}
}

func TestInspectSigningFromARepo(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Run("config", "gpg.format", "ssh")
	repo.Run("config", "user.signingkey", "~/.ssh/id_ed25519.pub")
	repo.Run("config", "commit.gpgsign", "true")

	cfg, err := git.ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}

	signing := InspectSigning(cfg)

	if signing.Format != SigningSSH {
		t.Errorf("Format = %q, want %q", signing.Format, SigningSSH)
	}
	if !signing.SigningEnabled() {
		t.Error("SigningEnabled() = false, want the repo's commit.gpgsign to be seen")
	}
	// ssh signing needs ssh-keygen, which git also uses for transport.
	if !strings.Contains(signing.Program, "ssh-keygen") {
		t.Errorf("Program = %q, want ssh-keygen", signing.Program)
	}
	if signing.KeyScope != git.ScopeLocal {
		t.Errorf("KeyScope = %q, want %q", signing.KeyScope, git.ScopeLocal)
	}
}

// Whatever the setup, gix must never surface key material — only the
// identifier git itself stores.
func TestInspectSigningNeverExposesPrivateKeys(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Run("config", "gpg.format", "openpgp")
	repo.Run("config", "user.signingkey", "5B8CC34F0337D366")

	cfg, err := git.ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}

	signing := InspectSigning(cfg)

	if strings.Contains(signing.Key, "PRIVATE KEY") {
		t.Errorf("Key = %q, want only a key id", signing.Key)
	}
	if signing.Key != "5B8CC34F0337D366" {
		t.Errorf("Key = %q, want the key id exactly as configured", signing.Key)
	}
}
