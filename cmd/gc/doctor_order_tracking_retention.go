package main

import (
	"fmt"
	"strings"

	"github.com/gastownhall/gascity/internal/beads"
	"github.com/gastownhall/gascity/internal/config"
	"github.com/gastownhall/gascity/internal/doctor"
)

const (
	orderTrackingRetentionCheckThreshold = 500
	orderTrackingRetentionCheckListLimit = 501
)

// orderTrackingRetentionCheck reports the count of closed order-tracking beads
// in the city store and warns when retention sweeps are overdue. The controller
// watchdog prunes these automatically (7d TTL default); the check surfaces
// cities where the watchdog has not yet run or where the backlog is large enough
// to be operationally visible. It is pure observability and never gates.
type orderTrackingRetentionCheck struct {
	cfg      *config.City
	cityPath string
	newStore func(string) (beads.Store, error)
}

// newOrderTrackingRetentionCheck constructs an orderTrackingRetentionCheck.
// cfg supplies the effective retention TTL the watchdog prunes with.
func newOrderTrackingRetentionCheck(cfg *config.City, cityPath string, newStore func(string) (beads.Store, error)) *orderTrackingRetentionCheck {
	return &orderTrackingRetentionCheck{cfg: cfg, cityPath: cityPath, newStore: newStore}
}

// Name implements doctor.Check.
func (c *orderTrackingRetentionCheck) Name() string { return "order-tracking-retention" }

// CanFix implements doctor.Check.
func (c *orderTrackingRetentionCheck) CanFix() bool { return false }

// Fix implements doctor.Check.
func (c *orderTrackingRetentionCheck) Fix(_ *doctor.CheckContext) error { return nil }

// WarmupEligible implements doctor.Check.
func (c *orderTrackingRetentionCheck) WarmupEligible() bool { return false }

// Run implements doctor.Check.
func (c *orderTrackingRetentionCheck) Run(_ *doctor.CheckContext) *doctor.CheckResult {
	res := &doctor.CheckResult{Name: c.Name(), Severity: doctor.SeverityAdvisory}
	if c.newStore == nil || strings.TrimSpace(c.cityPath) == "" {
		res.Status = doctor.StatusOK
		res.Message = "order-tracking retention: no bead store configured"
		return res
	}
	store, err := c.newStore(c.cityPath)
	if err != nil {
		res.Status = doctor.StatusWarning
		res.Message = fmt.Sprintf("order-tracking retention unknown: opening city bead store: %v", err)
		return res
	}
	// The city work store holds the pre-cutover backlog; the orders binding
	// holds everything a split city has written since. Counting only one of them
	// reports a healthy 0 on exactly the city whose backlog is growing.
	stores := []beads.Store{store}
	if ordersStore := relocatedOrdersClassStore(c.cityPath, nil); ordersStore != nil && ordersStore != store {
		stores = append(stores, ordersStore)
	}
	count := 0
	capped := false
	for _, s := range stores {
		entries, err := beads.HandlesFor(s).Live.List(beads.ListQuery{
			Status:   "closed",
			Label:    labelOrderTracking,
			TierMode: beads.TierBoth,
			Limit:    orderTrackingRetentionCheckListLimit,
		})
		if err != nil {
			res.Status = doctor.StatusWarning
			res.Message = fmt.Sprintf("order-tracking retention unknown: listing closed beads: %v", err)
			return res
		}
		count += len(entries)
		// Each store's read is capped independently, so "at least this many"
		// has to be tracked per store rather than inferred from the total.
		if len(entries) >= orderTrackingRetentionCheckListLimit {
			capped = true
		}
	}
	if count >= orderTrackingRetentionCheckThreshold {
		countStr := fmt.Sprintf("%d", count)
		if capped {
			countStr = "≥" + countStr
		}
		res.Status = doctor.StatusWarning
		// Config composition always fills delete_after_close (default 7d), so
		// the verdict states the effective TTL and where it comes from rather
		// than telling the operator to configure a value that is already set.
		res.Message = fmt.Sprintf("%s closed order-tracking beads: retention watchdog will prune automatically (TTL %s from [beads.policies.order_tracking].delete_after_close)",
			countStr, orderTrackingRetentionPolicyForConfig(c.cfg).deleteAfterClose)
		return res
	}
	res.Status = doctor.StatusOK
	res.Message = fmt.Sprintf("%d closed order-tracking beads", count)
	return res
}
