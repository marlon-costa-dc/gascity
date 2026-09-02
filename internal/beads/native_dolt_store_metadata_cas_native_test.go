//go:build beads_rowlock

// Conformance suites that require NativeDoltStore to be CAS-capable. Both
// entrypoints assert the store implements beads.MetadataCASWriter /
// beads.ConditionalWriter, which holds only when
// native_dolt_store_conditional.go is compiled. See beads gc-5oauf.

package beads_test

import (
	"testing"

	"github.com/gastownhall/gascity/internal/beads"
	"github.com/gastownhall/gascity/internal/beads/beadstest"
)

// TestNativeDoltStoreMetadataCASConformance holds NativeDoltStore to the
// complete value-CAS contract. The in-memory native fixture serializes
// transaction callbacks so its contention behavior matches real Dolt.
func TestNativeDoltStoreMetadataCASConformance(t *testing.T) {
	beadstest.RunMetadataCASConformance(t, "NativeDoltStore",
		func(_ *testing.T) beads.Store { return beads.NewNativeDoltStoreForConformance() })
}

func TestNativeDoltStoreConditionalWriterConformance(t *testing.T) {
	beadstest.RunConditionalWriterConformanceWithOptions(t, "NativeDoltStore",
		func(_ *testing.T) beads.Store { return beads.NewNativeDoltStoreForConformance() },
		beadstest.ConditionalWriterOptions{
			RowBackedMutationFlavors: true,
			RestrictedUpdateFields:   true,
			SuppliesCurrent:          true,
		},
	)
}
