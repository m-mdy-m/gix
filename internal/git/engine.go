package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/m-mdy-m/gix/internal/logger"
)

var log = logger.WithPrefix("git")

// RunStatus runs git and returns its stdout even when the command exits
// non-zero.
//
// Some git commands put the real answer on stdout and use the exit status
// only as a flag — `merge-tree` prints the conflicted file names that way.
// Run deliberately returns "" on error, so those answers would be lost.
func RunStatus(args ...string) (string, error) {
	log.Verbosef("git %s", strings.Join(args, " "))

	cmd := exec.Command("git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	log.Debugf("stdout: %q", strings.TrimSpace(stdout.String()))
	if stderr.Len() > 0 {
		log.Debugf("stderr: %q", strings.TrimSpace(stderr.String()))
	}

	return strings.TrimSpace(stdout.String()), err
}

// MergeTreeConflicts reports the paths a merge of two branches would
// conflict on, without touching the repository. No conflicts means the
// merge is clean.
//
// merge-tree prints each conflict as a tab-separated stage line and exits
// non-zero when there was at least one, so the exit status and the file
// list have to be read together.
func MergeTreeConflicts(ours, theirs string) ([]string, error) {
	out, err := RunStatus("merge-tree", "--write-tree", ours, theirs)
	if err == nil {
		return nil, nil
	}

	var files []string
	seen := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		// A conflict line looks like "100644 <sha> 2<TAB>path". Only those
		// carry a tab; the rest of the output is the resulting tree id.
		_, path, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		path = strings.TrimSpace(path)
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		files = append(files, path)
	}

	// A conflict git can't express as a stage line is still worth naming,
	// so fall back to the message it wrote.
	for _, file := range conflictFilesFromMessage(err.Error()) {
		if !seen[file] {
			seen[file] = true
			files = append(files, file)
		}
	}

	return files, nil
}

// conflictFilesFromMessage pulls paths out of merge-tree's
// "CONFLICT (<kind>): Merge conflict in <path>" lines.
func conflictFilesFromMessage(msg string) []string {
	var files []string
	for _, line := range strings.Split(msg, "\n") {
		_, after, ok := strings.Cut(line, "Merge conflict in ")
		if !ok {
			continue
		}
		if file := strings.TrimSpace(after); file != "" {
			files = append(files, file)
		}
	}
	return files
}

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

// Scopes a config value can come from. These are git's own words, so
// reporting them verbatim keeps `gix doctor` honest about where a setting
// actually lives.
const (
	ScopeLocal    = "local"
	ScopeGlobal   = "global"
	ScopeSystem   = "system"
	ScopeWorktree = "worktree"
	ScopeCommand  = "command"
)

// ConfigEntry is one effective config value plus the scope it came from.
type ConfigEntry struct {
	Value string
	Scope string
}

// Config is the effective git configuration, keyed by config name.
// Last value wins, matching how git resolves multi-valued keys.
type Config map[string]ConfigEntry

// Get returns the value and scope for key, and whether it is set at all.
func (c Config) Get(key string) (ConfigEntry, bool) {
	e, ok := c[key]
	return e, ok
}

// ReadConfig snapshots the effective configuration with one git call.
// Reading it as a list, rather than one `config --get` per key, keeps
// unset keys unambiguous: a missing key exits non-zero exactly like a
// genuine failure, so its error can't be told apart from one.
func ReadConfig() (Config, error) {
	out, err := Run("config", "--show-scope", "--list")
	if err != nil {
		return nil, err
	}

	cfg := Config{}
	for _, line := range strings.Split(out, "\n") {
		scope, rest, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		key, value, ok := strings.Cut(rest, "=")
		if !ok {
			continue
		}
		cfg[key] = ConfigEntry{Value: value, Scope: scope}
	}
	return cfg, nil
}

// ConfigValue returns a single config value, or "" when it isn't set.
func (c Config) ConfigValue(key string) string {
	e, _ := c.Get(key)
	return e.Value
}

// SetConfig writes key at the given scope. Only local and global are
// accepted: they are the two a person decides on purpose, and anything
// else would mean editing config gix doesn't own.
func SetConfig(key, value, scope string) error {
	args := []string{"config"}
	switch scope {
	case "", ScopeLocal:
	case ScopeGlobal:
		args = append(args, "--global")
	default:
		return fmt.Errorf("unsupported config scope %q (want %s or %s)", scope, ScopeLocal, ScopeGlobal)
	}
	args = append(args, key, value)

	_, err := Run(args...)
	if err != nil {
		return fmt.Errorf("setting %s in %s config: %w", key, scopeOrDefault(scope), err)
	}
	return nil
}

func scopeOrDefault(scope string) string {
	if scope == "" {
		return ScopeLocal
	}
	return scope
}

// Remotes lists the configured remote names.
func Remotes() ([]string, error) {
	out, err := Run("remote")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// HasRemote reports whether a remote by that name exists.
func HasRemote(name string) bool {
	out, err := Run("remote")
	if err != nil {
		return false
	}
	for _, r := range strings.Split(out, "\n") {
		if r == name {
			return true
		}
	}
	return false
}

// UpstreamOf returns the ref a branch tracks, e.g. "origin/main". A branch
// with no upstream yields "" and no error, because that is a normal state
// rather than a failure.
func UpstreamOf(branch string) (string, error) {
	ref := branch + "@{upstream}"
	out, err := Run("rev-parse", "--abbrev-ref", "--symbolic-full-name", ref)
	if err != nil {
		return "", nil
	}
	return out, nil
}

// HooksPath returns the hooks directory git runs hooks from, honouring
// core.hooksPath when it is set.
func HooksPath() (string, error) {
	return Run("rev-parse", "--git-path", "hooks")
}

// ListTags returns every tag name, newest version first.
//
// Git's version sort is used rather than alphabetical order, because it
// is what makes v0.10.0 come before v0.9.0 — the difference between "the
// latest release" and the wrong one.
func ListTags() ([]string, error) {
	out, err := Run("tag", "--list", "--sort=-v:refname")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// TrackStatus is git's own summary of how a branch compares to its
// upstream, exactly as %(upstream:track) renders it: "" when in sync,
// "[ahead 1]", "[behind 2]", "[ahead 1, behind 2]", or "[gone]".
type TrackStatus string

func (t TrackStatus) Gone() bool { return t == "[gone]" }

// counts pulls the ahead and behind numbers out of git's own wording, so
// git stays the single source of truth for what "behind" means.
func (t TrackStatus) counts() (ahead, behind int) {
	if t == "" || t.Gone() {
		return 0, 0
	}
	// Git brackets the whole summary, so "[ahead 1, behind 2]" splits
	// into "[ahead 1" and "behind 2]" unless the brackets come off first.
	for _, part := range strings.Split(strings.Trim(string(t), "[]"), ",") {
		fields := strings.Fields(part)
		if len(fields) != 2 {
			continue
		}
		n, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		switch fields[0] {
		case "ahead":
			ahead = n
		case "behind":
			behind = n
		}
	}
	return ahead, behind
}

// BranchTracking pairs a local branch with what git knows about its
// upstream. Track is empty for a branch that tracks nothing.
type BranchTracking struct {
	Name     string
	Upstream string
	Track    TrackStatus
}

// ListBranchTracking returns every local branch with git's upstream and
// ahead/behind summary, in refname order.
//
// This is one git call on purpose: %(upstream:track) already resolves the
// tracking configuration and the commit counts, including the "[gone]"
// case that has to be distinguished from "no upstream configured".
// Asking git per branch instead would re-derive what it computed here.
func ListBranchTracking() ([]BranchTracking, error) {
	const format = "%(refname:short)%09%(upstream:short)%09%(upstream:track)"
	out, err := Run("for-each-ref", "--format="+format, "refs/heads/")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	var refs []BranchTracking
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		// SplitN, not Split: git output is trimmed, so the final branch's
		// empty trailing field is gone and a plain split would drop it.
		fields := strings.SplitN(line, "\t", 3)
		ref := BranchTracking{Name: fields[0]}
		if len(fields) > 1 {
			ref.Upstream = fields[1]
		}
		if len(fields) > 2 {
			ref.Track = TrackStatus(fields[2])
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

// BranchCommit is when a branch's tip commit was committed, as git
// reports it. A branch with no commit yet reports Valid false.
type BranchCommit struct {
	Name  string
	When  time.Time
	Valid bool
}

// DaysSince reports how many whole days ago the commit was. The reference
// point is passed in so a whole report is measured against one moment
// rather than each call drifting a few milliseconds apart.
func (b BranchCommit) DaysSince(now time.Time) int {
	if !b.Valid {
		return 0
	}
	return int(now.Sub(b.When).Hours() / 24)
}

// ListBranchCommits returns every local branch with the date of its tip
// commit, in the same refname order as ListBranchTracking.
func ListBranchCommits() ([]BranchCommit, error) {
	const format = "%(refname:short)%09%(committerdate:unix)"
	out, err := Run("for-each-ref", "--format="+format, "refs/heads/")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	var commits []BranchCommit
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 2)
		commit := BranchCommit{Name: fields[0]}
		if len(fields) > 1 {
			if secs, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
				commit.When = time.Unix(secs, 0)
				commit.Valid = true
			}
		}
		commits = append(commits, commit)
	}
	return commits, nil
}

// Ahead returns how many commits the branch has that its upstream lacks.
func (t TrackStatus) Ahead() int { ahead, _ := t.counts(); return ahead }

// Behind returns how many commits the upstream has that the branch lacks.
func (t TrackStatus) Behind() int { _, behind := t.counts(); return behind }

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

// MergeNoFF merges non-interactively, recording a merge commit even when a
// fast-forward would do. A sync wants the fact that the source was pulled
// in to be visible in the history, not to disappear into a fast-forward.
func MergeNoFF(branch, message string) error {
	args := []string{"merge", "--no-ff", "--no-edit", branch}
	if message != "" {
		args = append(args, "-m", message)
	}
	_, err := Run(args...)
	return err
}

// AbortMerge clears an unfinished merge, leaving the working tree the way
// it was before the merge started.
func AbortMerge() error {
	_, err := Run("merge", "--abort")
	return err
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
