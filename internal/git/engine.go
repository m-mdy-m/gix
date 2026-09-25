package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/m-mdy-m/gix/internal/logger"
)

var log = logger.WithPrefix("git")

func Run(args ...string) (string, error) {
	return RunEnv(nil, args...)
}

func RunEnv(extraEnv []string, args ...string) (string, error) {
	log.Verbosef("git %s", strings.Join(args, " "))

	cmd := exec.Command("git", args...)
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	log.Debugf("stdout: %q", strings.TrimSpace(stdout.String()))
	if stderr.Len() > 0 {
		log.Debugf("stderr: %q", strings.TrimSpace(stderr.String()))
	}

	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func RunInteractive(args ...string) error {
	log.Verbosef("git %s (interactive)", strings.Join(args, " "))
	cmd := exec.Command("git", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func IsRepo() bool {
	out, err := Run("rev-parse", "--is-inside-work-tree")
	return err == nil && out == "true"
}

func InitRepo(initialBranch string) error {
	args := []string{"init"}
	if initialBranch != "" {
		args = append(args, "-b", initialBranch)
	}
	_, err := Run(args...)
	return err
}

func EmptyCommit(message string, extraEnv []string) error {
	_, err := RunEnv(extraEnv, "commit", "--allow-empty", "-m", message)
	return err
}

func RepoRoot() (string, error) {
	return Run("rev-parse", "--show-toplevel")
}

func CurrentBranch() (string, error) {
	out, err := Run("symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("not currently on a branch (detached HEAD?)")
	}
	return out, nil
}

type HeadKind int

const (
	HeadOnBranch HeadKind = iota

	HeadDetached

	HeadUnborn
)

type HeadState struct {
	Kind   HeadKind
	Branch string
	Commit string
}

func GetHeadState() (HeadState, error) {
	branch, symErr := Run("symbolic-ref", "--short", "HEAD")
	if symErr != nil {

		sha, err := Run("rev-parse", "--short", "HEAD")
		if err != nil {
			return HeadState{}, fmt.Errorf("resolving HEAD: %w", err)
		}
		return HeadState{Kind: HeadDetached, Commit: sha}, nil
	}

	if _, err := Run("rev-parse", "--verify", "-q", "HEAD"); err != nil {
		return HeadState{Kind: HeadUnborn, Branch: branch}, nil
	}
	sha, _ := Run("rev-parse", "--short", "HEAD")
	return HeadState{Kind: HeadOnBranch, Branch: branch, Commit: sha}, nil
}

func HasCommits() bool {
	_, err := Run("rev-parse", "--verify", "-q", "HEAD")
	return err == nil
}

func BranchExists(name string) bool {
	_, err := Run("show-ref", "--verify", "--quiet", "refs/heads/"+name)
	return err == nil
}

func IsClean() (bool, error) {
	out, err := Run("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out == "", nil
}

type InProgressOp string

const (
	OpMerge      InProgressOp = "merge"
	OpRebase     InProgressOp = "rebase"
	OpCherryPick InProgressOp = "cherry-pick"
	OpRevert     InProgressOp = "revert"
	OpBisect     InProgressOp = "bisect"
)

func gitDir() (string, error) {
	return Run("rev-parse", "--git-dir")
}

func InProgressOperation() (op InProgressOp, found bool, err error) {
	dir, err := gitDir()
	if err != nil {
		return "", false, err
	}

	markers := []struct {
		path string
		op   InProgressOp
	}{
		{"MERGE_HEAD", OpMerge},
		{"CHERRY_PICK_HEAD", OpCherryPick},
		{"REVERT_HEAD", OpRevert},
		{"rebase-merge", OpRebase},
		{"rebase-apply", OpRebase},
		{"BISECT_LOG", OpBisect},
	}
	for _, m := range markers {
		if _, statErr := os.Stat(dir + "/" + m.path); statErr == nil {
			return m.op, true, nil
		}
	}
	return "", false, nil
}

type Identity struct {
	Name  string
	Email string
}

func (id Identity) Configured() bool { return id.Name != "" && id.Email != "" }

func CurrentIdentity() Identity {
	name, _ := Run("config", "user.name")
	email, _ := Run("config", "user.email")
	return Identity{Name: name, Email: email}
}

func ListBranches() ([]string, error) {
	out, err := Run("for-each-ref", "--format=%(refname:short)", "refs/heads/")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

func ListBranchRefs() ([]BranchRef, error) {
	names, err := ListBranches()
	if err != nil {
		return nil, err
	}
	current, _ := CurrentBranch()

	refs := make([]BranchRef, 0, len(names))
	for _, n := range names {
		refs = append(refs, BranchRef{Name: n, Current: n == current})
	}
	return refs, nil
}

func Checkout(branch string) error {
	_, err := Run("checkout", branch)
	return err
}

func CheckoutNew(branch, from string) error {
	_, err := Run("checkout", "-b", branch, from)
	return err
}

func DeleteBranch(branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := Run("branch", flag, branch)
	return err
}

func Merge(branch, message string) error {
	args := []string{"merge", "--no-ff", branch}
	if message != "" {
		args = append(args, "-m", message)
	}
	return RunInteractive(args...)
}

var ErrNothingToCommit = fmt.Errorf("nothing to commit")

//

func MergeSquash(branch, message string) error {
	if err := RunInteractive("merge", "--squash", branch); err != nil {
		return err
	}

	clean, err := IsClean()
	if err != nil {
		return err
	}
	if clean {
		return ErrNothingToCommit
	}

	args := []string{"commit"}
	if message != "" {
		args = append(args, "-m", message)
	}
	return RunInteractive(args...)
}

func RebaseOnto(onto string) error {
	return RunInteractive("rebase", onto)
}

func FastForwardMerge(branch string) error {
	_, err := Run("merge", "--ff-only", branch)
	return err
}

func Tag(name, message string) error {
	_, err := Run("tag", "-a", name, "-m", message)
	return err
}

func TagExists(name string) bool {
	out, err := Run("tag", "-l", name)
	return err == nil && out != ""
}

func AheadBehind(branch, base string) (ahead, behind int, err error) {
	out, err := Run("rev-list", "--left-right", "--count", branch+"..."+base)
	if err != nil {
		return 0, 0, err
	}
	parts := strings.Fields(out)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected rev-list output: %q", out)
	}
	ahead, err1 := strconv.Atoi(parts[0])
	behind, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("parsing rev-list output %q", out)
	}
	return ahead, behind, nil
}

//

func IsAncestor(ancestor, descendant string) bool {
	_, err := Run("merge-base", "--is-ancestor", ancestor, descendant)
	return err == nil
}

func GetStatus(base string) (Status, error) {
	head, err := GetHeadState()
	if err != nil {
		return Status{}, err
	}

	clean, err := IsClean()
	if err != nil {
		return Status{}, err
	}

	st := Status{Head: head, Branch: head.Branch, Clean: clean, Base: base}

	if base != "" && head.Kind == HeadOnBranch && head.Branch != base && BranchExists(base) {
		ahead, behind, err := AheadBehind(head.Branch, base)
		if err == nil {
			st.Ahead, st.Behind, st.HasBase = ahead, behind, true
		}
	}

	return st, nil
}
