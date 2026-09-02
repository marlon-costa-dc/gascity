package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/gastownhall/gascity/internal/fsys"
)

func TestRunDoltCleanup_ExactTargetDryRunSurfacesOnlyTarget(t *testing.T) {
	client := &fakeCleanupDoltClient{
		databases: []string{"flext_infra", "hs", "testdb_other"},
	}
	var stdout, stderr bytes.Buffer
	opts := cleanupOptions{
		FS:                fsys.NewFake(),
		JSON:              true,
		Force:             false,
		DoltClient:        client,
		ExactTarget:       "flext_infra",
		DiscoverProcesses: func() ([]DoltProcInfo, error) { return nil, nil },
	}
	code := runDoltCleanup(opts, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d, stderr=%q", code, stderr.String())
	}
	var r CleanupReport
	if err := json.Unmarshal(stdout.Bytes(), &r); err != nil {
		t.Fatalf("Unmarshal: %v\nstdout: %s", err, stdout.String())
	}

	// Dry-run: only flext_infra is the would-be target; hs and testdb_other
	// must NOT appear.
	if !equalStringSlice(r.Dropped.Names, []string{"flext_infra"}) {
		t.Errorf("Dropped.Names = %v, want [flext_infra]", r.Dropped.Names)
	}
	if len(client.dropped) != 0 {
		t.Errorf("DropDatabase called %d times in dry-run; want 0", len(client.dropped))
	}
}

func TestRunDoltCleanup_ExactTargetForceDropsOnlyTarget(t *testing.T) {
	client := &fakeCleanupDoltClient{
		databases: []string{"flext_infra", "hs", "testdb_other"},
	}
	var stdout, stderr bytes.Buffer
	opts := cleanupOptions{
		FS:                fsys.NewFake(),
		JSON:              true,
		Force:             true,
		DoltClient:        client,
		ExactTarget:       "flext_infra",
		DiscoverProcesses: func() ([]DoltProcInfo, error) { return nil, nil },
		ReapGracePeriod:   1,
	}
	code := runDoltCleanup(opts, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d, stderr=%q", code, stderr.String())
	}
	var r CleanupReport
	if err := json.Unmarshal(stdout.Bytes(), &r); err != nil {
		t.Fatalf("Unmarshal: %v\nstdout: %s", err, stdout.String())
	}

	if !equalStringSlice(client.dropped, []string{"flext_infra"}) {
		t.Errorf("dropped = %v, want [flext_infra]", client.dropped)
	}
	// hs must NOT have been dropped.
	for _, d := range client.dropped {
		if d == "hs" {
			t.Errorf("hs was dropped; it must never be touched")
		}
	}
}

func TestRunDoltCleanup_ExactTargetRefusesRegisteredRigDB(t *testing.T) {
	fs := fsys.NewFake()
	fs.Files["/city/.beads/metadata.json"] = []byte(`{"dolt_database":"flext_infra"}`)
	rigs := []resolverRig{{Name: "city", Path: "/city", HQ: true}}
	client := &fakeCleanupDoltClient{
		databases: []string{"flext_infra", "hs"},
	}
	var stdout, stderr bytes.Buffer
	opts := cleanupOptions{
		Rigs:              rigs,
		FS:                fs,
		JSON:              true,
		Force:             true,
		DoltClient:        client,
		ExactTarget:       "flext_infra",
		DiscoverProcesses: func() ([]DoltProcInfo, error) { return nil, nil },
		ReapGracePeriod:   1,
	}
	code := runDoltCleanup(opts, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d, stderr=%q", code, stderr.String())
	}
	var r CleanupReport
	if err := json.Unmarshal(stdout.Bytes(), &r); err != nil {
		t.Fatalf("Unmarshal: %v\nstdout: %s", err, stdout.String())
	}
	if len(client.dropped) != 0 {
		t.Errorf("dropped = %v, want no drops when exact-target is a registered rig DB", client.dropped)
	}
	foundRegisteredErr := false
	for _, e := range r.Errors {
		if e.Kind == cleanupErrorKindExactTargetRegistered && e.Name == "flext_infra" {
			foundRegisteredErr = true
		}
	}
	if !foundRegisteredErr {
		t.Errorf("expected exact-target-registered-owner error for flext_infra; errors=%+v", r.Errors)
	}
}

func TestRunDoltCleanup_ExactTargetRejectsInvalidIdentifier(t *testing.T) {
	client := &fakeCleanupDoltClient{
		databases: []string{"flext_infra"},
	}
	var stdout, stderr bytes.Buffer
	opts := cleanupOptions{
		FS:                fsys.NewFake(),
		JSON:              true,
		Force:             true,
		DoltClient:        client,
		ExactTarget:       "flext; DROP DATABASE hs; --",
		DiscoverProcesses: func() ([]DoltProcInfo, error) { return nil, nil },
		ReapGracePeriod:   1,
	}
	code := runDoltCleanup(opts, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d, stderr=%q", code, stderr.String())
	}
	var r CleanupReport
	if err := json.Unmarshal(stdout.Bytes(), &r); err != nil {
		t.Fatalf("Unmarshal: %v\nstdout: %s", err, stdout.String())
	}
	if len(client.dropped) != 0 {
		t.Errorf("dropped = %v, want no drops for invalid identifier", client.dropped)
	}
	foundInvalidErr := false
	for _, e := range r.Errors {
		if e.Kind == cleanupErrorKindExactTargetInvalidID {
			foundInvalidErr = true
		}
	}
	if !foundInvalidErr {
		t.Errorf("expected exact-target-invalid-identifier error; errors=%+v", r.Errors)
	}
}

func TestRunDoltCleanup_ExactTargetNotFoundOnServer(t *testing.T) {
	client := &fakeCleanupDoltClient{
		databases: []string{"hs", "beads"},
	}
	var stdout, stderr bytes.Buffer
	opts := cleanupOptions{
		FS:                fsys.NewFake(),
		JSON:              true,
		Force:             true,
		DoltClient:        client,
		ExactTarget:       "flext_infra",
		DiscoverProcesses: func() ([]DoltProcInfo, error) { return nil, nil },
		ReapGracePeriod:   1,
	}
	code := runDoltCleanup(opts, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d, stderr=%q", code, stderr.String())
	}
	var r CleanupReport
	if err := json.Unmarshal(stdout.Bytes(), &r); err != nil {
		t.Fatalf("Unmarshal: %v\nstdout: %s", err, stdout.String())
	}
	if len(client.dropped) != 0 {
		t.Errorf("dropped = %v, want no drops when target does not exist", client.dropped)
	}
	foundNotFound := false
	for _, e := range r.Errors {
		if e.Kind == cleanupErrorKindExactTargetNotFound && e.Name == "flext_infra" {
			foundNotFound = true
		}
	}
	if !foundNotFound {
		t.Errorf("expected exact-target-not-found error for flext_infra; errors=%+v", r.Errors)
	}
}

func TestRunDoltCleanup_ExactTargetProbeFailureFailClosed(t *testing.T) {
	client := &fakeCleanupDoltClient{
		databases:       []string{"flext_infra"},
		liveSessionsErr: errors.New("connection refused"),
	}
	var stdout, stderr bytes.Buffer
	opts := cleanupOptions{
		FS:                fsys.NewFake(),
		JSON:              true,
		Force:             true,
		DoltClient:        client,
		ExactTarget:       "flext_infra",
		DiscoverProcesses: func() ([]DoltProcInfo, error) { return nil, nil },
		ReapGracePeriod:   1,
	}
	code := runDoltCleanup(opts, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d, stderr=%q", code, stderr.String())
	}
	var r CleanupReport
	if err := json.Unmarshal(stdout.Bytes(), &r); err != nil {
		t.Fatalf("Unmarshal: %v\nstdout: %s", err, stdout.String())
	}
	if len(client.dropped) != 0 {
		t.Errorf("dropped = %v, want 0 when probe fails (fail-closed)", client.dropped)
	}
	foundProbeFB := false
	for _, fb := range r.ForceBlockers {
		if fb.Kind == cleanupErrorKindLiveSessionProbeFailed {
			foundProbeFB = true
		}
	}
	if !foundProbeFB {
		t.Errorf("expected live-session-probe-failed force blocker; force_blockers=%+v", r.ForceBlockers)
	}
}

func TestRunDoltCleanup_ExactTargetLiveSessionProtected(t *testing.T) {
	client := &fakeCleanupDoltClient{
		databases:    []string{"flext_infra"},
		liveSessions: map[string]int{"flext_infra": 2},
	}
	var stdout, stderr bytes.Buffer
	opts := cleanupOptions{
		FS:                fsys.NewFake(),
		JSON:              true,
		Force:             true,
		DoltClient:        client,
		ExactTarget:       "flext_infra",
		DiscoverProcesses: func() ([]DoltProcInfo, error) { return nil, nil },
		ReapGracePeriod:   1,
	}
	code := runDoltCleanup(opts, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d, stderr=%q", code, stderr.String())
	}
	var r CleanupReport
	if err := json.Unmarshal(stdout.Bytes(), &r); err != nil {
		t.Fatalf("Unmarshal: %v\nstdout: %s", err, stdout.String())
	}
	if len(client.dropped) != 0 {
		t.Errorf("dropped = %v, want 0 when target has live sessions", client.dropped)
	}
	foundLiveSkip := false
	for _, s := range r.Dropped.Skipped {
		if s.Name == "flext_infra" && s.Reason == DropSkipReasonLiveSession {
			foundLiveSkip = true
		}
	}
	if !foundLiveSkip {
		t.Errorf("expected live-session skip for flext_infra; skipped=%+v", r.Dropped.Skipped)
	}
}

func TestRunDoltCleanup_ExactTargetWithoutDoltClientReportsError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	opts := cleanupOptions{
		FS:                fsys.NewFake(),
		JSON:              true,
		Force:             true,
		ExactTarget:       "flext_infra",
		DoltClientOpenErr: errors.New("no sql server on 127.0.0.1:3306"),
		DiscoverProcesses: func() ([]DoltProcInfo, error) { return nil, nil },
		ReapGracePeriod:   1,
	}
	code := runDoltCleanup(opts, &stdout, &stderr)
	// DoltClientOpenErr makes runDoltCleanup return 1 (exit 1), but the
	// report is still emitted before the exit-code check.
	if code != 1 {
		t.Fatalf("exit=%d, want 1 (DoltClientOpenErr); stderr=%q", code, stderr.String())
	}
	var r CleanupReport
	if err := json.Unmarshal(stdout.Bytes(), &r); err != nil {
		t.Fatalf("Unmarshal: %v\nstdout: %s", err, stdout.String())
	}
	// No drops should occur without a DoltClient.
	foundOpenErr := false
	for _, e := range r.Errors {
		if strings.Contains(e.Error, "no sql server") {
			foundOpenErr = true
		}
	}
	if !foundOpenErr {
		t.Errorf("expected DoltClientOpenErr to be surfaced; errors=%+v", r.Errors)
	}
}
