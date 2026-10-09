package flow

import (
	"os/exec"
	"runtime"

	"github.com/m-mdy-m/gix/internal/git"
)

// Signing formats git supports. gix only reads these out of the
// configuration and reports on them — it never signs anything itself.
const (
	SigningOpenPGP = "openpgp"
	SigningSSH     = "ssh"
	SigningX509    = "x509"
)

// SigningInfo describes how this repository is set up to sign, as far as
// configuration and the environment can tell. Nothing here exposes key
// material: only the key's identifier, exactly as git stores it.
type SigningInfo struct {
	Format     string // openpgp, ssh, x509, or "" when unset
	Key        string // user.signingkey, git's key id or public key path
	KeyScope   string // scope user.signingkey came from
	CommitSign bool   // commit.gpgsign
	TagSign    bool   // tag.gpgsign
	Program    string // external program the format needs, if any
	Available  bool   // that program is on PATH
}

// SigningEnabled reports whether git is configured to sign commits.
func (s SigningInfo) SigningEnabled() bool { return s.CommitSign }

// InspectSigning reads signing configuration from an already-read config
// snapshot. The format defaults to openpgp in git, so an unset
// gpg.format is reported as openpgp rather than "unknown".
func InspectSigning(cfg git.Config) SigningInfo {
	formatEntry, ok := cfg.Get("gpg.format")
	format := SigningOpenPGP
	if ok && formatEntry.Value != "" {
		format = formatEntry.Value
	}

	keyEntry, _ := cfg.Get("user.signingkey")

	info := SigningInfo{
		Format:     format,
		Key:        keyEntry.Value,
		KeyScope:   keyEntry.Scope,
		CommitSign: truthy(cfg.ConfigValue("commit.gpgsign")),
		TagSign:    truthy(cfg.ConfigValue("tag.gpgsign")),
	}

	info.Program, info.Available = signingProgram(format, cfg)
	return info
}

// signingProgram returns the executable a format needs and whether it can
// be found. x509 signing is built into git, so it needs nothing on PATH.
func signingProgram(format string, cfg git.Config) (string, bool) {
	switch format {
	case SigningOpenPGP:
		program := cfg.ConfigValue("gpg.program")
		if program == "" {
			program = "gpg"
		}
		return program, onPath(program)
	case SigningSSH:
		// git signs with `ssh-keygen -Y sign`, which it also uses for
		// transport; on Windows that ships as ssh-keygen.exe.
		name := "ssh-keygen"
		if runtime.GOOS == "windows" {
			name = "ssh-keygen.exe"
		}
		return name, onPath(name)
	case SigningX509:
		return "", true
	default:
		return "", false
	}
}

func onPath(program string) bool {
	_, err := exec.LookPath(program)
	return err == nil
}

// truthy reads a Git boolean, which treats any spelling of "true" as on
// and everything else — including an unset key — as off.
func truthy(v string) bool {
	switch v {
	case "true", "yes", "on", "1":
		return true
	default:
		return false
	}
}
