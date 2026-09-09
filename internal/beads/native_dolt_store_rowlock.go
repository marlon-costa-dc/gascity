//go:build beads_rowlock

package beads

import (
	"context"
	"fmt"

	beadslib "github.com/steveyegge/beads"
)

// This file carries the pieces of the native store that depend on beads library
// APIs introduced after the v1.2.2 line: Issue.RowVersion (backed by the
// issues.row_lock column from migration 0054) and WorkFilter.Statuses.
//
// Default builds use the beads_rowlock=off counterpart. See
// native_dolt_store_conditional.go for why the city pins the 1.2.2 line, and
// beads gc-5oauf / gct-83zky for the schema constraint.

// setIssueRowVersion stamps the CAS token onto an issue being handed to the
// library.
func setIssueRowVersion(issue *beadslib.Issue, revision int64) {
	issue.RowVersion = revision
}

// issueRowVersion reads the CAS token off a library issue.
func issueRowVersion(issue *beadslib.Issue) int64 {
	return issue.RowVersion
}

// getReadyWorkForOpenStatuses covers every open-class backing status in ONE
// GetReadyWork call via WorkFilter.Statuses, instead of re-paying the
// deferred-parents pre-query, the wisp arm, and the transaction round trips
// once per status (sr-5rz: ~30-70ms per Ready() on a live server-mode store).
//
// The backing Limit stays 0 because the gc-side post-filter (tier, excluded
// types/labels, defer) discards rows the store cannot, so a server-side limit
// could under-fill the result.
func getReadyWorkForOpenStatuses(
	ctx context.Context,
	storage beadslib.Storage,
	base beadslib.WorkFilter,
) ([]*beadslib.Issue, error) {
	base.Statuses = nativeDoltOpenReadyStatuses
	return storage.GetReadyWork(ctx, base)
}

// isBlockedBatchForStorage serves the ready projection's IsBlocked column from
// the library's batch querier.
func isBlockedBatchForStorage(
	ctx context.Context,
	storage beadslib.Storage,
	ids []string,
) (map[string]bool, error) {
	querier, ok := beadslib.AsBlockedQuerier(storage)
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
