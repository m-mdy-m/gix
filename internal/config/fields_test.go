package config

import (
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func marshalSpec(spec BranchSpec) (string, error) {
	out, err := yaml.Marshal(spec)
	return string(out), err
}

// Every key in the default config should be one gix can explain, and a
// user setting any of them should not hit a dead end. This is a
// documentation check as much as a code one: a field nobody reads is a
// field that lies in `gix config list`.
func TestDefaultConfigFieldsAreAllKnown(t *testing.T) {
	known := map[string]bool{
		"name": true, "type": true, "prefix": true, "parent": true,
		"merge_into": true, "upstream_strategy": true, "downstream_strategy": true,
		"tag": true, "autosync": true, "delete_on_finish": true,
	}

	for kind, spec := range Default().Branch {
		// Struct fields with no yaml tag marshal under their Go name, so
		// walk the marshalled form rather than the struct.
		raw, err := marshalSpec(spec)
		if err != nil {
			t.Fatalf("marshalling %s: %v", kind, err)
		}
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimSpace(line)
			key, _, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			if !known[key] {
				t.Errorf("branch.%s writes the field %q, which gix does not read anywhere", kind, key)
			}
		}
	}
}

// The top-level keys have to be read somewhere too, not just written.
func TestTopLevelFieldsAreAllRead(t *testing.T) {
	cfg := Default()

	if cfg.MainName() == "" || cfg.DevelopName() == "" {
		t.Error("the base branch names are not resolvable")
	}
	if len(cfg.Kinds()) == 0 {
		t.Error("no topic kinds resolve")
	}
	if len(cfg.BaseKeys()) == 0 {
		t.Error("no base keys resolve")
	}
	if cfg.Remote == "" {
		t.Error("the remote name is empty, but doctor reads it")
	}
}

// A user setting any documented field must not make the config
// unloadable.
func TestEveryDocumentedFieldIsAccepted(t *testing.T) {
	cfg := Default()

	spec := cfg.Branch["feature"]
	spec.Name = ""
	spec.Prefix = "feat/"
	spec.Parent = "main"
	spec.MergeInto = []string{"main", "develop"}
	spec.UpstreamStrategy = StrategyRebase
	spec.DownstreamStrategy = StrategySquash
	spec.Tag = true
	spec.AutoSync = true
	spec.DeleteOnFinish = false
	cfg.Branch["feature"] = spec

	if err := Validate(cfg); err != nil {
		t.Errorf("a config using every documented field failed validation: %v", err)
	}
}
