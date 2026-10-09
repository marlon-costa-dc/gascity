package main

import (
	"slices"
	"testing"

	"github.com/gastownhall/gascity/internal/beads"
	"github.com/gastownhall/gascity/internal/config"
)

// canonicalSingletonPoolSession builds the session bead shape a canonical
// singleton pool worker carries: pool-managed, its alias forced to the agent's
// qualified name (normalizeNonExpandingPoolSessionInfo), and a generated
// session name. Measured 2026-10-09 on the live city: bd.dog session
// gc-wisp-qeh0t7 (session_name bd__dog-gc-wisp-qeh0t7, alias bd.dog) claimed
// routed work with assignee "bd.dog" and was drained "orphaned" about a minute
// later, its claim released (gct-lv2pe item 4).
func canonicalSingletonPoolSession(id, template, alias string) beads.Bead {
	return beads.Bead{
		ID:     id,
		Status: "open",
		Type:   sessionBeadType,
		Metadata: map[string]string{
			"session_name": "dog-" + id,
			"template":     template,
			"alias":        alias,
			"pool_managed": "true",
			"state":        "active",
		},
	}
}

func TestSessionAssignmentIdentifiers_CanonicalSingletonPoolIncludesAlias(t *testing.T) {
	cfg := &config.City{Agents: []config.Agent{poolAgent("dog", "", intPtr(1), 0)}}
	bead := canonicalSingletonPoolSession("gc-wisp-qeh0t7", "dog", "dog")

	ids := sessionAssignmentIdentifiersForConfig(bead, cfg)
	if !slices.Contains(ids, "dog") {
		t.Fatalf("canonical singleton pool session must answer to its alias, which gc hook --claim stamps; got %v", ids)
	}
	infoIDs := sessionAssignmentIdentifiersForConfigInfo(sessionInfosFromBeads([]beads.Bead{bead})[0], cfg)
	if !slices.Equal(ids, infoIDs) {
		t.Fatalf("bead and Info identifier twins diverged: bead=%v info=%v", ids, infoIDs)
	}
}

func TestSessionAssignmentIdentifiers_MultiSessionPoolExcludesTemplateAlias(t *testing.T) {
	cfg := &config.City{Agents: []config.Agent{poolAgent("dog", "", intPtr(2), 0)}}
	bead := canonicalSingletonPoolSession("gc-wisp-a", "dog", "dog")

	if ids := sessionAssignmentIdentifiersForConfig(bead, cfg); slices.Contains(ids, "dog") {
		t.Fatalf("a multi-session pool worker must not own the bare template alias (several sessions share it); got %v", ids)
	}
}

func TestSessionAssigneeMatches_CanonicalSingletonAliasKeepsWorkerAwake(t *testing.T) {
	bead := AwakeSessionBead{ID: "gc-wisp-qeh0t7", SessionName: "dog-gc-wisp-qeh0t7", CanonicalSingletonAlias: "dog"}
	work := []AwakeWorkBead{{ID: "gc-henbes", Assignee: "dog", Status: "in_progress"}}

	if !sessionHasAssignedWork(work, nil, bead) {
		t.Fatal("in_progress work claimed under the canonical singleton alias must keep its worker awake")
	}
	if sessionHasAssignedWork(work, nil, AwakeSessionBead{ID: "gc-wisp-b", SessionName: "dog-gc-wisp-b"}) {
		t.Fatal("a session without the canonical alias must not match work assigned to the bare template")
	}
}
