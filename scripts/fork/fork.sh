#!/usr/bin/env bash
# fork.sh — maintain this fork as an upstream release tree plus fork plumbing.
#
#   scripts/fork/fork.sh sync [<upstream-tag>]  rebuild the integration branch on an
#                                               upstream release and open the PR
#   scripts/fork/fork.sh release                tag the merged integration head as the
#                                               next fork prerelease (<base>-<label>.<N>)
#   scripts/fork/fork.sh pair [<bd-fork-tag>]   link the bd fork release this build
#                                               pairs with (repos that link beads only)
#
# Every fact is derived, never configured: the upstream repository and the
# integration branch come from GitHub (parent, default branch), the base is the
# newest upstream release already merged into the integration branch, and the
# fork label (fd, fc, ...) comes from this fork's own release tags. FORK.md
# documents the full cycle.
set -euo pipefail

die() {
	echo "fork.sh: $*" >&2
	exit 1
}

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

repo_json=$(gh repo view --json nameWithOwner,parent,defaultBranchRef)
self=$(jq -r '.nameWithOwner' <<<"$repo_json")
upstream=$(jq -r 'if .parent then "\(.parent.owner.login)/\(.parent.name)" else "" end' <<<"$repo_json")
integration=$(jq -r '.defaultBranchRef.name' <<<"$repo_json")
[[ -n "$upstream" ]] || die "$self is not a fork on GitHub"
upstream_url="https://github.com/${upstream}.git"

release_re='^v[0-9]+\.[0-9]+\.[0-9]+$'
fork_re='^v[0-9]+\.[0-9]+\.[0-9]+-(f[a-z])\.?([0-9]+)$'

fetch_integration() {
	git fetch --no-tags origin "refs/heads/${integration}:refs/remotes/origin/${integration}"
}

fetch_upstream_tag() {
	git fetch --no-tags "$upstream_url" "refs/tags/$1:refs/tags/$1"
}

# Newest upstream release tag that is already an ancestor of the integration head.
current_base() {
	local tag
	while read -r tag; do
		[[ "$tag" =~ $release_re ]] || continue
		git rev-parse -q --verify "refs/tags/${tag}^{commit}" >/dev/null || fetch_upstream_tag "$tag"
		if git merge-base --is-ancestor "$tag" "origin/${integration}"; then
			echo "$tag"
			return 0
		fi
	done < <(gh release list -R "$upstream" --exclude-pre-releases --limit 100 --json tagName --jq '.[].tagName' | sort -rV)
	die "no upstream release of $upstream is an ancestor of origin/${integration}"
}

# The fork label (fd, fc, ...) used by this fork's existing release tags.
fork_label() {
	local tag
	while read -r tag; do
		if [[ "$tag" =~ $fork_re ]]; then
			echo "${BASH_REMATCH[1]}"
			return 0
		fi
	done < <(gh release list -R "$self" --limit 100 --json tagName --jq '.[].tagName')
	die "no fork release tag of $self matches vX.Y.Z-f<letter>[.]N"
}

cmd_sync() {
	fetch_integration
	local target="${1:-}"
	[[ -n "$target" ]] || target=$(gh release view -R "$upstream" --json tagName --jq .tagName)
	[[ "$target" =~ $release_re ]] || die "upstream tag must be vX.Y.Z, got: $target"
	local base
	base=$(current_base)
	if [[ "$target" == "$base" ]]; then
		echo "fork.sh: already at $target"
		return 0
	fi
	fetch_upstream_tag "$target"

	local main_root lane worktree patch
	main_root=$(dirname "$(git rev-parse --path-format=absolute --git-common-dir)")
	lane="lane/${integration}-${target}"
	worktree="$(dirname "$main_root")/worktrees/$(basename "$main_root")/lane-${integration}-${target}"
	patch=$(mktemp)
	git diff --binary "$base" "origin/${integration}" >"$patch"

	git worktree add -b "$lane" "$worktree" "$target"
	git -C "$worktree" merge --no-ff -s ours "origin/${integration}" \
		-m "merge ${integration} history into the ${target} fork base (tree stays ${target})"
	git -C "$worktree" apply --3way --index "$patch"
	rm -f "$patch"
	git -C "$worktree" commit -m "re-apply the fork plumbing from ${base} onto ${target}"
	if grep -q '^BD_FORK_VERSION=' "$worktree/deps.env" 2>/dev/null; then
		(cd "$worktree" && "$worktree/scripts/fork/fork.sh" pair)
		git -C "$worktree" add go.mod go.sum deps.env
		git -C "$worktree" commit -m "refresh the bd fork pairing on ${target}"
	fi
	git -C "$worktree" push -u origin "$lane"
	gh pr create -R "$self" --base "$integration" --head "$lane" --draft \
		--title "Rebuild ${integration} on upstream ${target}" \
		--body "Upstream ${target} with the fork plumbing re-applied from ${base} (\`git diff ${base} origin/${integration}\`). Created by scripts/fork/fork.sh sync; see FORK.md."
}

cmd_release() {
	fetch_integration
	local base label head n tag
	base=$(current_base)
	label=$(fork_label)
	head=$(git rev-parse "origin/${integration}")
	n=0
	while read -r tag; do
		[[ "$tag" =~ ^${base}-${label}\.([0-9]+)$ ]] || continue
		if ((BASH_REMATCH[1] > n)); then
			n=${BASH_REMATCH[1]}
		fi
	done < <(git ls-remote --tags --refs origin "refs/tags/${base}-${label}.*" | sed 's#.*refs/tags/##')
	tag="${base}-${label}.$((n + 1))"
	echo "fork.sh: ${self} ${integration} ${head} = ${base} + fork plumbing:"
	git diff --stat "$base" "$head"
	git tag -a "$tag" -m "Fork release ${tag} of upstream ${base}" "$head"
	git push origin "refs/tags/${tag}"
	echo "fork.sh: pushed ${tag}; the release workflow publishes it"
}

cmd_pair() {
	[[ -f deps.env ]] || die "pair needs deps.env"
	local module="github.com/steveyegge/beads" required fork_repo tag
	required=$(go list -m -f '{{.Version}}' "$module") || die "go.mod does not require $module"
	# The bd fork is this fork owner's fork of the repository the module lives in.
	local module_repo owner
	module_repo=$(gh api "repos/${module#github.com/}" --jq .full_name)
	owner=${self%%/*}
	fork_repo=$(gh api "repos/${module_repo}/forks" --paginate --jq ".[] | select(.owner.login == \"${owner}\") | .full_name")
	[[ -n "$fork_repo" ]] || die "$owner has no fork of $module_repo"
	tag="${1:-}"
	if [[ -z "$tag" ]]; then
		tag=$(gh release list -R "$fork_repo" --limit 100 --json tagName --jq '.[].tagName' | grep -E "^${required}-f[a-z]\.[0-9]+$" | sort -rV | head -n 1)
	fi
	[[ "$tag" =~ ^${required}-f[a-z]\.[0-9]+$ ]] || die "bd fork tag must be ${required}-f<letter>.N (the linked version), got: ${tag:-none}"
	go mod edit -replace "${module}=github.com/${fork_repo}@${tag}"
	go mod tidy
	local key
	for key in BD_FORK_MODULE BD_FORK_VERSION; do
		sed -i "/^${key}=/d" deps.env
	done
	printf 'BD_FORK_MODULE=github.com/%s\nBD_FORK_VERSION=%s\n' "$fork_repo" "$tag" >>deps.env
	go list -m "$module"
	make check-gomod-replace
}

case "${1:-}" in
sync) shift && cmd_sync "$@" ;;
release) cmd_release ;;
pair) shift && cmd_pair "$@" ;;
*) die "usage: fork.sh sync [<upstream-tag>] | release | pair [<bd-fork-tag>]" ;;
esac
