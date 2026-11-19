package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSupervisorDetermineUnitOwnership exercises
// supervisorDetermineUnitOwnership, which compares a live supervisor PID
// against gc's own systemd user unit for `gc supervisor status` and the
// doctor check (ga-9pjtoy). "outside_unit" covers both ways the unit can
// fail to own the live process: the unit is inactive, or it is active but
// tracking a different MainPID.
func TestSupervisorDetermineUnitOwnership(t *testing.T) {
	const livePID = 4242

	cases := []struct {
		name       string
		writeUnit  bool
		unitActive bool
		mainPID    int
		mainPIDOK  bool
		wantStatus string
	}{
		{
			name:       "no unit installed",
			writeUnit:  false,
			wantStatus: "no_unit",
		},
		{
			name:       "unit installed, active, MainPID matches live supervisor",
			writeUnit:  true,
			unitActive: true,
			mainPID:    livePID,
			mainPIDOK:  true,
			wantStatus: "owned",
		},
		{
			name:       "unit installed but inactive",
			writeUnit:  true,
			unitActive: false,
			mainPID:    0,
			mainPIDOK:  false,
			wantStatus: "outside_unit",
		},
		{
			name:       "unit installed and active but MainPID mismatch",
			writeUnit:  true,
			unitActive: true,
			mainPID:    9999,
			mainPIDOK:  true,
			wantStatus: "outside_unit",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			homeDir := t.TempDir()
			t.Setenv("HOME", homeDir)

			unitPath := supervisorSystemdServicePath()
			if tc.writeUnit {
				if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(unitPath, []byte("[Unit]\nDescription=test\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			oldActive := supervisorSystemctlActive
			oldMainPID := supervisorSystemctlMainPID
			supervisorSystemctlActive = func(_ string) bool { return tc.unitActive }
			supervisorSystemctlMainPID = func(_ string) (int, bool) { return tc.mainPID, tc.mainPIDOK }
			t.Cleanup(func() {
				supervisorSystemctlActive = oldActive
				supervisorSystemctlMainPID = oldMainPID
			})

			got := supervisorDetermineUnitOwnership(livePID)
			if got.Status != tc.wantStatus {
				t.Fatalf("supervisorDetermineUnitOwnership(%d) = %+v, want Status %q", livePID, got, tc.wantStatus)
			}
			if tc.writeUnit && got.Unit == "" {
				t.Fatalf("supervisorDetermineUnitOwnership(%d) = %+v, want non-empty Unit when a unit is installed", livePID, got)
			}
		})
	}
}

// installFakeDelegatedSystemctlShow installs a PATH systemctl shim that
// answers is-active with isActiveExit and `show ... --property=MainPID`
// with mainPID, recording argv.
func installFakeDelegatedSystemctlShow(t *testing.T, isActiveExit int, mainPID string) string {
	t.Helper()
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "systemctl-args")
	script := fmt.Sprintf("#!/bin/sh\necho \"$@\" >> %q\ncase \" $* \" in *\" is-active \"*) exit %d ;; *\" show \"*) echo %q ;; esac\nexit 0\n", argsFile, isActiveExit, mainPID)
	if err := os.WriteFile(filepath.Join(dir, "systemctl"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsFile
}

// TestSupervisorDetermineUnitOwnershipDelegated proves that under
// GC_SUPERVISOR_SYSTEMD_UNIT the operator-managed unit is resolved through
// systemd at its scope, wherever its unit file lives — not by statting gc's
// own unit path, which reported "no_unit" for a unit installed under
// ~/.config/systemd/user.
func TestSupervisorDetermineUnitOwnershipDelegated(t *testing.T) {
	const livePID = 4242
	cases := []struct {
		name         string
		scope        string
		isActiveExit int
		mainPID      string
		wantStatus   string
		wantArg      string
	}{
		{"user unit owns live PID", "user", 0, "4242", "owned", "--user show gascity-supervisor.service --property=MainPID --value"},
		{"system unit owns live PID", "system", 0, "4242", "owned", "show gascity-supervisor.service --property=MainPID --value"},
		{"unit active, MainPID mismatch", "user", 0, "9999", "outside_unit", "--user show"},
		{"unit inactive", "user", 3, "0", "outside_unit", "--user is-active --quiet gascity-supervisor.service"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir()) // gc's own unit path is absent
			setDelegationEnvForTest(t, "gascity-supervisor.service", tc.scope)
			argsFile := installFakeDelegatedSystemctlShow(t, tc.isActiveExit, tc.mainPID)

			got := supervisorDetermineUnitOwnership(livePID)
			if got.Status != tc.wantStatus || got.Unit != "gascity-supervisor.service" {
				t.Fatalf("supervisorDetermineUnitOwnership(%d) = %+v, want Status %q on the delegated unit", livePID, got, tc.wantStatus)
			}
			args := strings.Join(readRecordedSystemctlArgs(t, argsFile), "\n")
			if !strings.Contains(args, tc.wantArg) {
				t.Fatalf("systemctl argv = %q, want an invocation containing %q", args, tc.wantArg)
			}
		})
	}
}

func TestSupervisorDetermineUnitOwnershipDelegationError(t *testing.T) {
	setDelegationEnvForTest(t, "gascity-supervisor.service", "usr")
	got := supervisorDetermineUnitOwnership(4242)
	if got.Status != "delegation_error" || !strings.Contains(got.Error, "usr") {
		t.Fatalf("supervisorDetermineUnitOwnership = %+v, want delegation_error naming the bad scope", got)
	}
}
