package main

import (
	"context"
	"fmt"
)

// same three-signal safety contract as the all-orphans path: identifier
// validation, registered-owner (rig protection) check, and live-session probe
// (FAIL-CLOSED). It bypasses the stale-prefix scan entirely — the operator has
// identified a specific orphan database that none of the built-in prefixes
// match.
//
// In dry-run mode (no --force) the would-be target is projected into
// report.Dropped.Names so the operator can confirm before destruction.
// In --force mode the database is actually dropped via DropDatabase.
//
// Returns false when the target is refused (invalid identifier, registered
// owner, not found, live sessions, or a destructive-stage error). Returns
// true when the drop is allowed to proceed or when the caller cannot act
// (no DoltClient) but there is no refusal of the target itself — mirroring
// runDropStage's contract so purge/reap stages still get a chance to run.
func runExactTargetDrop(report *CleanupReport, opts cleanupOptions) bool {
	target := opts.ExactTarget

	// Identifier guard: reject unsafe identifiers before any I/O.
	// This is the first line of defense against injection — the name
	// flows into DROP DATABASE and USE statements.
	if !validDoltDatabaseIdentifier(target) {
		recordCleanupErrorKind(report, "exact-target", cleanupErrorKindExactTargetInvalidID, target,
			fmt.Errorf("invalid database identifier %q for --exact-target", target))
		return false
	}

	// When the SQL client could not be opened, we cannot check existence
	// or registered ownership. Record the underlying open error (same as
	// runDropStage) and let runDoltCleanup's exit-code logic surface it.
	if opts.DoltClient == nil {
		if opts.DoltClientOpenErr != nil {
			recordCleanupError(report, "exact-target", target, opts.DoltClientOpenErr)
		}
		return true
	}

	// Fail-closed on rig-protection errors: if rig metadata is unreadable
	// or corrupt, we cannot prove the target is not a registered rig DB.
	if opts.Force && hasRigProtectionError(report) {
		return true
	}

	// Live-session probe (FAIL-CLOSED): if the probe cannot complete,
	// refuse to drop. The probe error is recorded both as a force-blocker
	// (so dry-run surfaces it) and as a stage error (so --force refuses).
	probeCtx, probeCancel := context.WithTimeout(context.Background(), cleanupLiveSessionProbeTimeout)
	liveSessions, probeErr := opts.DoltClient.ProbeLiveSessions(probeCtx)
	probeCancel()
	if probeErr != nil {
		recordCleanupForceBlocker(report, cleanupErrorKindLiveSessionProbeFailed, target, probeErr)
		if opts.Force {
			recordCleanupErrorKind(report, "drop", cleanupErrorKindLiveSessionProbeFailed, target, probeErr)
			return false
		}
		// Dry-run: fall through so the existence/owner checks still
		// surface in the report alongside the force-blocker.
	}

	// Existence guard: the target must exist on the server.
	listCtx, listCancel := context.WithTimeout(context.Background(), cleanupListTimeout)
	allDBs, err := opts.DoltClient.ListDatabases(listCtx)
	listCancel()
	if err != nil {
		recordCleanupError(report, "exact-target", target, err)
		return false
	}

	exists := false
	for _, name := range allDBs {
		if name == target {
			exists = true
			break
		}
	}
	if !exists {
		recordCleanupErrorKind(report, "exact-target", cleanupErrorKindExactTargetNotFound, target,
			fmt.Errorf("database %q does not exist on the Dolt server", target))
		return false
	}

	// Registered-owner guard: the target must not be a registered rig DB.
	for _, rp := range report.RigsProtected {
		if rp.DB == target {
			recordCleanupErrorKind(report, "exact-target", cleanupErrorKindExactTargetRegistered, target,
				fmt.Errorf("database %q is a registered rig database (%s); refusing exact-target drop", target, rp.Rig))
			return false
		}
	}

	// Live-session guard for the specific target.
	if probeErr == nil {
		if count, live := liveSessions[target]; live {
			report.Dropped.Skipped = append(report.Dropped.Skipped, DoltDropSkip{
				Name:   target,
				Reason: DropSkipReasonLiveSession,
			})
			_ = count
			return false
		}
	}

	// Dry-run: project the would-be target.
	if !opts.Force {
		report.Dropped.Count = 1
		report.Dropped.Names = []string{target}
		return true
	}

	// Force: actually drop.
	dropCtx, dropCancel := context.WithTimeout(context.Background(), cleanupDropTimeout)
	dropErr := opts.DoltClient.DropDatabase(dropCtx, target)
	dropCancel()
	if dropErr != nil {
		report.Dropped.Failed = append(report.Dropped.Failed, CleanupDropFailure{
			Name:  target,
			Error: dropErr.Error(),
		})
		report.Errors = append(report.Errors, CleanupError{
			Stage: "drop",
			Name:  target,
			Error: dropErr.Error(),
		})
		report.Summary.ErrorsTotal++
		return false
	}

	report.Dropped.Count = 1
	report.Dropped.Names = append(report.Dropped.Names, target)
	return true
}
