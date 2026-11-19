package doctor

import "sync"

// Deferred returns a check registered under name whose construction, and any
// I/O that construction needs, happens on its first use. A doctor run that
// selects other checks with --check never pays for building it. The built
// check must report the same name.
//
// A deferred check is never warm-up eligible: deferral exists for checks whose
// construction does I/O, which gc start's warm-up must not pay for.
func Deferred(name string, build func() Check) Check {
	return &deferredCheck{name: name, build: sync.OnceValue(build)}
}

type deferredCheck struct {
	name  string
	build func() Check
}

// Name returns the identifier the check was registered under.
func (d *deferredCheck) Name() string { return d.name }

// Run builds the check on first use and runs it.
func (d *deferredCheck) Run(ctx *CheckContext) *CheckResult { return d.build().Run(ctx) }

// CanFix reports whether the built check supports remediation.
func (d *deferredCheck) CanFix() bool { return d.build().CanFix() }

// Fix runs the built check's remediation.
func (d *deferredCheck) Fix(ctx *CheckContext) error { return d.build().Fix(ctx) }

// WarmupEligible returns false; see Deferred.
func (d *deferredCheck) WarmupEligible() bool { return false }
