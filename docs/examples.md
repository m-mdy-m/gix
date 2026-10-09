# Examples

Real sequences, copy-paste style. Assumes `gix flow init` was run.

## Normal feature

```bash
gix feature start login-form
# hack, then:
git add -A && git commit -m "feat(ui): add login form"
gix feature finish login-form
# → merged into develop, feature/login-form deleted
```

## Bug in develop

```bash
gix bugfix start null-pointer
# fix it, commit...
gix bugfix finish null-pointer
```

## Production is on fire

```bash
gix hotfix start crash-on-startup
# fix on a branch off main...
git commit -am "fix: crash on startup"
gix hotfix finish crash-on-startup --tag v1.0.1
# → merged into main AND develop, tagged on main, branch deleted
```

## Release

```bash
gix release start v1.1.0
# bump version, update CHANGELOG, commit...
gix release finish v1.1.0 --tag v1.1.0
```

## Keep the branch after merging

```bash
gix feature finish login-form --no-delete
```

## Squash instead of merge commit

```bash
gix feature finish login-form --strategy squash
# strategies: merge | rebase | squash
```

## See what's going on

```bash
gix status           # where am I?
gix list             # flat list of open branches
gix list --tree      # tree view of the whole flow
gix config list      # the active flow config
gix doctor           # anything that would stop gix working
gix branch doctor    # every branch's upstream and ahead/behind state
```

`gix branch doctor` is the quickest way to answer "is anything out of
sync?":

```
BRANCH             KIND    UPSTREAM           AHEAD  BEHIND  STATE
feature/login-form feature origin/feature/…    1      3       diverged
bugfix/null-deref  bugfix  -                   0      0       untracked
develop            base    origin/develop      0      0       tracked
```

`behind` just means the remote branch moved on. `diverged` means you
have commits it doesn't and it has commits you don't — resolve that
before pushing, or `git push` will be rejected.

## Cleaning up leftovers

```bash
gix branch prune
```

Lists branches whose work is already merged, or whose upstream is gone.
It is a preview and deletes nothing:

```
BRANCH        KIND    WHY
feature/old   feature  already merged into develop

this is a preview — gix has not deleted anything
```

Base branches and the branch you're on are never listed, and a branch
with unmerged work is never a candidate.

To actually delete what is fully merged:

```bash
gix branch prune --apply
```

This uses `git branch -d`, never `-D` — git's own refusal to delete an
unmerged branch is the safety net. A branch whose upstream merely
disappeared is listed but left alone, because a missing upstream does
not mean its work was merged.

## develop moved on, bring it into your branches

```bash
gix branch sync --dry-run    # see what would happen first
gix branch sync              # then do it
```

```
BRANCH              RESULT
feature/clean       ✓ merged
feature/conflicted  ✗ conflict
  • feature/conflicted: conflicting files: shared.txt

✗ feature/conflicted could not take the merge from develop — the other branches were synced
```

The conflicting branch is left exactly as it was — gix reports the
problem and never resolves it for you. Fix it, finish it, or delete it;
then re-run the sync. The other branches were still synced.

Sync only ever merges, so no history is rewritten, and it puts you back
on the branch you started on.

## Only update the branches that asked for it

Mark a kind (or a base branch) with `autosync: true`, then sync just
those:

```bash
gix config set feature --autosync=true
gix branch sync --from=develop --auto
```

The default config already marks `develop` as autosync of `main`, so
this keeps `develop` up to date with `main`:

```bash
gix branch sync --from=main --auto --dry-run
```

```
BRANCH   WOULD
develop  would merge
```

`--auto` is only a filter over which branches are considered — a plain
`gix branch sync` already covers every feature branch. And gix never
syncs on its own: no hooks, no background process. You run the command
or you don't.

## Cutting a release

```bash
gix version show                 # where are we?
gix version bump minor --dry-run # what would happen?
gix version bump minor           # creates v0.3.0
```

```
$ gix version bump minor --dry-run
→ dry run — nothing was created
  current     0.2.0 (v0.2.0)
  next        0.3.0
  tag         v0.3.0
  commit      716ef9d (main)
  since       12 commit(s)

gix would create the tag v0.3.0 and nothing else
```

gix only picks the number. It does not touch `CHANGELOG.md`, does not
make a commit, does not sign, and does not push — so the release is
yours:

```bash
# 1. update CHANGELOG.md and commit it
# 2. tag the release
gix version bump minor
# 3. push it
git push origin v0.3.0
```

Releases are cut from `main`. `gix version bump` refuses to run from a
feature branch, so you can't tag a work-in-progress branch by accident.

### Prereleases

```bash
gix version bump major --pre rc --dry-run   # 0.9.0 -> 1.0.0-rc.1
gix version bump major --pre rc             # creates v1.0.0-rc.1
gix version bump --pre rc                   # creates v1.0.0-rc.2
gix version bump --release                  # creates v1.0.0
```

A prerelease tag is a normal tag: gix does not push it, and you push it
yourself. Promoting is explicit (`--release`): `gix version bump minor`
on `1.0.0-rc.1` goes to `1.1.0`, not to `1.0.0`. In `CHANGELOG.md`,
head a prerelease `## [1.0.0-rc.1]`, so it stays apart from `## [1.0.0]`
and the release workflow finds it.

Pushing a prerelease tag runs the same release workflow as a stable
tag. It publishes the GitHub Release marked as a prerelease and does not
move the Docker `latest` tag.

## Something looks wrong

Start with `gix doctor` — it checks the flow setup, HEAD, the working
tree, remotes, upstream, your Git identity, signing, and whether the
base branches `.gix/config` names really exist:

```bash
gix doctor
```

It only reports, so it's safe to run while you work out the problem.
For the identity, it will tell you the exact command to set it:

```bash
gix doctor --fix --set-user-name "Your Name" --set-user-email "you@example.com"
```

If the same identity is wanted in every repository:

```bash
gix doctor --fix --scope global --set-user-name "Your Name" --set-user-email "you@example.com"
```

## Signing

```bash
gix signing status   # format, key, whether commits/tags are signed, availability
```

`gix signing status` reads `gpg.format`, `user.signingkey`,
`commit.gpgsign`, `tag.gpgsign`, and `gpg.program`, and checks the
signing program is on `PATH`. It never signs anything and never prints
key material.

## See what gix is actually doing

```bash
gix status --verbose   # prints every git command gix runs
gix status --debug     # + raw git output and timing
```
