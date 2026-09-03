package main

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/gastownhall/gascity/internal/config"
)

func TestDrainWorkflowServeWorkPropagatesFirstControlFailure(t *testing.T) {
	clearGCEnv(t)

	blocked := errors.New(`pl-pujtf: completing workflow head: updating bead "pl-pujtf": exit status 1: cannot close blocked issue: pl-pujtf is blocked by [pl-mmneh]`)

	prevList := workflowServeList
	prevControl := controlDispatcherServe
	t.Cleanup(func() {
		workflowServeList = prevList
		controlDispatcherServe = prevControl
	})

	workflowServeList = func(context.Context, string, string, map[string]string) ([]hookBead, error) {
		return []hookBead{{ID: "pl-mmneh", Metadata: hookBeadMetadata{"gc.kind": "workflow-finalize"}}}, nil
	}
	serveCalls := 0
	controlDispatcherServe = func(context.Context, string, string, string, io.Writer, io.Writer) error {
		serveCalls++
		return blocked
	}

	cityPath := t.TempDir()
	result, err := drainWorkflowServeWork(
		context.Background(), config.Agent{Name: "control-dispatcher"}, cityPath, cityPath, "bd ready", nil, io.Discard)
	if err == nil {
		t.Fatal("drainWorkflowServeWork returned nil, want first control failure")
	}
	if !errors.Is(err, blocked) {
		t.Fatalf("drainWorkflowServeWork error = %v, want blocked cause", err)
	}
	if !strings.Contains(err.Error(), "processing control bead pl-mmneh") {
		t.Fatalf("drainWorkflowServeWork error = %v, want bead context", err)
	}
	if serveCalls != 1 {
		t.Fatalf("controlDispatcherServe called %d times, want 1", serveCalls)
	}
	if result.processedAny || result.pendingAny {
		t.Fatalf("result = %+v, want no processed or pending progress after first failure", result)
	}
}
