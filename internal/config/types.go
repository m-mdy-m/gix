package config

type Strategy string

const (
	StrategyMerge Strategy = "merge"

	StrategyRebase Strategy = "rebase"

	StrategySquash Strategy = "squash"
)

func (s Strategy) Valid() bool {
	switch s {
	case StrategyMerge, StrategyRebase, StrategySquash:
		return true
	default:
		return false
	}
}

type Role string

const (
	RoleBase  Role = "base"
	RoleTopic Role = "topic"
)

type BranchSpec struct {
	Name string `yaml:"name,omitempty"`

	Role Role `yaml:"type"`

	Prefix string `yaml:"prefix,omitempty"`

	Parent string `yaml:"parent,omitempty"`

	MergeInto []string `yaml:"merge_into,omitempty"`

	UpstreamStrategy Strategy `yaml:"upstream_strategy,omitempty"`

	DownstreamStrategy Strategy `yaml:"downstream_strategy,omitempty"`

	Tag bool `yaml:"tag,omitempty"`

	AutoSync bool `yaml:"autosync,omitempty"`

	DeleteOnFinish bool `yaml:"delete_on_finish"`
}

type Config struct {
	Gix    GixMeta               `yaml:"gix"`
	Remote string                `yaml:"remote,omitempty"`
	Branch map[string]BranchSpec `yaml:"branch"`
}

type GixMeta struct {
	Version string `yaml:"version"`
}

func (c *Config) MainName() string { return c.baseName("main", "main") }

func (c *Config) DevelopName() string { return c.baseName("develop", "develop") }

func (c *Config) baseName(key, fallback string) string {
	if c == nil || c.Branch == nil {
		return fallback
	}
	if spec, ok := c.Branch[key]; ok && spec.Name != "" {
		return spec.Name
	}
	return fallback
}

func (c *Config) Kinds() []string {
	if c == nil {
		return nil
	}
	wellKnown := []string{"feature", "bugfix", "hotfix", "release"}
	seen := make(map[string]bool, len(wellKnown))
	var out []string
	for _, k := range wellKnown {
		if spec, ok := c.Branch[k]; ok && spec.Role == RoleTopic {
			out = append(out, k)
		}
		seen[k] = true
	}

	var extra []string
	for k, spec := range c.Branch {
		if spec.Role == RoleTopic && !seen[k] {
			extra = append(extra, k)
		}
	}
	sortStrings(extra)
	return append(out, extra...)
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

func (c *Config) Spec(kind string) (BranchSpec, bool) {
	if c == nil || c.Branch == nil {
		return BranchSpec{}, false
	}
	spec, ok := c.Branch[kind]
	return spec, ok
}

func (c *Config) BaseKeys() []string {
	if c == nil {
		return nil
	}
	seen := make(map[string]bool, len(c.Branch))
	var out []string
	for _, k := range []string{"main", "develop"} {
		if spec, ok := c.Branch[k]; ok && spec.Role == RoleBase {
			out = append(out, k)
		}
		seen[k] = true
	}
	var extra []string
	for k, spec := range c.Branch {
		if spec.Role == RoleBase && !seen[k] {
			extra = append(extra, k)
		}
	}
	sortStrings(extra)
	return append(out, extra...)
}

func (spec BranchSpec) MergeTargets() []string {
	if len(spec.MergeInto) > 0 {
		return spec.MergeInto
	}
	if spec.Parent != "" {
		return []string{spec.Parent}
	}
	return nil
}

func (spec BranchSpec) Upstream() Strategy {
	if spec.UpstreamStrategy.Valid() {
		return spec.UpstreamStrategy
	}
	return StrategyMerge
}

func (spec BranchSpec) Downstream() Strategy {
	if spec.DownstreamStrategy.Valid() {
		return spec.DownstreamStrategy
	}
	return StrategyMerge
}
