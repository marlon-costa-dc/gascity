//go:build !beads_rowlock

package beads

import (
	"context"
	"fmt"

	beadslib "github.com/steveyegge/beads"
)

// Default build: the city pins the beads v1.2.2 library line, whose embedded
// migrations top out at 0053 — matching every live store. That line has no
// Issue.RowVersion field and no WorkFilter.Statuses, so this file supplies the
// shims the native store calls.
//
// This is not a downgrade of working behavior. Issue.RowVersion is backed by
// issues.row_lock, a column migration 0054 creates; at schema 53 the column
// does not exist, so the CAS fence could not function even if the symbols were
// present. beads.conditional_writes is `off` accordingly.
//
// Build with -tags beads_rowlock against a library line >= 0054 to restore the
// fence and the single-call ready query. Doing so migrates every database
// forward on open and is one-way — see beads gc-5oauf.

// setIssueRowVersion is a no-op: the 1.2.2 line has no CAS token to stamp.
func setIssueRowVersion(_ *beadslib.Issue, _ int64) {}

// issueRowVersion reports 0, the zero CAS token. Callers compare it against a
// caller-supplied expected revision; conditional writes are unavailable in this
// build (ConditionalWriterFor returns false), so no comparison reaches here.
func issueRowVersion(_ *beadslib.Issue) int64 { return 0 }

// getReadyWorkForOpenStatuses queries each open-class backing status in turn,
// because WorkFilter on this library line carries a single Status rather than a
// set. Results are concatenated in status order; the caller de-duplicates by ID
// and applies the gc-side post-filter, so the extra rows are harmless.
//
// This costs one round trip per status (~30-70ms per Ready() on a live
// server-mode store versus the single-call path). It remains cheaper than the
// BdStore fallback it replaces, which forks a bd process per operation.
func getReadyWorkForOpenStatuses(
	ctx context.Context,
	storage beadslib.Storage,
	base beadslib.WorkFilter,
) ([]*beadslib.Issue, error) {
	var out []*beadslib.Issue
	for _, status := range nativeDoltOpenReadyStatuses {
		filter := base
		filter.Status = status
		issues, err := storage.GetReadyWork(ctx, filter)
		if err != nil {
			return nil, err
		}
		out = append(out, issues...)
	}
	return out, nil
}

// blockedBatchQuerier is the local shape of the library's BlockedQuerier: a
// storage that can answer the denormalized, transitive is_blocked column in one
// batched read.
//
// The 1.2.2 library line exports no AsBlockedQuerier helper, so this build
// asserts the method set directly. That is equivalent for every backing that
// actually implements it — including the real Dolt storage — and keeps the
// projection working on the pinned line.
type blockedBatchQuerier interface {
	IsBlockedBatch(ctx context.Context, ids []string) (map[string]bool, error)
}

// isBlockedBatchForStorage serves the ready projection's IsBlocked column when
// the backing can answer it, and reports the named degraded state when it
// cannot.
//
// ErrReadyProjectionUnsupported is a state the cache already handles: the ROWS
// stay whole, so List/Get/DepList keep serving from cache, and cachedBeadReady
// derives readiness from the bead's own status instead of the projected column.
func isBlockedBatchForStorage(
	ctx context.Context,
	storage beadslib.Storage,
	ids []string,
) (map[string]bool, error) {
	querier, ok := storage.(blockedBatchQuerier)
	if !ok {
		return nil, fmt.Errorf("native ready projection: %w: storage %T does not expose beads.BlockedQuerier",
			ErrReadyProjectionUnsupported, storage)
	}
	blocked, err := querier.IsBlockedBatch(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("native ready projection: %w", err)
	}
	return blocked, nil
}
