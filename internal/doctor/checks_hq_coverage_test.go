package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gastownhall/gascity/internal/config"
)

func TestHQPackCoverageCheck_NoPacks(t *testing.T) {
	cfg := &config.City{}
	c := NewHQPackCoverageCheck(cfg)
	r := c.Run(&CheckContext{})
	if r.Status != StatusOK {
		t.Errorf("status = %d, want OK; msg = %s", r.Status, r.Message)
	}
}

// TestHQPackCoverageCheck_RigOnlyImportUncovered is the gc-o1u9e
// regression: a pack that declares a city-scope always session but is
// imported only at some rig's scope (never at city/HQ scope) must be
// flagged — its formulas will not materialize into the city root's
// .beads/formulas even though the pack is "known" to the city.
func TestHQPackCoverageCheck_RigOnlyImportUncovered(t *testing.T) {
	dir := t.TempDir()
	packDir := filepath.Join(dir, "packs", "gastown")
	writeTestPack(t, packDir, `
[pack]
name = "gastown"
schema = 2

[[named_session]]
template = "deacon"
scope = "city"
mode = "always"
`)
	writeTestAgent(t, packDir, "deacon")

	cfg := &config.City{
		Rigs: []config.Rig{{Name: "cliproxy-mgmt"}},
		RigPackDirs: map[string][]string{
			"cliproxy-mgmt": {packDir},
		},
	}
	c := NewHQPackCoverageCheck(cfg)
	r := c.Run(&CheckContext{})
	if r.Status != StatusWarning {
		t.Errorf("status = %d, want Warning (rig-only import does not cover city scope); msg = %s", r.Status, r.Message)
	}
	found := false
	for _, d := range r.Details {
		if strings.Contains(d, "gastown") && strings.Contains(d, "deacon") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a detail naming pack gastown and template deacon, got %v", r.Details)
	}
}

// TestHQPackCoverageCheck_CityImportCovers is the positive case: once
// the same pack is also imported at city/HQ scope (PackDirs), the gap
// closes even though the rig-level import is still present too.
func TestHQPackCoverageCheck_CityImportCovers(t *testing.T) {
	dir := t.TempDir()
	packDir := filepath.Join(dir, "packs", "gastown")
	writeTestPack(t, packDir, `
[pack]
name = "gastown"
schema = 2

[[named_session]]
template = "deacon"
scope = "city"
mode = "always"
`)
	writeTestAgent(t, packDir, "deacon")

	cfg := &config.City{
		PackDirs: []string{packDir},
		Rigs:     []config.Rig{{Name: "cliproxy-mgmt"}},
		RigPackDirs: map[string][]string{
			"cliproxy-mgmt": {packDir},
		},
	}
	c := NewHQPackCoverageCheck(cfg)
	r := c.Run(&CheckContext{})
	if r.Status != StatusOK {
		t.Errorf("status = %d, want OK (city/HQ import covers it); msg = %s; details = %v", r.Status, r.Message, r.Details)
	}
}

// TestHQPackCoverageCheck_RigScopedIgnored asserts the check only
// evaluates scope="city" sessions — a pack whose only always-mode
// session is rig-scoped (RigPackCoverageCheck's concern) must not
// trigger a warning here even when it's imported nowhere at city scope.
func TestHQPackCoverageCheck_RigScopedIgnored(t *testing.T) {
	dir := t.TempDir()
	packDir := filepath.Join(dir, "packs", "workflow")
	writeTestPack(t, packDir, `
[pack]
name = "workflow"
schema = 2

[[named_session]]
template = "witness"
scope = "rig"
mode = "always"
`)
	writeTestAgent(t, packDir, "witness")

	cfg := &config.City{
		Rigs: []config.Rig{{Name: "myproject"}},
		RigPackDirs: map[string][]string{
			"myproject": {packDir},
		},
	}
	c := NewHQPackCoverageCheck(cfg)
	r := c.Run(&CheckContext{})
	if r.Status != StatusOK {
		t.Errorf("status = %d, want OK (rig-scoped session is not this check's concern); msg = %s; details = %v", r.Status, r.Message, r.Details)
	}
}

// TestHQPackCoverageCheck_OnDemandNotWarned mirrors the rig-coverage
// check's own rule: only mode="always" sessions represent an operating
// expectation worth flagging.
func TestHQPackCoverageCheck_OnDemandNotWarned(t *testing.T) {
	dir := t.TempDir()
	packDir := filepath.Join(dir, "packs", "workflow")
	writeTestPack(t, packDir, `
[pack]
name = "workflow"
schema = 2

[[named_session]]
template = "helper"
scope = "city"
mode = "on_demand"
`)
	writeTestAgent(t, packDir, "helper")

	cfg := &config.City{
		Rigs: []config.Rig{{Name: "myproject"}},
		RigPackDirs: map[string][]string{
			"myproject": {packDir},
		},
	}
	c := NewHQPackCoverageCheck(cfg)
	r := c.Run(&CheckContext{})
	if r.Status != StatusOK {
		t.Errorf("status = %d, want OK (on_demand should not warn); msg = %s", r.Status, r.Message)
	}
}

// TestHQPackCoverageCheck_MalformedPackToml mirrors the rig-coverage
// check's [M1] behavior — a pack whose pack.toml exists but cannot be
// parsed surfaces a warning identifying the pack and the parse error,
// rather than silently skipping.
func TestHQPackCoverageCheck_MalformedPackToml(t *testing.T) {
	dir := t.TempDir()
	packDir := filepath.Join(dir, "packs", "broken")
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packDir, "pack.toml"), []byte("not valid = toml = at all"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.City{
		Rigs: []config.Rig{{Name: "myproject"}},
		RigPackDirs: map[string][]string{
			"myproject": {packDir},
		},
	}
	c := NewHQPackCoverageCheck(cfg)
	r := c.Run(&CheckContext{})
	if r.Status != StatusWarning {
		t.Fatalf("status = %d, want Warning (malformed pack.toml); details = %v", r.Status, r.Details)
	}
	var found bool
	for _, d := range r.Details {
		if strings.Contains(d, "broken") && strings.Contains(d, "pack.toml") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a detail identifying packs/broken/pack.toml, got %v", r.Details)
	}
}

func TestHQPackCoverageCheck_FixHint(t *testing.T) {
	dir := t.TempDir()
	packDir := filepath.Join(dir, "packs", "gastown")
	writeTestPack(t, packDir, `
[pack]
name = "gastown"
schema = 2

[[named_session]]
template = "deacon"
scope = "city"
mode = "always"
`)
	writeTestAgent(t, packDir, "deacon")

	cfg := &config.City{
		Rigs: []config.Rig{{Name: "cliproxy-mgmt"}},
		RigPackDirs: map[string][]string{
			"cliproxy-mgmt": {packDir},
		},
	}
	c := NewHQPackCoverageCheck(cfg)
	r := c.Run(&CheckContext{})
	if r.FixHint == "" {
		t.Error("expected a fix hint")
	}
	if !strings.Contains(r.FixHint, "gc import add") {
		t.Errorf("FixHint = %q, want it to name the canonical gc import add surface", r.FixHint)
	}
}
