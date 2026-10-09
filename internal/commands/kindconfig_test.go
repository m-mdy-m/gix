package commands

import (
	"strings"
	"testing"

	"github.com/m-mdy-m/gix/internal/testutil"
)

// addKind writes a new topic kind into .gix/config the way a user would,
// and commits it.
func addKind(t *testing.T, repo *testutil.Repo, kind, prefix string) {
	t.Helper()

	raw, err := readFile(repo, ".gix/config")
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	entry := "branch:\n  " + kind + ":\n    type: topic\n    prefix: " + prefix +
		"\n    parent: develop\n    upstream_strategy: merge\n    delete_on_finish: true\n"
	patched := strings.Replace(raw, "branch:\n", entry, 1)
	if patched == raw {
		t.Fatalf("could not find a branch: key in .gix/config:\n%s", raw)
	}
	writeFile(t, repo, ".gix/config", patched)
	repo.Commit("chore(gix): add a " + kind + " kind")
}

// A kind added to .gix/config must get its own command group, so adding a
// branch type is a config edit rather than a code change.
func TestConfiguredKindGetsCommandGroup(t *testing.T) {
	repo := gixRepo(t)

	if hasCommand(t, "wip") {
		t.Fatal("wip is registered before it is configured")
	}

	addKind(t, repo, "wip", "wip/")

	if !hasCommand(t, "wip") {
		t.Fatal("a kind configured in .gix/config did not get a command group")
	}

	run(t, repo, "wip", "start", "my-thing")

	if got := repo.CurrentBranch(); got != "wip/my-thing" {
		t.Errorf("current branch = %q, want wip/my-thing", got)
	}

	out := run(t, repo, "list")
	if !strings.Contains(out, "wip") || !strings.Contains(out, "my-thing") {
		t.Errorf("`gix list` should include the new kind:\n%s", out)
	}
}

func TestConfiguredKindFinish(t *testing.T) {
	repo := gixRepo(t)
	addKind(t, repo, "wip", "wip/")

	run(t, repo, "wip", "start", "my-thing")
	repo.Write("thing.txt", "work\n")
	repo.Commit("chore: work")

	run(t, repo, "wip", "finish", "my-thing")

	if got := repo.CurrentBranch(); got != "develop" {
		t.Errorf("current branch = %q, want develop after finishing", got)
	}
	if _, err := repo.Try("show-ref", "--verify", "--quiet", "refs/heads/wip/my-thing"); err == nil {
		t.Error("the branch was not deleted by finish")
	}
}

// The built-in groups must exist with or without a config, so their help
// text and error messages never depend on the working directory.
func TestBuiltInKindsExistWithoutConfig(t *testing.T) {
	testutil.NewDir(t)

	for _, kind := range []string{"feature", "bugfix", "hotfix", "release"} {
		if !hasCommand(t, kind) {
			t.Errorf("built-in kind %q is not registered outside a repository", kind)
		}
	}
}

func TestBuiltInKindsAreNotRegisteredTwice(t *testing.T) {
	gixRepo(t)

	counts := map[string]int{}
	for _, c := range New().Commands() {
		counts[c.Name()]++
	}
	for kind, n := range counts {
		if n > 1 {
			t.Errorf("command %q is registered %d times", kind, n)
		}
	}
}

// A broken config must not stop the built-in commands from working, and
// its error must name the problem rather than pretend gix is uninitialized.
func TestUnusableConfigLeavesBuiltInKindsWorking(t *testing.T) {
	repo := gixRepo(t)
	writeRawConfig(t, repo.Root, "gix:\n  version: \"1\"\nbranch: {}\n")

	if !hasCommand(t, "feature") {
		t.Error("a broken config removed the built-in kind groups")
	}
	runFailsWith(t, repo, "no branches defined", "list")
}

// A base branch key must never become a start/finish command group, even
// if the config turns it into a topic entry — otherwise a config typo
// could shadow a real command.
func TestBaseBranchKeysAreNotRegisteredAsKinds(t *testing.T) {
	repo := gixRepo(t)

	raw, err := readFile(repo, ".gix/config")
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	// Replace the base entry with a topic one of the same key.
	replaced := strings.Replace(raw, "  main:\n    name: main\n    type: base\n",
		"  main:\n    type: topic\n    prefix: main/\n    parent: develop\n", 1)
	if replaced == raw {
		t.Fatalf("could not find the main base entry:\n%s", raw)
	}
	writeFile(t, repo, ".gix/config", replaced)
	repo.Commit("chore(gix): turn main into a topic kind")

	if hasCommand(t, "main") {
		t.Error("a base branch key became a command group")
	}
	// The built-in kinds must still be registered.
	if !hasCommand(t, "feature") {
		t.Error("a base-key clash removed the built-in kinds")
	}
}

// A kind named after an existing command must not shadow it.
func TestKindCannotShadowAnExistingCommand(t *testing.T) {
	for _, name := range []string{"status", "list", "config", "doctor", "branch", "signing"} {
		t.Run(name, func(t *testing.T) {
			repo := gixRepo(t)

			raw, err := readFile(repo, ".gix/config")
			if err != nil {
				t.Fatalf("reading config: %v", err)
			}
			entry := "  " + name + ":\n    type: topic\n    prefix: " + name + "shadow/\n    parent: develop\n"
			patched := strings.Replace(raw, "branch:\n", "branch:\n"+entry, 1)
			writeFile(t, repo, ".gix/config", patched)
			repo.Commit("chore(gix): add a colliding kind")

			// The real command has to survive, so it still behaves the way
			// its own command does rather than becoming a start/finish group.
			for _, c := range New().Commands() {
				if c.Name() != name {
					continue
				}
				if c.Short == "" {
					t.Errorf("the %q command lost its help text", name)
				}
				for _, sub := range c.Commands() {
					if sub.Name() == "start" || sub.Name() == "finish" {
						t.Errorf("the %q command was replaced by a branch kind group", name)
					}
				}
				return
			}
			t.Errorf("the %q command disappeared", name)
		})
	}
}

func hasCommand(t *testing.T, name string) bool {
	t.Helper()

	for _, c := range New().Commands() {
		if c.Name() == name {
			return true
		}
	}
	return false
}
