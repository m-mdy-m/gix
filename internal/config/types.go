package config

// [GLOBAL]
type GlobalConfig struct {
	Users    UserConfig        `yaml:"user"`
	Defaults DefaultConfig     `yaml:"defaults"`
	Aliases  map[string]string `yaml:"aliases,omitempty"`
}

// User Config
type UserConfig struct {
	Name   string `yaml:"name"`
	Email  string `yaml:"email"`
	GPGKey string `yaml:"signing_key,omitempty"`
}

// [START] Default Config Section
type DefaultConfig struct {
	Branch    BranchConfig    `yaml:"branch,omitempty"`
	Remote    string          `yaml:"remote,omitempty"`
	Commit    CommitConfig    `yaml:"commit,omitempty"`
	Workflows WorkflowsConfig `yaml:"workflows,omitempty"`
	Hooks     HooksConfig     `yaml:"hooks,omitempty"`
}
type BranchConfig struct {
	Main    string `yaml:"main,omitempty"`
	Develop string `yaml:"develop,omitempty"`
}
type CommitConfig struct {
	Sign         bool `yaml:"sign,omitempty"`         // GPG signature
	Lint         bool `yaml:"lint,omitempty"`         // run commit linters
	Conventional bool `yaml:"conventional,omitempty"` // use conventional commits
}
type WorkflowsConfig struct {
	FeaturePrefix string `yaml:"feature_prefix,omitempty"`
	HotfixPrefix  string `yaml:"hotfix_prefix,omitempty"`
	ReleasePrefix string `yaml:"release_prefix,omitempty"`
}

type HooksConfig struct {
	PreCommit  bool `yaml:"pre_commit,omitempty"`
	PostCommit bool `yaml:"post_commit,omitempty"`
	CommitMsg  bool `yaml:"commit_msg,omitempty"`
	PrePush    bool `yaml:"pre_push,omitempty"`
}

// [END] Default Config
// [END] [GLOBAL]

// [LOCAL]

type LocalConfig struct {
	Project   LocalProject     `yaml:"project,omitempty"`
	Branch    BranchConfig     `yaml:"branch,omitempty"`
	Commit    CommitConfig     `yaml:"commit,omitempty"`
	Workflows WorkflowsConfig  `yaml:"workflows,omitempty"`
	Hooks     LocalHooksConfig `yaml:"hooks,omitempty"`
}

type LocalProject struct {
	Name string `yaml:"name,omitempty"`
}

/*
Each hook section (pre, post, commit_msg) accepts a list of
commands that are executed sequentially in the order they
are defined.
Example YAML configuration:
pre:
  - "make test"
  - "make lint"
post:
  - "echo Commit finished"
commit_msg:
  - "commitlint"
*/
type LocalHooksConfig struct {
	PreCommit  []string `yaml:"pre_commit,omitempty"`
	PostCommit []string `yaml:"post_commit,omitempty"`
	CommitMsg  []string `yaml:"commit_msg,omitempty"`
	PrePush    []string `yaml:"pre_push,omitempty"`
}
