#!/usr/bin/env bash
# check-native-dependency-surface.sh [--gc-binary PATH]
#
# Guards the native beads dependency surface: module-graph size and per-family
# module counts (from go.sum, run from the repository root), and the gc binary
# (no product-metrics testhook, size cap).
#
# Without --gc-binary the script builds the release-shaped binary itself
# (`CGO_ENABLED=0 go build -trimpath ./cmd/gc`, what max_binary_bytes below is
# measured against). The Bazel sh_test //scripts:check_native_dependency_surface_test
# passes the hermetically built //cmd/gc instead (cgo, so a larger binary) and
# sets its own GC_NATIVE_DEP_MAX_BINARY_BYTES.
set -euo pipefail

gc_binary=""
case "${1:-}" in
"") ;;
--gc-binary)
	[ -n "${2:-}" ] || { echo "usage: $0 [--gc-binary PATH]" >&2; exit 2; }
	gc_binary="$2"
	;;
*)
	echo "usage: $0 [--gc-binary PATH]" >&2
	exit 2
	;;
esac

# max_modules re-baselined 2026-09-01 for beads v1.3.0-rc.1: measured 727 before
# and 737 after, with ten additions and no removals. Re-measured 2026-09-10 on
# the move to v1.3.0-rc.2: still 737, so the cap is carried forward unchanged
# rather than re-derived. Nine are the OpenAPI
# toolchain behind bd's new `bd serve` HTTP API (kin-openapi, oapi-codegen/v2,
# speakeasy-api/{openapi,jsonpath}, oasdiff/{yaml,yaml3}, vmware-labs/yaml-jsonpath,
# dprotaso/go-yit) plus zeebo/errs; the tenth is cloud.google.com/go/pubsub/v2,
# pulled through by the google.golang.org/api bump MVS forced alongside it. None
# of the OpenAPI stack links into gc -- only bd's internal/httpapi/apigen imports
# it, which the root beads package never reaches.
# max_modules re-baselined 2026-09-29 for the beads fork v1.3.0-fd.6 pairing
# (PR #29): measured 761, up from 737. The +24 are the fork's upstream-main
# sync (fd.6, beads #54) bringing the charm.land TUI stack
# (bubbles/bubbletea/glamour/huh/lipgloss v2) and the
# cloud.google.com/go/* service surface its metrics/jira tooling reaches
# through google.golang.org/api. Same shape as the 2026-09-01 re-baseline:
# first-party-driven growth behind the paired bd replace, none of it linked
# into gc itself.
max_modules="${GC_NATIVE_DEP_MAX_MODULES:-761}"
# max_binary_bytes re-baselined 2026-09-29 for the beads fork v1.3.0-fd.6
# pairing (PR #29): measured 180,671,133 on this host. The prior cap of
# 180,000,000 was set 2026-08-29 against a 172.1MB measurement; the +8.6MB is
# the fd.6 fork's upstream-main sync (charm.land TUI stack plus the
# cloud.google.com service surface its metrics tooling reaches), the same
# first-party-driven step the 2026-09-01 module re-baseline recorded. At the
# documented ~90KB/day first-party growth rate, 188,000,000 restores ~81
# days of headroom.
# The 2026-10-05 upstream measurement (180,031,776 bytes) raises the
# binary limit to 190,000,000 while the paired fork retains its module cap.
max_binary_bytes="${GC_NATIVE_DEP_MAX_BINARY_BYTES:-190000000}"
max_aws_modules="${GC_NATIVE_DEP_MAX_AWS_MODULES:-25}"
max_azure_modules="${GC_NATIVE_DEP_MAX_AZURE_MODULES:-9}"
max_dolthub_modules="${GC_NATIVE_DEP_MAX_DOLTHUB_MODULES:-15}"
max_google_api_modules="${GC_NATIVE_DEP_MAX_GOOGLE_API_MODULES:-1}"

# The module graph is read from go.sum rather than `go list -m all`, which
# needs a module cache (and the network to fill it). go.sum holds a checksum
# for every module in the build list (the go command refuses to load the
# graph otherwise), so its distinct module paths plus the main module are an
# upper bound that equals `go list -m all` on a tidy go.sum; a stale entry
# can only over-count, failing loud until `go mod tidy`.
if [ ! -s go.sum ]; then
	echo "native dependency guard: go.sum not found in $(pwd); run from the repository root" >&2
	exit 1
fi
modules="$(awk 'NF >= 3 {print $1 " "}' go.sum | sort -u)"
total_modules="$(( $(printf '%s\n' "$modules" | sed '/^$/d' | wc -l | tr -d ' ') + 1 ))"
if [ "$total_modules" -gt "$max_modules" ]; then
	echo "native dependency guard: module graph has $total_modules modules; max is $max_modules" >&2
	exit 1
fi

beads_module="github.com/steveyegge/beads"
if [ -f deps.env ]; then
	fork_module="$(sed -n 's/^BD_FORK_MODULE=//p' deps.env)"
	fork_version="$(sed -n 's/^BD_FORK_VERSION=//p' deps.env)"
	if [ -n "$fork_module" ] || [ -n "$fork_version" ]; then
		if [ -z "$fork_module" ] || [ -z "$fork_version" ]; then
			echo "native dependency guard: BD_FORK_MODULE and BD_FORK_VERSION must be paired" >&2
			exit 1
		fi
		# A replaced module is recorded under its replacement path in go.sum.
		# Keep the same one-Beads-module limit for the declared fork release.
		beads_module="$fork_module"
		if ! awk -v module="$fork_module" -v version="$fork_version" '
			$1 == module && ($2 == version || $2 == version "/go.mod") {found = 1}
			END {exit !found}
		' go.sum; then
			echo "native dependency guard: go.sum is missing paired module $fork_module $fork_version" >&2
			exit 1
		fi
	fi
fi

counts="$(printf '%s\n' "$modules" | awk -v beads_module="$beads_module" '
	/^github.com\/aws\/aws-sdk-go-v2( |\/)/ {aws++}
	/^github.com\/Azure\/azure-sdk-for-go( |\/)/ {azure++}
	/^github.com\/dolthub\// {dolthub++}
	$1 == "github.com/steveyegge/beads" || $1 == beads_module {beads++}
	/^google\.golang\.org\/api / {googleapi++}
	END {
		printf "aws=%d azure=%d dolthub=%d beads=%d googleapi=%d\n",
			aws, azure, dolthub, beads, googleapi
	}
')"
eval "$counts"

if [ "${beads:-0}" -ne 1 ]; then
	echo "native dependency guard: expected exactly one beads module ($beads_module), got ${beads:-0}" >&2
	exit 1
fi
if [ "${aws:-0}" -gt "$max_aws_modules" ]; then
	echo "native dependency guard: AWS SDK module count ${aws:-0} exceeds $max_aws_modules" >&2
	exit 1
fi
if [ "${azure:-0}" -gt "$max_azure_modules" ]; then
	echo "native dependency guard: Azure SDK module count ${azure:-0} exceeds $max_azure_modules" >&2
	exit 1
fi
if [ "${dolthub:-0}" -gt "$max_dolthub_modules" ]; then
	echo "native dependency guard: DoltHub module count ${dolthub:-0} exceeds $max_dolthub_modules" >&2
	exit 1
fi
if [ "${googleapi:-0}" -gt "$max_google_api_modules" ]; then
	echo "native dependency guard: google.golang.org/api count ${googleapi:-0} exceeds $max_google_api_modules" >&2
	exit 1
fi

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT INT TERM HUP
if [ -n "$gc_binary" ]; then
	[ -x "$gc_binary" ] || { echo "native dependency guard: --gc-binary $gc_binary is not executable" >&2; exit 1; }
	cp -f "$gc_binary" "$tmpdir/gc"
else
	CGO_ENABLED=0 go build -trimpath -o "$tmpdir/gc" ./cmd/gc
fi

# Symbol names live in the binary itself (the ELF symbol table, and the
# pclntab for every linked function), so a byte search finds any linked
# testhook symbol without `go tool nm` and the Go toolchain it needs.
for forbidden_symbol in \
	"main.runProductMetricsTesthookChild" \
	"main.newProductMetricsTesthookRecordHelpCommand" \
	"internal/productmetrics.OpenTesthook" \
	"internal/productmetrics.testhookLoopbackHost"
do
	if LC_ALL=C grep -aFq -- "$forbidden_symbol" "$tmpdir/gc"; then
		echo "native dependency guard: normal gc contains product-metrics testhook symbol $forbidden_symbol" >&2
		exit 1
	fi
done

for forbidden_literal in \
	"GC_PRODUCT_METRICS_TESTHOOK_ENDPOINT" \
	"GC_PRODUCT_METRICS_TESTHOOK_CA_FILE" \
	"__testhook-record-help"
do
	if LC_ALL=C grep -aFq -- "$forbidden_literal" "$tmpdir/gc"; then
		echo "native dependency guard: normal gc contains product-metrics testhook literal $forbidden_literal" >&2
		exit 1
	fi
done

mkdir -p "$tmpdir/home" "$tmpdir/gc-home"
if ! HOME="$tmpdir/home" GC_HOME="$tmpdir/gc-home" \
	"$tmpdir/gc" metrics --help > "$tmpdir/metrics-help.txt" 2>&1; then
	echo "native dependency guard: normal gc metrics --help failed" >&2
	cat "$tmpdir/metrics-help.txt" >&2
	exit 1
fi
if grep -Fq -- "__testhook-record-help" "$tmpdir/metrics-help.txt"; then
	echo "native dependency guard: normal gc metrics --help exposes the product-metrics testhook command" >&2
	exit 1
fi

binary_bytes="$(wc -c < "$tmpdir/gc" | tr -d ' ')"
# Under `bazel coverage` (COVERAGE=1, COVERAGE_DIR set for every test) the
# //cmd/gc this test receives is coverage-instrumented whenever the
# instrumentation filter matches it (the nightly's `coverage //...` uses the
# default "^//"), so its size says nothing about the shipped binary. Every
# other check above still runs; only the size cap is skipped.
if [ "${COVERAGE:-}" = "1" ] || [ -n "${COVERAGE_DIR:-}" ]; then
	echo "native dependency guard: coverage build (COVERAGE_DIR set); skipping binary size cap ($binary_bytes bytes)"
elif [ "$binary_bytes" -gt "$max_binary_bytes" ]; then
	echo "native dependency guard: gc binary is $binary_bytes bytes; max is $max_binary_bytes" >&2
	exit 1
fi

echo "native dependency guard: modules=$total_modules aws=${aws:-0} azure=${azure:-0} dolthub=${dolthub:-0} googleapi=${googleapi:-0} binary_bytes=$binary_bytes"
