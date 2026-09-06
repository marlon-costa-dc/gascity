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

// isBlockedBatchForStorage serves the ready projection from the same canonical
// GetReadyWork query NativeDoltStore.Ready uses. The pinned v1.2.2 Storage
// interface does not expose IsBlockedBatch, including on its real DoltStore,
// but it does require GetReadyWork. Querying both open-class statuses therefore
// preserves the backing's transitive is_blocked verdict without depending on a
// method the production storage cannot implement.
func isBlockedBatchForStorage(
	ctx context.Context,
	storage beadslib.Storage,
	ids []string,
) (map[string]bool, error) {
	issues, err := getReadyWorkForOpenStatuses(ctx, storage, beadslib.WorkFilter{IncludeEphemeral: true})
	if err != nil {
		return nil, fmt.Errorf("native ready projection: %w", err)
	}
	ready := make(map[string]struct{}, len(issues))
	for _, issue := range issues {
		ready[issue.ID] = struct{}{}
	}
	blocked := make(map[string]bool, len(ids))
	for _, id := range ids {
		_, isReady := ready[id]
		blocked[id] = !isReady
	}
	return blocked, nil
}
