//go:build beads_rowlock

package beads

import (
	beadslib "github.com/steveyegge/beads"
)

// workFilterMatchesStatus mirrors the backing store's status selection for
// GetReadyWork spies. On this library line Statuses carries the whole
// open-class set with OR semantics, and singular Status stays honored for
// callers that still set it.
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
