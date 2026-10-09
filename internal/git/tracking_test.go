package git

import (
	"strings"
	"testing"
	"time"

	"github.com/m-mdy-m/gix/internal/testutil"
)

// track makes a branch look like it tracks remote/branch, without needing
// a real remote to push to. Safe to call for several branches on one
// remote.
func track(t *testing.T, repo *testutil.Repo, branch, remote, remoteBranch string) {
	t.Helper()

	if _, err := repo.Try("remote", "add", remote, "https://example.invalid/"+remote+".git"); err != nil {
		if out, err := repo.Try("remote"); err != nil || !containsLine(strings.Split(out, "\n"), remote) {
			t.Fatalf("adding remote %s: %v\n%s", remote, err, out)
		}
	}
	repo.Run("update-ref", "refs/remotes/"+remote+"/"+remoteBranch, remoteBranch)
	repo.Run("config", "branch."+branch+".remote", remote)
	repo.Run("config", "branch."+branch+".merge", "refs/heads/"+remoteBranch)
}

func containsLine(lines []string, line string) bool {
	for _, l := range lines {
		if l == line {
			return true
		}
	}
	return false
}

func TestListBranchTracking(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Branch("develop")
	track(t, repo, "main", "origin", "main")

	refs, err := ListBranchTracking()
	if err != nil {
		t.Fatalf("ListBranchTracking: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("got %d branches, want 2: %+v", len(refs), refs)
	}

	if refs[0].Name != "develop" || refs[0].Upstream != "" || refs[0].Track != "" {
		t.Errorf("develop = %+v, want no upstream", refs[0])
	}
	if refs[1].Name != "main" || refs[1].Upstream != "origin/main" || refs[1].Track != "" {
		t.Errorf("main = %+v, want origin/main and no divergence", refs[1])
	}
}

// The last branch git prints must not be dropped when its trailing field
// is empty: trimmed command output loses that final empty column.
func TestListBranchTrackingKeepsLastBranch(t *testing.T) {
	repo := testutil.NewRepo(t)
	track(t, repo, "main", "origin", "main")

	refs, err := ListBranchTracking()
	if err != nil {
		t.Fatalf("ListBranchTracking: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("got %d branches, want main to survive parsing: %+v", len(refs), refs)
	}
	if refs[0].Name != "main" {
		t.Errorf("Name = %q, want main", refs[0].Name)
	}
	if refs[0].Upstream != "origin/main" {
		t.Errorf("Upstream = %q, want origin/main", refs[0].Upstream)
	}
}

func TestListBranchTrackingOrder(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Branch("zeta")
	repo.Branch("alpha")
	repo.Branch("middle")

	refs, err := ListBranchTracking()
	if err != nil {
		t.Fatalf("ListBranchTracking: %v", err)
	}

	var names []string
	for _, r := range refs {
		names = append(names, r.Name)
	}
	if got := strings.Join(names, ","); got != "alpha,main,middle,zeta" {
		t.Errorf("order = %q, want git's stable refname order", got)
	}
}

func TestTrackStatusCounts(t *testing.T) {
	tests := []struct {
		status     TrackStatus
		wantGone   bool
		wantAhead  int
		wantBehind int
	}{
		{status: "", wantAhead: 0, wantBehind: 0},
		{status: "[ahead 1]", wantAhead: 1, wantBehind: 0},
		{status: "[ahead 12]", wantAhead: 12, wantBehind: 0},
		{status: "[behind 3]", wantAhead: 0, wantBehind: 3},
		{status: "[ahead 1, behind 2]", wantAhead: 1, wantBehind: 2},
		{status: "[behind 7, ahead 1]", wantAhead: 1, wantBehind: 7},
		{status: "[gone]", wantGone: true, wantAhead: 0, wantBehind: 0},
		{status: "[something new]", wantAhead: 0, wantBehind: 0},
	}

	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			if got := tc.status.Gone(); got != tc.wantGone {
				t.Errorf("Gone() = %v, want %v", got, tc.wantGone)
			}
			if got := tc.status.Ahead(); got != tc.wantAhead {
				t.Errorf("Ahead() = %d, want %d", got, tc.wantAhead)
			}
			if got := tc.status.Behind(); got != tc.wantBehind {
				t.Errorf("Behind() = %d, want %d", got, tc.wantBehind)
			}
		})
	}
}

func TestListBranchTrackingDiverged(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Branch("topic")
	track(t, repo, "topic", "origin", "topic")

	// topic gets a commit its upstream never sees…
	repo.Checkout("topic")
	repo.Write("topic.txt", "topic\n")
	repo.Commit("feat: topic")

	// …while the remote moves on with two commits topic never sees.
	repo.Checkout("main")
	repo.Write("m1.txt", "m1\n")
	repo.Commit("chore: main one")
	repo.Write("m2.txt", "m2\n")
	repo.Commit("chore: main two")
	repo.Run("update-ref", "refs/remotes/origin/topic", "main")
	repo.Checkout("topic")

	refs, err := ListBranchTracking()
	if err != nil {
		t.Fatalf("ListBranchTracking: %v", err)
	}

	var topic *BranchTracking
	for i := range refs {
		if refs[i].Name == "topic" {
			topic = &refs[i]
		}
	}
	if topic == nil {
		t.Fatalf("topic is missing from %+v", refs)
	}
	if topic.Track.Behind() != 2 {
		t.Errorf("Behind() = %d, want 2", topic.Track.Behind())
	}
	if topic.Track.Ahead() != 1 {
		t.Errorf("Ahead() = %d, want 1", topic.Track.Ahead())
	}
}

func TestListBranchCommits(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Branch("topic")

	commits, err := ListBranchCommits()
	if err != nil {
		t.Fatalf("ListBranchCommits: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("got %d branches, want 2: %+v", len(commits), commits)
	}

	// Both branches point at the same commit, so both dates are valid.
	for _, c := range commits {
		if !c.Valid {
			t.Errorf("%s has Valid = false on a branch with a commit", c.Name)
		}
		if got := c.DaysSince(time.Now()); got != 0 {
			t.Errorf("%s DaysSince = %d, want 0 for a commit just made", c.Name, got)
		}
	}
}

func TestBranchCommitDaysSince(t *testing.T) {
	now := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		commit BranchCommit
		want   int
	}{
		{name: "same day", commit: BranchCommit{When: now.Add(-3 * time.Hour), Valid: true}, want: 0},
		{name: "one day", commit: BranchCommit{When: now.Add(-25 * time.Hour), Valid: true}, want: 1},
		{name: "nine days", commit: BranchCommit{When: now.Add(-9 * 24 * time.Hour), Valid: true}, want: 9},
		{name: "no commit", commit: BranchCommit{}, want: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.commit.DaysSince(now); got != tc.want {
				t.Errorf("DaysSince = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestListBranchTrackingGone(t *testing.T) {
	repo := testutil.NewRepo(t)
	track(t, repo, "main", "origin", "main")

	refs, err := ListBranchTracking()
	if err != nil {
		t.Fatalf("ListBranchTracking: %v", err)
	}
	if len(refs) != 1 || refs[0].Track.Gone() {
		t.Fatalf("main = %+v, want it tracked before the ref is removed", refs)
	}

	// Deleting the remote-tracking ref is what a fetch --prune sees when
	// the branch was deleted on the remote.
	repo.Run("update-ref", "-d", "refs/remotes/origin/main")

	refs, err = ListBranchTracking()
	if err != nil {
		t.Fatalf("ListBranchTracking: %v", err)
	}
	if !refs[0].Track.Gone() {
		t.Errorf("main = %+v, want its upstream reported as gone", refs[0])
	}
	if refs[0].Upstream != "origin/main" {
		t.Errorf("Upstream = %q, want the configured upstream to still be named", refs[0].Upstream)
	}
}
