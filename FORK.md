# This fork

This repository is a fork of its GitHub parent. Its integration branch (the
repository's default branch) tracks the parent's default branch, plus a small
set of fork plumbing and a few product patches. Fork releases are prereleases
named `vX.Y.Z-<label>.N`: `vX.Y.Z` is the newest upstream release the branch
contains, `<label>` is this fork's release label, and the tag annotation
records the upstream commit the release is built on.

## Policy

- **Product code equals upstream plus named patches.** `git diff $(git merge-base
  upstream/<branch> origin/<default-branch>) origin/<default-branch>` lists only
  fork plumbing (this file, `scripts/fork/`, the fork release workflow, the
  integration branch in workflow triggers) and the product patches below.
- **A product patch needs a reason.** Each one is developed on its own
  `feat/<slug>` branch from the integration branch, as one surgical commit that
  can become an upstream pull request, and exists only for a defect or need we
  hit at runtime with no upstream fix. Refactors, style changes and dead-code
  removal of upstream code go upstream, never here.
- **Upstream's own mechanisms come first.** Before patching, look for an
  existing configuration key, environment variable, extension point or
  upstream commit.

## Sync with upstream

```bash
scripts/fork/fork.sh sync
```

`sync` does the following:
1. Fetches the parent's default branch and creates
   `lane/<default-branch>-<upstream-branch>-<sha>` in a linked worktree next to
   the other worktrees.
2. Branches from that upstream commit and merges the default branch in with
   `-s ours`, so the tree equals upstream and the PR merges completely.
3. Re-applies the fork delta (`git diff <last-synced-upstream> origin/<default-branch>`)
   with `git apply --3way`. A conflict stops the script.
4. Refreshes the bd pairing, where this repository has one.
5. Pushes the lane and opens a draft PR.

When the default branch already contains the upstream head, `sync` prints
`already at upstream <branch> <sha>` and changes nothing.

## Release

After the PR is merged:

```bash
scripts/fork/fork.sh release
```

This tags the merged head as the next `<base>-<label>.N`, records the synced
upstream commit in the tag annotation, and pushes the tag. The fork release
workflow then builds and publishes the prerelease.

## bd ↔ gc pairing

Gas City checks that the `bd` CLI version equals the version of the beads
library it links, as an exact string. In the gascity fork,
`scripts/fork/fork.sh pair [<bd-fork-tag>]` does the pairing:
- it points `go.mod`'s `replace` for `github.com/steveyegge/beads` at the bd
  fork release, by default the newest one for the linked version;
- it refreshes `go.sum`;
- it records `BD_FORK_MODULE` and `BD_FORK_VERSION` in `deps.env`.

`scripts/check-gomod-replace.sh` accepts exactly that replace and nothing
else. Release the bd fork first, then pair and release gc.
