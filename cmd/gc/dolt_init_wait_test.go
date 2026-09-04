package main

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
)

// stubManagedDoltInitWait replaces the injectable functions used by
// waitForManagedDoltInitReady for the duration of a test.
type stubManagedDoltInitWait struct {
	isManaged bool
	state     doltRuntimeState
	stateOK   bool
	reachable bool
	pidAlive  bool
}

func (s *stubManagedDoltInitWait) install(t *testing.T) {
	t.Helper()
	prevIsManaged := managedDoltInitWaitIsManaged
	prevReadState := managedDoltInitWaitReadState
	prevReachable := managedDoltInitWaitReachable
	prevPidAlive := managedDoltInitWaitPidAlive

	managedDoltInitWaitIsManaged = func(_ string) bool { return s.isManaged }
	managedDoltInitWaitReadState = func(_ string) (doltRuntimeState, bool) { return s.state, s.stateOK }
	managedDoltInitWaitReachable = func(_, _ string) bool { return s.reachable }
	managedDoltInitWaitPidAlive = func(_ int) bool { return s.pidAlive }

	t.Cleanup(func() {
		managedDoltInitWaitIsManaged = prevIsManaged
		managedDoltInitWaitReadState = prevReadState
		managedDoltInitWaitReachable = prevReachable
		managedDoltInitWaitPidAlive = prevPidAlive
	})
}

// --- waitForManagedDoltInitReady tests ---

// Criterion: non-managed Dolt city — skip immediately, return nil.
func TestWaitForManagedDoltInitReady_NonManaged(t *testing.T) {
	stub := &stubManagedDoltInitWait{isManaged: false}
	stub.install(t)
	if err := waitForManagedDoltInitReady(context.Background(), "any"); err != nil {
		t.Errorf("non-managed: want nil, got %v", err)
	}
}

// Criterion: fast ready — TCP-reachable immediately, return nil.
func TestWaitForManagedDoltInitReady_FastReady(t *testing.T) {
	stub := &stubManagedDoltInitWait{
		isManaged: true,
		state:     doltRuntimeState{PID: 999, Port: 28231},
		stateOK:   true,
		reachable: true,
		pidAlive:  true,
	}
	stub.install(t)
	if err := waitForManagedDoltInitReady(context.Background(), "any"); err != nil {
		t.Errorf("fast ready: want nil, got %v", err)
	}
}

// Criterion: missing state returns immediately; callers retry by reinvoking the
// owner, not inside this helper.
func TestWaitForManagedDoltInitReady_MissingStateReturnsFirstFailure(t *testing.T) {
	var calls atomic.Int32
	prevIsManaged := managedDoltInitWaitIsManaged
	prevReadState := managedDoltInitWaitReadState
	prevReachable := managedDoltInitWaitReachable
	prevPidAlive := managedDoltInitWaitPidAlive
	t.Cleanup(func() {
		managedDoltInitWaitIsManaged = prevIsManaged
		managedDoltInitWaitReadState = prevReadState
		managedDoltInitWaitReachable = prevReachable
		managedDoltInitWaitPidAlive = prevPidAlive
	})
	managedDoltInitWaitIsManaged = func(_ string) bool { return true }
	managedDoltInitWaitPidAlive = func(_ int) bool { return true }
	managedDoltInitWaitReadState = func(_ string) (doltRuntimeState, bool) {
		calls.Add(1)
		return doltRuntimeState{}, false
	}
	managedDoltInitWaitReachable = func(_, _ string) bool {
		t.Fatal("reachable probe ran without published state")
		return false
	}

	err := waitForManagedDoltInitReady(context.Background(), "any")
	if err == nil {
		t.Fatal("missing state error = nil")
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("state read calls = %d, want 1", n)
	}
}

// Criterion: missing port (state not published) — return the first failure.
func TestWaitForManagedDoltInitReady_MissingState(t *testing.T) {
	stub := &stubManagedDoltInitWait{
		isManaged: true,
		stateOK:   false, // port never published
		reachable: false,
		pidAlive:  true,
	}
	stub.install(t)

	err := waitForManagedDoltInitReady(context.Background(), "any")
	if err == nil {
		t.Error("expected error when state never published, got nil")
	}
}

// Criterion: process exits before TCP-ready — return error quickly.
func TestWaitForManagedDoltInitReady_ProcessExits(t *testing.T) {
	stub := &stubManagedDoltInitWait{
		isManaged: true,
		state:     doltRuntimeState{PID: 9999999, Port: 28231},
		stateOK:   true,
		reachable: false, // TCP not ready
		pidAlive:  false, // process exited
	}
	stub.install(t)

	err := waitForManagedDoltInitReady(context.Background(), "any")
	if err == nil {
		t.Error("expected error when process exits, got nil")
	}
}

// Criterion: port published and process alive but TCP not ready — return the first failure.
func TestWaitForManagedDoltInitReady_TCPNotReady(t *testing.T) {
	stub := &stubManagedDoltInitWait{
		isManaged: true,
		state:     doltRuntimeState{PID: 123, Port: 28231},
		stateOK:   true,
		reachable: false, // TCP never becomes ready
		pidAlive:  true,
	}
	stub.install(t)

	err := waitForManagedDoltInitReady(context.Background(), "any")
	if err == nil {
		t.Error("expected TCP readiness error, got nil")
	}
}

// Verify that managedDoltTCPReachable works correctly with a real socket.
// This is an implementation sanity-check, not a wait test per se.
func TestManagedDoltTCPReachable_RealSocket(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer lis.Close() //nolint:errcheck

	addr := lis.Addr().String()
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	if !managedDoltTCPReachable(host, port) {
		t.Errorf("managedDoltTCPReachable(%q, %q) = false, want true", host, port)
	}
	if managedDoltTCPReachable(host, "1") {
		t.Error("managedDoltTCPReachable on port 1 (no listener) = true, want false")
	}
}
