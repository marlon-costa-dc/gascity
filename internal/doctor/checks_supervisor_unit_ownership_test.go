package doctor

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestSupervisorUnitOwnershipCheck_Name(t *testing.T) {
	c := NewSupervisorUnitOwnershipCheck(false, 0, SupervisorUnitOwnership{})
	if c.Name() != "supervisor-unit-ownership" {
		t.Errorf("Name() = %q, want %q", c.Name(), "supervisor-unit-ownership")
	}
}

func TestSupervisorUnitOwnershipCheck_CanFix(t *testing.T) {
	c := NewSupervisorUnitOwnershipCheck(false, 0, SupervisorUnitOwnership{})
	if c.CanFix() {
		t.Error("CanFix() = true, want false")
	}
}

func TestSupervisorUnitOwnershipCheck_WarmupEligible(t *testing.T) {
	c := NewSupervisorUnitOwnershipCheck(false, 0, SupervisorUnitOwnership{})
	if c.WarmupEligible() {
		t.Error("WarmupEligible() = true, want false")
	}
}

// TestSupervisorUnitOwnershipCheckRun exercises
// SupervisorUnitOwnershipCheck.Run, the doctor-facing half of ga-9pjtoy: it
// must flag as an error a live supervisor that is not owned by gc's own
// installed systemd unit (unit inactive, or active with the supervisor
// outside its MainPID's process tree), and only warn, naming the probe
// error, when the supervisor's process ancestry could not be read. "No
// supervisor running" and "no unit installed" are OK.
func TestSupervisorUnitOwnershipCheckRun(t *testing.T) {
	const livePID = 4242

	cases := []struct {
		name              string
		supervisorRunning bool
		ownership         SupervisorUnitOwnership
		wantStatus        CheckStatus
	}{
		{
			name:              "supervisor not running",
			supervisorRunning: false,
			ownership:         SupervisorUnitOwnership{Status: "no_unit"},
			wantStatus:        StatusOK,
		},
		{
			name:              "no unit installed",
			supervisorRunning: true,
			ownership:         SupervisorUnitOwnership{Status: "no_unit"},
			wantStatus:        StatusOK,
		},
		{
			name:              "owned by unit",
			supervisorRunning: true,
			ownership: SupervisorUnitOwnership{
				Status:     "owned",
				Unit:       "gascity-supervisor.service",
				UnitActive: true,
				UnitPID:    livePID,
			},
			wantStatus: StatusOK,
		},
		{
			name:              "unit installed but inactive",
			supervisorRunning: true,
			ownership: SupervisorUnitOwnership{
				Status:     "outside_unit",
				Unit:       "gascity-supervisor.service",
				UnitActive: false,
				UnitPID:    0,
			},
			wantStatus: StatusError,
		},
		{
			name:              "unit active but supervisor is not a MainPID descendant",
			supervisorRunning: true,
			ownership: SupervisorUnitOwnership{
				Status:     "outside_unit",
				Unit:       "gascity-supervisor.service",
				UnitActive: true,
				UnitPID:    9999,
			},
			wantStatus: StatusError,
		},
		{
			name:              "unit active but supervisor ancestry unreadable",
			supervisorRunning: true,
			ownership: SupervisorUnitOwnership{
				Status:     "outside_unit",
				Unit:       "gascity-supervisor.service",
				UnitActive: true,
				UnitPID:    9999,
				ProbeErr:   errors.New("reading the parent of PID 4242: open /proc/4242/stat: no such file or directory"),
			},
			wantStatus: StatusWarning,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			check := NewSupervisorUnitOwnershipCheck(tc.supervisorRunning, livePID, tc.ownership)
			result := check.Run(&CheckContext{})

			if result.Status != tc.wantStatus {
				t.Fatalf("Status = %v, want %v (result: %+v)", result.Status, tc.wantStatus, result)
			}
			if tc.wantStatus == StatusOK {
				return
			}
			for _, wantPID := range []int{livePID, tc.ownership.UnitPID} {
				if !strings.Contains(result.Message, fmt.Sprintf("%d", wantPID)) {
					t.Errorf("Message = %q, want it to name PID %d", result.Message, wantPID)
				}
			}
			if tc.wantStatus == StatusWarning {
				if !strings.Contains(result.Message, tc.ownership.ProbeErr.Error()) {
					t.Errorf("Message = %q, want it to name the probe error %q", result.Message, tc.ownership.ProbeErr)
				}
				if result.FixHint != "" {
					t.Errorf("FixHint = %q, want none: an unverified result has no remedy to run", result.FixHint)
				}
				return
			}
			if result.FixHint == "" || !strings.Contains(result.FixHint, "reset-failed") {
				t.Errorf("FixHint = %q, want a remedy mentioning systemctl --user reset-failed", result.FixHint)
			}
		})
	}
}

// A misconfigured systemd delegation env is an error naming the cause, not a
// silent "no unit installed".
func TestSupervisorUnitOwnershipCheckRunDelegationError(t *testing.T) {
	check := NewSupervisorUnitOwnershipCheck(true, 4242, SupervisorUnitOwnership{
		Status: "delegation_error",
		Error:  `invalid GC_SUPERVISOR_SYSTEMD_SCOPE="usr"`,
	})
	result := check.Run(&CheckContext{})
	if result.Status != StatusError {
		t.Fatalf("Status = %v, want StatusError (result: %+v)", result.Status, result)
	}
	if !strings.Contains(result.Message, `GC_SUPERVISOR_SYSTEMD_SCOPE="usr"`) {
		t.Errorf("Message = %q, want the delegation error", result.Message)
	}
}

// A broken delegation env is reported even when no supervisor runs: it is often
// why none does.
func TestSupervisorUnitOwnershipCheckRunDelegationErrorWithoutSupervisor(t *testing.T) {
	r := NewSupervisorUnitOwnershipCheck(false, 0, SupervisorUnitOwnership{Status: "delegation_error", Error: "bad scope"}).Run(&CheckContext{})
	if r.Status != StatusError || !strings.Contains(r.Message, "bad scope") {
		t.Fatalf("result = %d %q, want the delegation error with no supervisor running", r.Status, r.Message)
	}
}

// An active unit whose MainPID could not be read leaves ownership unknown, not
// disproven; a delegated unit's remedy is the caller's, never --user/install.
func TestSupervisorUnitOwnershipCheckRunUnknownMainPIDAndDelegatedRemedy(t *testing.T) {
	r := NewSupervisorUnitOwnershipCheck(true, 4242, SupervisorUnitOwnership{Status: "outside_unit", Unit: "gc.service", UnitActive: true}).Run(&CheckContext{})
	if r.Status != StatusWarning || !strings.Contains(r.Message, "unverified") {
		t.Fatalf("unknown MainPID result = %d %q, want an unverified warning", r.Status, r.Message)
	}
	remedy := "run 'systemctl reset-failed gc.service && systemctl start gc.service' to let the delegated unit reclaim the process"
	r = NewSupervisorUnitOwnershipCheck(true, 4242, SupervisorUnitOwnership{Status: "outside_unit", Unit: "gc.service", UnitActive: true, UnitPID: 9, Remedy: remedy}).Run(&CheckContext{})
	if r.Status != StatusError || r.FixHint != remedy {
		t.Fatalf("delegated result = %d hint %q, want error with the delegated remedy", r.Status, r.FixHint)
	}
}
