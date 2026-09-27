package main

import (
	"fmt"
	"strings"

	"github.com/gastownhall/gascity/internal/beads"
)

// City + per-rig store checks skipped on outage-shaped preflight; keep in sync with buildDoctorChecks.
const (
	doctorCityStoreCheckCount   = 16
	doctorPerRigStoreCheckCount = 3
)

// City-scoped store probe before store-dependent checks. Tests override.
var doctorBeadStorePreflight = defaultDoctorBeadStorePreflight

// defaultDoctorBeadStorePreflight probes the store the way every store check
// reads it: through the run's memoized factory, whose open IS the proxied
// lane's admission.
//
// It used to fork a raw `bd list` under NoRecovery and a 5s deadline instead.
// That probe has no admission behind it, and admission is the only code that
// can classify a proxied endpoint — and, on the zombie shape (the data port
// accepts and never greets because the Dolt child took SIGTERM), walk its
// budgeted escalation: three no-greeting probes, one provider ping, then one
// provider recover per generation. Gating the store checks on the raw fork
// turned every recoverable zombie into a declared outage: the probe burned
// its deadline on accept-without-greet dials, bead-store-preflight failed as
// a blocking error, and the beads-store check that would have run admission
// and healed the scope was among the checks it gated off — so a doctor run
// on a disturbed city spent its budget in unrelated check timeouts and still
// reported the store unreachable
// (test/acceptance TestProxiedNativeLifecycle/child-term-zombie, root-move).
//
// Routing the probe through the factory keeps the #5064 contract — ONE probe
// per run, because the store this open returns is memoized and every
// store-dependent check reuses it — and the open's budget is the ladder's
// own: spacing sleeps, provider verbs queued on the lifecycle slot, bd's
// per-command timeout. A short caller deadline is deliberately NOT wrapped
// around the open: one that landed between a recover's `bd dolt stop` and
// its re-admitting ping would cut the recover in half, which is exactly the
// state the generation ledger's IssueStop exists to prevent. The post-open
// read is what actually carries the verdict for the stores that construct
// lazily (bd front door, exec): only a read proves the store serves, and its
// ceiling is bd's own per-command timeout.
func defaultDoctorBeadStorePreflight(cityPath string, storeFactory func(string) (beads.Store, error)) error {
	store, err := storeFactory(cityPath)
	if err != nil {
		return err
	}
	_, err = store.List(beads.ListQuery{Status: "open", Limit: 1})
	return err
}

// True for live store outages (breaker/conn/timeout), not missing/uninitialized stores.
//
// Superset of the transport shapes in bdTransportRetryableError
// (cmd/gc/bd_env.go) plus store-pool exhaustion. Deliberately separate:
// that list drives retry, this one drives check omission. Keep them from
// drifting when new bd/Dolt error shapes appear.
func isBeadStoreUnreachable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, sig := range []string{
		"dolt circuit breaker is open",
		"server appears down",
		"dolt server unreachable",
		"dolt server not reachable",
		"max waiting connections",
		"client rejected",
		"too many connections",
		"connection refused",
		"dial tcp",
		"bad connection",
		"invalid connection",
		"connection reset",
		"broken pipe",
		"i/o timeout",
		"timed out after",
		"context deadline exceeded",
		"unexpected eof",
		"use of closed network connection",
	} {
		if strings.Contains(msg, sig) {
			return true
		}
	}
	return false
}

func beadStorePreflightSkipCount(activeRigCount int) int {
	return doctorCityStoreCheckCount + doctorPerRigStoreCheckCount*activeRigCount
}

func beadStorePreflightSkipMessage(skipCount, rigCount int, probeErr error) string {
	// City-scoped probe: skip is a city-outage gate (per-rig endpoints may differ).
	base := fmt.Sprintf(
		"bead store unreachable — skipped %d store checks (%d city, %d rigs); city store was probed (per-rig endpoints, including doltlite, may differ)",
		skipCount, doctorCityStoreCheckCount, rigCount,
	)
	if probeErr == nil {
		return base
	}
	return fmt.Sprintf("%s: %v", base, probeErr)
}
