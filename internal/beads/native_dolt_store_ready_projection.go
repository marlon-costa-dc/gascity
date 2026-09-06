package beads

import (
	"context"

	beadslib "github.com/steveyegge/beads"
)

// NativeDoltStore hands readiness to the cache — listIncludesCompleteDependencies
// reports true, which latches depsComplete — so it must also supply the column
// that makes the cache's answer equal its own. Losing this method silently
// reverts readiness to the weaker dependency-derived predicate, so pin it.
var _ readyProjectionEnrichmentStore = (*NativeDoltStore)(nil)

// enrichReadyProjectionForCache fills bd's denormalized is_blocked column onto
// items so a cache over this store answers readiness with the same predicate
// the store's own Ready() uses.
//
// It is not an optimization; it is what makes the cache's answer EQUAL the
// backing's. NativeDoltStore.listIncludesCompleteDependencies reports true, so
// a CachingStore over it latches depsComplete and serves readiness itself. With
// the column absent every Bead.IsBlocked is nil — beadFromNativeIssue cannot
// set it, because beadslib's types.Issue carries no is_blocked field — and
// cachedBeadReady then derives readiness from the bead's OWN direct
// blocks/waits-for/conditional-blocks deps. That fallback is weaker than the
// column in two ways that matter to gascity, which creates parent-child edges
// pervasively:
//
//   - is_blocked propagates transitively DOWN parent-child edges
//     (issueops.markBlockedTemplateForIssues joins `d.type = 'parent-child' AND
//     p.is_blocked = 1`), so a child of a blocked parent carries no blocking
//     edge of its own and reads ready to the fallback.
//   - cachedBeadReady treats a dep as blocking only when the target's status is
//     resident in the same cache, so an edge onto another store's row — a
//     relocated `gcg-` graph bead — is invisible to it.
//
// GetReadyWork filters `is_blocked = 0` (sqlbuild.ReadyWhere), so either gap
// offers the control dispatcher a step whose gate has not opened, the
// regression #3218 closed.
//
// The default v1.2.2 build derives the column from the same in-process
// GetReadyWork queries as NativeDoltStore.Ready, because that library's real
// DoltStore does not expose IsBlockedBatch. The row-lock build can use the
// newer library's direct batch projection. Both run inside withReadRetry, so a
// managed-Dolt rebind has the same behavior as every other native read.
func (s *NativeDoltStore) enrichReadyProjectionForCache(items []Bead) ([]Bead, error) {
	if len(items) == 0 {
		return items, nil
	}
	ids := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		// Same exclusions as the bd path: message and nudge rows are
		// notifications rather than dependency-blocked work, and stamping them
		// makes the CachingStore reconciler re-emit bead.updated every cycle.
		// Leaving their IsBlocked nil keeps the reconcile diff convergent.
		if skipBDReadyProjectionEnrichment(item) {
			continue
		}
		if _, ok := seen[item.ID]; ok {
			continue
		}
		seen[item.ID] = struct{}{}
		ids = append(ids, item.ID)
	}
	if len(ids) == 0 {
		return items, nil
	}

	var projection map[string]bool
	err := s.withReadRetry(func(ctx context.Context, storage beadslib.Storage) error {
		blocked, err := isBlockedBatchForStorage(ctx, storage, ids)
		if err != nil {
			return err
		}
		projection = blocked
		return nil
	})
	if err != nil {
		return items, err
	}

	enriched := make([]Bead, len(items))
	copy(enriched, items)
	for i := range enriched {
		if skipBDReadyProjectionEnrichment(enriched[i]) {
			continue
		}
		blocked, ok := projection[enriched[i].ID]
		if !ok {
			// A direct batch query can omit ids that raced out of the ledger.
			// Preserve the last value rather than inventing a verdict.
			continue
		}
		enriched[i].IsBlocked = cloneBoolPtr(&blocked)
	}
	return enriched, nil
}
