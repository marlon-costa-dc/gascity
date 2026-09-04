package main

import (
	"context"
	"fmt"
	"strconv"
)

// Injectable function vars for unit testing.
var (
	managedDoltInitWaitIsManaged = cityUsesManagedDoltBeadsLifecycle
	managedDoltInitWaitReadState = readValidPublishedManagedDoltState
	managedDoltInitWaitReachable = managedDoltTCPReachable
	managedDoltInitWaitPidAlive  = pidAlive
)

// waitForManagedDoltInitReady blocks until the managed Dolt process for
// cityPath is TCP-reachable, or returns the first observed readiness failure.
//
// Returns nil immediately when cityPath is not a managed-Dolt city, so
// callers can invoke it unconditionally.
func waitForManagedDoltInitReady(ctx context.Context, cityPath string) error {
	if ctx == nil {
		return fmt.Errorf("wait for managed Dolt init readiness: nil context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !managedDoltInitWaitIsManaged(cityPath) {
		return nil
	}
	state, ok := managedDoltInitWaitReadState(cityPath)
	if !ok || state.Port <= 0 {
		return fmt.Errorf("managed Dolt runtime state is not published")
	}
	if state.PID > 0 && !managedDoltInitWaitPidAlive(state.PID) {
		return fmt.Errorf("managed Dolt process (pid %d) exited before becoming TCP-ready", state.PID)
	}
	host := managedDoltConnectHost("")
	port := strconv.Itoa(state.Port)
	if !managedDoltInitWaitReachable(host, port) {
		return fmt.Errorf("managed Dolt on port %d is not TCP-ready", state.Port)
	}
	return nil
}
