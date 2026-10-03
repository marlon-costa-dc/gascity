#!/usr/bin/env bash
# check-gomod-replace.sh [go.mod-path]
#
# Fails if go.mod contains any replace directive targeting an unreleased
# version: pseudo-version, local filesystem path, or git branch/ref.
#
# Policy: gascity is a public project. It must only pin released semver tags.
# The only override is an explicit human-operator decision (e.g. an emergency
# security fix from an unreleased commit). That override is a manual admin
# bypass of this required CI check — automated workers may NEVER self-authorize
# an unreleased dependency.
#
# Sanctioned exception (operator 2026-10-03, disaster deployment): the embedded
# beads library line is redirected from github.com/steveyegge/beads to the
# fleet fork github.com/marlon-costa-dc/beads at a released fork tag
# vX.Y.Z-fd.N. Upstream's 1.3.x line ships embedded migrations only to 0066
# (schema v66); the fleet's standalone bd 1.3.0-fd.6 migrated every shared
# tracker store to schema v69, so only the fork line can open them. This one
# redirect is admitted here and by the tree-side arm
# TestModuleGraphReplaceDirectivesAreSanctioned; every other module, local
# path, pseudo-version, and plain prerelease stays blocked.
#
# Released: exactly vX.Y.Z where X, Y, Z are integers (e.g. v1.0.5, v0.0.1).
# Blocked: pseudo-version, prerelease label, local path, git branch/ref, or
#          any non-semver version token.
#
# Handles both single-line and grouped multi-line replace blocks:
#   replace foo => bar v1.0.0-pseudo          (single-line)
#   replace (                                 (grouped block)
#       foo => bar v1.0.0-pseudo
#   )
set -euo pipefail

gomod="${1:-go.mod}"

if [[ ! -f "$gomod" ]]; then
	echo "check-gomod-replace: $gomod not found" >&2
	exit 1
fi

check_replace_rhs() {
	local stripped="$1" rhs="$2"
	local version="" path_part="$rhs"

	# Strip an inline `// comment` from the RHS before splitting; otherwise the
	# trailing comment tokens defeat the two-token path/version match below and
	# an unreleased version slips through (e.g. `=> mod v1.0.5-pseudo // why`).
	if [[ "$rhs" == *"//"* ]]; then
		rhs="${rhs%%//*}"
	fi
	rhs="${rhs%"${rhs##*[![:space:]]}"}"   # trim trailing whitespace
	path_part="$rhs"

	# Split into path and optional version (last space-separated token).
	if [[ "$rhs" =~ ^([^ ]+)[[:space:]]+([^ ]+)$ ]]; then
		path_part="${BASH_REMATCH[1]}"
		version="${BASH_REMATCH[2]}"
	fi

	# Local filesystem paths are always unreleased.
	if [[ "$path_part" == ./* || "$path_part" == ../* || "$path_part" == /* ]]; then
		echo "check-gomod-replace: BLOCKED — replace directive targets a local path:" >&2
		echo "  $stripped" >&2
		echo "" >&2
		echo "  Policy: gascity is a public project that must only pin released semver deps." >&2
		echo "  Local-path replaces (./  ../  /) may not appear in committed go.mod." >&2
		echo "  Override: human operator must manually bypass this required CI check." >&2
		return 1
	fi

	# No version: path-only redirect with no version to check.
	[[ -n "$version" ]] || return 0

	# Only pure vX.Y.Z release tags are allowed. Everything else — pseudo-versions
	# (timestamp+sha suffix), prerelease labels (-rc1, -beta), and non-semver
	# tokens (branch names like "main", git refs) — is blocked. The one
	# exception is the sanctioned beads-line redirect (see header): the fleet
	# fork's release tags vMajor.Minor.Patch-fd.N.
	if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
		if is_sanctioned_beads_replace "$stripped" "$version"; then
			return 0
		fi
		echo "check-gomod-replace: BLOCKED — replace directive targets an unreleased version:" >&2
		echo "  $stripped" >&2
		echo "" >&2
		echo "  Policy: gascity is a public project that must only pin released semver deps." >&2
		echo "  Only exact vX.Y.Z release tags are allowed; pseudo-versions, prerelease" >&2
		echo "  labels, and git branch/ref tokens are not. Version seen: $version" >&2
		echo "  Override: human operator must manually bypass this required CI check." >&2
		return 1
	fi

	return 0
}

# is_sanctioned_beads_replace reports whether a replace line is the one
# operator-sanctioned redirect: github.com/steveyegge/beads (no old-side
# version) => github.com/marlon-costa-dc/beads at a released fork tag
# vX.Y.Z-fd.N. Every other old/new path or version shape is refused here.
is_sanctioned_beads_replace() {
	local stripped="$1" version="$2"
	local lhs="${stripped%%=>*}"
	lhs="${lhs#"${lhs%%[![:space:]]*}"}"
	lhs="${lhs%"${lhs##*[![:space:]]}"}"
	# Single-line form still carries the "replace" verb; the block form does not.
	[[ "$lhs" == "replace "* ]] && lhs="${lhs#replace }"
	lhs="${lhs#"${lhs%%[![:space:]]*}"}"
	[[ "$lhs" == "github.com/steveyegge/beads" ]] || return 1
	local rhs_path
	# Recompute the RHS path from the line: version alone is the last token.
	local rhs="${stripped#*=>}"
	rhs="${rhs%%//*}"
	rhs="${rhs#"${rhs%%[![:space:]]*}"}"
	rhs="${rhs%"${rhs##*[![:space:]]}"}"
	if [[ "$rhs" =~ ^([^[:space:]]+)[[:space:]]+([^[:space:]]+)$ ]]; then
		rhs_path="${BASH_REMATCH[1]}"
		[[ "${BASH_REMATCH[2]}" == "$version" ]] || return 1
	fi
	[[ "$rhs_path" == "github.com/marlon-costa-dc/beads" ]] || return 1
	[[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+-fd\.[0-9]+$ ]]
}

failed=0
in_replace_block=0
while IFS= read -r line; do
	stripped="${line#"${line%%[! ]*}"}"

	# Detect opening of a grouped block: replace (
	if [[ "$stripped" == "replace (" ]]; then
		in_replace_block=1
		continue
	fi

	# Detect closing of a grouped block.
	if [[ $in_replace_block -eq 1 && "$stripped" == ")" ]]; then
		in_replace_block=0
		continue
	fi

	if [[ $in_replace_block -eq 1 ]]; then
		# Inner line of a block: "old [version] => new [version]"
		[[ -z "$stripped" || "$stripped" == //* ]] && continue
		[[ "$stripped" == *"=>"* ]] || continue
		rhs="${stripped#*=>}"
		rhs="${rhs#"${rhs%%[! ]*}"}"
		check_replace_rhs "$stripped" "$rhs" || failed=1
		continue
	fi

	# Single-line replace: starts with "replace " but is not "replace ("
	[[ "$stripped" == replace\ * && "$stripped" != "replace (" ]] || continue
	[[ "$stripped" == *"=>"* ]] || continue
	rhs="${stripped#*=>}"
	rhs="${rhs#"${rhs%%[! ]*}"}"
	check_replace_rhs "$stripped" "$rhs" || failed=1
done < "$gomod"

if [[ $failed -ne 0 ]]; then
	exit 1
fi

echo "check-gomod-replace: OK (no unreleased replace directives)"
