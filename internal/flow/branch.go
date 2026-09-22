package flow

import (
	"errors"
	"fmt"
	"strings"

	"github.com/m-mdy-m/gix/internal/config"
	"github.com/m-mdy-m/gix/internal/git"
)

func FullName(cfg *config.Config, kind, short string) (string, error) {
	spec, ok := cfg.Spec(kind)
	if !ok || spec.Role != config.RoleTopic {
		return "", fmt.Errorf("unknown branch kind %q", kind)
	}
	short = strings.TrimPrefix(short, spec.Prefix)
	return spec.Prefix + short, nil
}

func Start(cfg *config.Config, kind, short string) (string, error) {
	spec, ok := cfg.Spec(kind)
	if !ok || spec.Role != config.RoleTopic {
		return "", fmt.Errorf("unknown branch kind %q", kind)
	}
	if short == "" {
		return "", fmt.Errorf("%s name is required, e.g. `gix %s start api-auth`", kind, kind)
	}

	if err := preflightMutation(); err != nil {
		return "", err
	}

	full, err := FullName(cfg, kind, short)
	if err != nil {
		return "", err
	}
	if err := validateBranchName(full); err != nil {
		return "", err
	}
	if git.BranchExists(full) {
		return "", fmt.Errorf("branch %s already exists", full)
	}

	if spec.Parent == "" {
		return "", fmt.Errorf("%s has no configured parent branch", kind)
	}
	base := resolveBranchName(cfg, spec.Parent)
	if !git.BranchExists(base) {
		return "", fmt.Errorf("base branch %s does not exist — run `gix flow init` first", base)
	}

	log.Verbosef("starting %s from %s", full, base)
	if err := git.CheckoutNew(full, base); err != nil {
		return "", err
	}
	return full, nil
}

type FinishOptions struct {
	DeleteBranch bool

	Tag string

	Strategy config.Strategy
}

type FinishResult struct {
	Branch     string
	Kind       string
	Strategy   config.Strategy
	MergedInto []string

	AlreadyMerged []string
	Tagged        string
	TaggedOn      string

	BranchDeleted   bool
	DeleteAttempted bool
	DeleteErr       error
}

func Finish(cfg *config.Config, kind, short string, opts FinishOptions) (*FinishResult, error) {
	spec, ok := cfg.Spec(kind)
	if !ok || spec.Role != config.RoleTopic {
		return nil, fmt.Errorf("unknown branch kind %q", kind)
	}
	if short == "" {
		return nil, fmt.Errorf("%s name is required, e.g. `gix %s finish api-auth`", kind, kind)
	}

	full, err := FullName(cfg, kind, short)
	if err != nil {
		return nil, err
	}
	if !git.BranchExists(full) {
		return nil, fmt.Errorf("branch %s does not exist", full)
	}

	if err := preflightMutation(); err != nil {
		return nil, err
	}

	if opts.Tag != "" && !spec.Tag {
		return nil, fmt.Errorf("%s branches don't support --tag (only kinds with tag: true in config do)", kind)
	}

	strategy := spec.Upstream()
	if opts.Strategy != "" {
		if !opts.Strategy.Valid() {
			return nil, fmt.Errorf("invalid strategy %q", opts.Strategy)
		}
		strategy = opts.Strategy
	}

	targetKeys := spec.MergeTargets()
	if len(targetKeys) == 0 {
		return nil, fmt.Errorf("%s has no configured merge target", kind)
	}

	targets := make([]string, 0, len(targetKeys))
	for _, key := range targetKeys {
		name := resolveBranchName(cfg, key)
		if !git.BranchExists(name) {
			return nil, fmt.Errorf("cannot finish %s: merge target %q (branch %q) does not exist", full, key, name)
		}
		targets = append(targets, name)
	}

	mainName := cfg.MainName()
	if opts.Tag != "" {
		if !git.BranchExists(mainName) {
			return nil, fmt.Errorf("cannot tag: main branch %q does not exist", mainName)
		}
		if git.TagExists(opts.Tag) {
			return nil, fmt.Errorf("tag %s already exists", opts.Tag)
		}
	}

	result := &FinishResult{Branch: full, Kind: kind, Strategy: strategy}

	for _, target := range targets {

		if git.IsAncestor(full, target) {
			log.Verbosef("%s already contains %s — skipping fold, no-op", target, full)
			result.AlreadyMerged = append(result.AlreadyMerged, target)
			continue
		}

		err := foldInto(target, full, strategy)
		if errors.Is(err, errAlreadyFolded) {
			log.Verbosef("%s already has %s's content — skipping, no-op", target, full)
			result.AlreadyMerged = append(result.AlreadyMerged, target)
			continue
		}
		if err != nil {
			if len(result.MergedInto) > 0 || len(result.AlreadyMerged) > 0 {
				done := append(append([]string{}, result.AlreadyMerged...), result.MergedInto...)
				return nil, fmt.Errorf("%w\n\n%s was already merged into %s before this failed — that merge was NOT undone; %s was NOT deleted",
					err, full, strings.Join(done, ", "), full)
			}
			return nil, err
		}
		result.MergedInto = append(result.MergedInto, target)
	}

	if opts.Tag != "" {
		if err := git.Checkout(mainName); err != nil {
			return result, fmt.Errorf("merges succeeded but switching to %s to tag failed: %w\n\n%s was NOT deleted", mainName, err, full)
		}
		if err := git.Tag(opts.Tag, fmt.Sprintf("%s %s", titleCase(kind), opts.Tag)); err != nil {
			return result, fmt.Errorf("merges succeeded but tagging failed: %w\n\n%s was NOT deleted", err, full)
		}
		result.Tagged = opts.Tag
		result.TaggedOn = mainName
	}

	if opts.DeleteBranch {
		result.DeleteAttempted = true
		if err := git.DeleteBranch(full, true); err != nil {

			result.DeleteErr = err
		} else {
			result.BranchDeleted = true
		}
	}

	return result, nil
}

func preflightMutation() error {
	op, found, err := git.InProgressOperation()
	if err != nil {
		return err
	}
	if found {
		return fmt.Errorf("repository has an unfinished git operation in progress: %s\n\nresolve conflicts and run `git %s --continue`, or abandon it with `git %s --abort`, then re-run this gix command", op, op, op)
	}

	clean, err := git.IsClean()
	if err != nil {
		return err
	}
	if !clean {
		return fmt.Errorf("working tree is not clean — commit or stash your changes first")
	}

	return nil
}

func resolveBranchName(cfg *config.Config, key string) string {
	if spec, ok := cfg.Spec(key); ok && spec.Name != "" {
		return spec.Name
	}
	return key
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func List(cfg *config.Config, kind string) ([]string, error) {
	spec, ok := cfg.Spec(kind)
	if !ok || spec.Role != config.RoleTopic {
		return nil, fmt.Errorf("unknown branch kind %q", kind)
	}

	all, err := git.ListBranches()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, b := range all {
		if strings.HasPrefix(b, spec.Prefix) {
			out = append(out, strings.TrimPrefix(b, spec.Prefix))
		}
	}
	return out, nil
}
