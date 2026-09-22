package flow

import (
	"errors"
	"fmt"

	"github.com/m-mdy-m/gix/internal/config"
	"github.com/m-mdy-m/gix/internal/git"
)

func foldInto(target, branch string, strategy config.Strategy) error {
	log.Verbosef("folding %s into %s via %s", branch, target, strategy)

	switch strategy {
	case config.StrategyMerge, "":
		return mergeInto(target, branch)
	case config.StrategyRebase:
		return rebaseInto(target, branch)
	case config.StrategySquash:
		return squashInto(target, branch)
	default:
		return fmt.Errorf("unknown merge strategy %q", strategy)
	}
}

func mergeInto(target, branch string) error {
	if err := git.Checkout(target); err != nil {
		return fmt.Errorf("switching to %s: %w", target, err)
	}
	msg := fmt.Sprintf("Merge branch '%s' into %s", branch, target)
	if err := git.Merge(branch, msg); err != nil {
		return fmt.Errorf("merging %s into %s: %w (resolve conflicts, then re-run finish)", branch, target, err)
	}
	return nil
}

func rebaseInto(target, branch string) error {
	if err := git.Checkout(branch); err != nil {
		return fmt.Errorf("switching to %s: %w", branch, err)
	}
	if err := git.RebaseOnto(target); err != nil {
		return fmt.Errorf("rebasing %s onto %s: %w (resolve conflicts, then re-run finish)", branch, target, err)
	}
	if err := git.Checkout(target); err != nil {
		return fmt.Errorf("switching to %s: %w", target, err)
	}
	if err := git.FastForwardMerge(branch); err != nil {
		return fmt.Errorf("fast-forwarding %s to %s: %w", target, branch, err)
	}
	return nil
}

func squashInto(target, branch string) error {
	if err := git.Checkout(target); err != nil {
		return fmt.Errorf("switching to %s: %w", target, err)
	}
	msg := fmt.Sprintf("Squash branch '%s' into %s", branch, target)
	err := git.MergeSquash(branch, msg)
	if errors.Is(err, git.ErrNothingToCommit) {
		return errAlreadyFolded
	}
	if err != nil {
		return fmt.Errorf("squash-merging %s into %s: %w (resolve conflicts, then re-run finish)", branch, target, err)
	}
	return nil
}

var errAlreadyFolded = fmt.Errorf("already up to date")
