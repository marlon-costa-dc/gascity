//go:build !beads_rowlock

package beads

import (
	beadslib "github.com/steveyegge/beads"
)

// workFilterMatchesStatus mirrors the backing store's status selection for
// GetReadyWork spies. On the pinned 1.2.2 line WorkFilter carries only the
// singular Status (one-call-per-status query in native_dolt_store_norowlock.go);
// the single-call Ready path carries the whole-open-class set in Statuses.
//
// The beads_rowlock counterpart also honors WorkFilter.Statuses, the
// whole-open-class set that exists only on the newer library line. See beads
// gc-5oauf.
func workFilterMatchesStatus(filter beadslib.WorkFilter, status beadslib.Status) bool {
	if filter.Status != "" {
		return status == filter.Status
	}
	for _, s := range filter.Statuses {
		if status == s {
			return true
		}
	}
	return false
}
