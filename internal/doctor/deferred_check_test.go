package doctor

import (
	"io"
	"testing"
)

// A deferred check is selectable by name without being built, and a doctor
// run builds it once, on first use, then runs and fixes the built check.
func TestDeferredCheckBuildsOnFirstUse(t *testing.T) {
	builds := 0
	inner := &mockCheck{name: "lazy", status: StatusError, canFix: true}
	deferred := Deferred("lazy", func() Check {
		builds++
		return inner
	})

	selected, unmatched := SelectChecks([]Check{deferred, &mockCheck{name: "other", status: StatusOK}}, []string{"other"})
	if len(unmatched) != 0 || len(selected) != 1 || selected[0].Name() != "other" {
		t.Fatalf("SelectChecks = %v, %v; want only other", selected, unmatched)
	}
	if builds != 0 {
		t.Fatalf("builds = %d after selecting another check, want 0", builds)
	}
	if deferred.WarmupEligible() || builds != 0 {
		t.Fatalf("WarmupEligible must be false and must not build (builds = %d)", builds)
	}

	d := &Doctor{}
	d.Register(deferred)
	report := d.Run(&CheckContext{}, io.Discard, true)
	if builds != 1 {
		t.Fatalf("builds = %d after a --fix run, want exactly 1", builds)
	}
	if !inner.fixed {
		t.Fatal("the built check's Fix was not reached")
	}
	if report.Passed != 1 || report.Failed != 0 {
		t.Fatalf("report = %d passed / %d failed, want the fixed built check counted as passed", report.Passed, report.Failed)
	}
}
