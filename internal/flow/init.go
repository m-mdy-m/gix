package flow

import (
	"fmt"
	"strings"

	"github.com/m-mdy-m/gix/internal/config"
	"github.com/m-mdy-m/gix/internal/git"
	"github.com/m-mdy-m/gix/internal/logger"
)

var log = logger.WithPrefix("flow")

type InitOptions struct {
	MainName string

	DevelopName string

	NoCommit bool

	AuthorName  string
	AuthorEmail string
}

type InitResult struct {
	Config *config.Config

	ConfigWritten  bool
	Committed      bool
	CommitSkipped  bool
	SkipReason     string
	DevelopCreated bool
	DevelopExisted bool
}

//

func Init(repoRoot string, opts InitOptions) (*InitResult, error) {
	if !git.HasCommits() {
		return nil, fmt.Errorf(`cannot initialize gix flow

the repository has no commits yet, so there is no base branch for
gix to build the flow on top of.

create an initial commit first, e.g.:

  git commit --allow-empty -m "initial commit"

then run gix flow init again`)
	}

	configRel := config.DirName + "/" + config.FileName
	partial := config.Exists(repoRoot) && !committedPath(configRel)
	if config.Exists(repoRoot) && !partial {
		return nil, fmt.Errorf("gix is already initialized (%s exists)", config.Path(repoRoot))
	}

	if err := validateInitNames(opts.MainName, opts.DevelopName); err != nil {
		return nil, err
	}

	result := &InitResult{}

	var cfg *config.Config
	if partial {

		loaded, err := config.Load(repoRoot)
		if err != nil {
			return nil, fmt.Errorf("%s exists but is invalid, and gix isn't committed yet: %w\n\nfix or remove %s and re-run `gix flow init`", config.Path(repoRoot), err, config.Path(repoRoot))
		}
		cfg = loaded
		log.Verbosef("resuming init: %s already written but not committed", configRel)
	} else {
		cfg = config.Default()

		mainSpec := cfg.Branch["main"]
		switch {
		case opts.MainName != "":
			mainSpec.Name = opts.MainName
		default:

			if head, err := git.GetHeadState(); err == nil && head.Kind == git.HeadOnBranch {
				if head.Branch == "main" || head.Branch == "master" || head.Branch == "trunk" {
					mainSpec.Name = head.Branch
				}
			}
		}
		cfg.Branch["main"] = mainSpec

		developSpec := cfg.Branch["develop"]
		if opts.DevelopName != "" {
			developSpec.Name = opts.DevelopName
		}
		cfg.Branch["develop"] = developSpec

		if err := config.Save(repoRoot, cfg); err != nil {
			return nil, err
		}
		result.ConfigWritten = true
	}

	mainName := cfg.MainName()
	developName := cfg.DevelopName()

	if !git.BranchExists(mainName) {
		suggestion := ""
		if all, err := git.ListBranches(); err == nil && len(all) > 0 {
			suggestion = fmt.Sprintf("\n\nrepository branches:\n  %s\n\nuse --main=<name> to point gix at the right one", strings.Join(all, "\n  "))
		}
		return nil, fmt.Errorf(`cannot initialize gix flow

configured main branch %q does not exist%s`, mainName, suggestion)
	}

	if err := commitConfigStep(repoRoot, configRel, opts, result); err != nil {
		return nil, err
	}

	if git.BranchExists(developName) {
		result.DevelopExisted = true
	} else {
		log.Verbosef("creating %s from %s", developName, mainName)
		if err := git.CheckoutNew(developName, mainName); err != nil {
			return nil, fmt.Errorf("creating %s branch: %w", developName, err)
		}
		result.DevelopCreated = true
	}

	result.Config = cfg
	return result, nil
}

func commitConfigStep(repoRoot, configRel string, opts InitOptions, result *InitResult) error {
	if opts.NoCommit {
		result.CommitSkipped = true
		result.SkipReason = "--no-commit was passed"
		return nil
	}

	haveOverride := opts.AuthorName != "" && opts.AuthorEmail != ""
	identity := git.CurrentIdentity()
	if !identity.Configured() && !haveOverride {
		result.CommitSkipped = true
		result.SkipReason = "no git author identity is configured"
		return nil
	}

	log.Verbosef("staging and committing %s", configRel)
	if _, err := git.Run("add", configRel); err != nil {
		return fmt.Errorf("staging %s: %w", configRel, err)
	}

	commitArgs := []string{"commit", "-m", "chore(gix): initialize gix flow config"}
	var env []string
	if haveOverride {
		env = []string{
			"GIT_AUTHOR_NAME=" + opts.AuthorName,
			"GIT_AUTHOR_EMAIL=" + opts.AuthorEmail,
			"GIT_COMMITTER_NAME=" + opts.AuthorName,
			"GIT_COMMITTER_EMAIL=" + opts.AuthorEmail,
		}
	}
	if _, err := git.RunEnv(env, commitArgs...); err != nil {
		return fmt.Errorf("committing %s: %w", configRel, err)
	}
	result.Committed = true
	return nil
}

func committedPath(path string) bool {
	_, err := git.Run("cat-file", "-e", "HEAD:"+path)
	return err == nil
}

func validateInitNames(mainName, developName string) error {
	if mainName != "" {
		if err := validateBranchName(mainName); err != nil {
			return fmt.Errorf("--main: %w", err)
		}
	}
	if developName != "" {
		if err := validateBranchName(developName); err != nil {
			return fmt.Errorf("--develop: %w", err)
		}
	}
	if mainName != "" && mainName == developName {
		return fmt.Errorf("invalid flow configuration: --main and --develop must be different (both %q)", mainName)
	}
	return nil
}

func validateBranchName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("branch name cannot be empty")
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("invalid branch name %q: cannot contain \"..\"", name)
	}
	if strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") {
		return fmt.Errorf("invalid branch name %q: cannot start or end with \"/\"", name)
	}
	if strings.ContainsAny(name, " \t\n~^:?*[\\") {
		return fmt.Errorf("invalid branch name %q: contains characters Git doesn't allow in refs", name)
	}
	return nil
}
