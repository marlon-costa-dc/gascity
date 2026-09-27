#!/usr/bin/env bash
# Resolves the source the bd "current" cell builds from, as GITHUB_OUTPUT
# lines: repo=<clone URL>, ref=<commit or tag>, ver=<declared version>.
#
# The cell is BD_CURRENT_REF on gastownhall/beads. When deps.env pairs gc with
# a bd fork release (BD_FORK_MODULE/BD_FORK_VERSION, written by
# scripts/fork/fork.sh pair), go.mod links that fork, so the current cell must
# build the same fork release: a bd from any other tree would sit on a
# different schema than the library gc opens the store with.
set -euo pipefail

deps_env="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/deps.env"
value() { sed -n "s/^$1=//p" "$deps_env"; }

ref="$(value BD_CURRENT_REF)"
ver="$(value BD_CURRENT_VERSION)"
if [[ -z "$ref" || -z "$ver" ]]; then
  echo "::error::BD_CURRENT_REF/BD_CURRENT_VERSION missing or empty in deps.env" >&2
  exit 1
fi
repo="https://github.com/gastownhall/beads"

fork_module="$(value BD_FORK_MODULE)"
fork_version="$(value BD_FORK_VERSION)"
if [[ -n "$fork_module" || -n "$fork_version" ]]; then
  if [[ -z "$fork_module" || -z "$fork_version" ]]; then
    echo "::error::deps.env declares only one of BD_FORK_MODULE/BD_FORK_VERSION" >&2
    exit 1
  fi
  repo="https://${fork_module}"
  ref="$fork_version"
  ver="$fork_version"
fi

echo "repo=$repo"
echo "ref=$ref"
echo "ver=$ver"
