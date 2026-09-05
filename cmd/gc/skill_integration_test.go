package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gastownhall/gascity/internal/config"
	"github.com/gastownhall/gascity/internal/materialize"
	"github.com/gastownhall/gascity/internal/shellquote"
)

func TestIsStage2EligibleSession(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		cityProvider string
		agentSession string
		wantEligible bool
	}{
		{"default empty → tmux (eligible)", "", "", true},
		{"tmux eligible", "tmux", "", true},
		// herdr executes PreStart via the herdr provider's runPreStart
		// before the agent is created (internal/runtime/herdr/provider.go).
		{"herdr eligible (executes PreStart)", "herdr", "", true},
		{"herdr + acp agent → ineligible", "herdr", "acp", false},
		// subprocess runtime does not execute PreStart in v0.15.1 —
		// ineligible per Phase 3 pass-1 review.
		{"subprocess ineligible (no PreStart execution)", "subprocess", "", false},
		{"k8s ineligible", "k8s", "", false},
		{"acp city ineligible", "acp", "", false},
		{"hybrid ineligible", "hybrid", "", false},
		{"exec prefix ineligible", "exec:./run.sh", "", false},
		{"fake ineligible", "fake", "", false},
		{"tmux + acp agent → ineligible", "tmux", "acp", false},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			agent := &config.Agent{Session: c.agentSession}
			got := isStage2EligibleSession(c.cityProvider, agent)
			if got != c.wantEligible {
				t.Fatalf("isStage2EligibleSession(%q, %q) = %v, want %v",
					c.cityProvider, c.agentSession, got, c.wantEligible)
			}
		})
	}
}

func TestAgentScopeRoot(t *testing.T) {
	t.Parallel()
	rigs := []config.Rig{
		{Name: "fe", Path: "/rigs/fe"},
		{Name: "be", Path: "/rigs/be"},
	}
	cases := []struct {
		name  string
		agent config.Agent
		want  string
	}{
		{"city-scoped returns cityPath", config.Agent{Scope: "city"}, "/city"},
		{"rig-scoped returns rig path", config.Agent{Scope: "rig", Dir: "fe"}, "/rigs/fe"},
		{"empty scope defaults to rig", config.Agent{Dir: "be"}, "/rigs/be"},
		{"unknown rig falls back to cityPath", config.Agent{Scope: "rig", Dir: "unknown"}, "/city"},
		{"empty dir rig-scope falls back", config.Agent{Scope: "rig"}, "/city"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := agentScopeRoot(&c.agent, "/city", rigs)
			if got != c.want {
				t.Fatalf("agentScopeRoot(%+v) = %q, want %q", c.agent, got, c.want)
			}
		})
	}
}

func TestAgentRigScopeName(t *testing.T) {
	t.Parallel()

	rigs := []config.Rig{
		{Name: "fe", Path: "/rigs/fe"},
		{Name: "be", Path: "/rigs/be"},
	}
	cases := []struct {
		name  string
		agent *config.Agent
		want  string
	}{
		{name: "nil agent", agent: nil, want: ""},
		{name: "city-scoped matching dir stays city", agent: &config.Agent{Scope: "city", Dir: "fe"}, want: ""},
		{name: "rig-scoped matching dir uses rig", agent: &config.Agent{Scope: "rig", Dir: "fe"}, want: "fe"},
		{name: "empty scope matching dir defaults to rig", agent: &config.Agent{Dir: "be"}, want: "be"},
		{name: "plain dir not a rig", agent: &config.Agent{Dir: "workdir"}, want: ""},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := agentRigScopeName(c.agent, rigs); got != c.want {
				t.Fatalf("agentRigScopeName(%+v) = %q, want %q", c.agent, got, c.want)
			}
		})
	}
}

func TestSharedSkillCatalogForAgentDoesNotFallBackWhenRigCatalogFails(t *testing.T) {
	t.Parallel()

	cityPath := t.TempDir()
	writeSkillSource(t, filepath.Join(cityPath, "skills", "city-shared"))

	badRigCatalog := filepath.Join(cityPath, "broken-rig-catalog")
	if err := os.Mkdir(badRigCatalog, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(badRigCatalog, 0o755) })
	if _, err := os.ReadDir(badRigCatalog); err == nil {
		t.Skip("environment ignores chmod 000 (likely running as root)")
	}

	cfg := &config.City{
		PackSkillsDir: filepath.Join(cityPath, "skills"),
		Rigs:          []config.Rig{{Name: "fe", Path: filepath.Join(cityPath, "rigs", "fe")}},
		RigPackSkills: map[string][]config.DiscoveredSkillCatalog{
			"fe": {{
				SourceDir:   badRigCatalog,
				BindingName: "ops",
				PackName:    "helper",
			}},
		},
	}

	var stderr strings.Builder
	params := newAgentBuildParams("test-city", cityPath, cfg, nil, time.Now(), nil, &stderr)
	if params.skillCatalog == nil {
		t.Fatal("city skill catalog should still load")
	}
	if got := params.sharedSkillCatalogForAgent(&config.Agent{Name: "rig-agent", Scope: "rig", Dir: "fe"}); got != nil {
		t.Fatalf("sharedSkillCatalogForAgent() = %+v, want nil when rig catalog load fails", got)
	}
	if !strings.Contains(stderr.String(), `LoadCityCatalog rig "fe"`) {
		t.Fatalf("stderr = %q, want rig catalog load error", stderr.String())
	}
}

func TestSharedSkillCatalogForAgentUsesCachedRigCatalogAfterFailure(t *testing.T) {
	resetSkillCatalogCache()

	cityPath := t.TempDir()
	writeSkillSource(t, filepath.Join(cityPath, "skills", "city-shared"))
	realRigCatalog := filepath.Join(cityPath, "real-rig-skills")
	rigCatalog := filepath.Join(cityPath, "rig-skills-link")
	writeSkillSource(t, filepath.Join(realRigCatalog, "ops"))
	symlinkOrSkip(t, realRigCatalog, rigCatalog)

	cfg := &config.City{
		PackSkillsDir: filepath.Join(cityPath, "skills"),
		Rigs:          []config.Rig{{Name: "fe", Path: filepath.Join(cityPath, "rigs", "fe")}},
		RigPackSkills: map[string][]config.DiscoveredSkillCatalog{
			"fe": {{
				SourceDir:   rigCatalog,
				BindingName: "ops",
				PackName:    "helper",
			}},
		},
	}
	agent := &config.Agent{Name: "rig-agent", Scope: "rig", Dir: "fe"}

	params := newAgentBuildParams("test-city", cityPath, cfg, nil, time.Now(), nil, nil)
	if got := params.sharedSkillCatalogForAgent(agent); got == nil || len(got.Entries) == 0 {
		t.Fatalf("baseline sharedSkillCatalogForAgent() = %+v, want non-empty rig catalog", got)
	}

	replaceWithSelfSymlink(t, rigCatalog)
	var stderr strings.Builder
	params = newAgentBuildParams("test-city", cityPath, cfg, nil, time.Now(), nil, &stderr)
	got := params.sharedSkillCatalogForAgent(agent)
	if got == nil || len(got.Entries) == 0 {
		t.Fatalf("sharedSkillCatalogForAgent() = %+v, want cached rig catalog after transient failure", got)
	}
	if !strings.Contains(stderr.String(), `LoadCityCatalog rig "fe"`) {
		t.Errorf("stderr = %q, want rig catalog load error", stderr.String())
	}
}

func TestNewAgentBuildParams_EmptyRigCatalogClearsLastGoodCatalog(t *testing.T) {
	resetSkillCatalogCache()

	cityPath := t.TempDir()
	realRigCatalog := filepath.Join(cityPath, "rig-skills")
	writeSkillSource(t, filepath.Join(realRigCatalog, "ops"))
	cfg := &config.City{
		Rigs: []config.Rig{{Name: "fe", Path: filepath.Join(cityPath, "rigs", "fe")}},
		RigPackSkills: map[string][]config.DiscoveredSkillCatalog{
			"fe": {{
				SourceDir:   realRigCatalog,
				BindingName: "ops",
				PackName:    "helper",
			}},
		},
	}
	agent := &config.Agent{Name: "rig-agent", Scope: "rig", Dir: "fe"}

	params := newAgentBuildParams("test-city", cityPath, cfg, nil, time.Now(), nil, nil)
	if got := params.sharedSkillCatalogForAgent(agent); got == nil || len(got.Entries) == 0 {
		t.Fatalf("baseline sharedSkillCatalogForAgent() = %+v, want non-empty rig catalog", got)
	}

	if err := os.RemoveAll(filepath.Join(realRigCatalog, "ops")); err != nil {
		t.Fatal(err)
	}
	params = newAgentBuildParams("test-city", cityPath, cfg, nil, time.Now(), nil, nil)
	got := params.sharedSkillCatalogForAgent(agent)
	if got == nil {
		t.Fatal("empty successful rig catalog should be represented as an empty catalog, not nil")
	}
	if len(got.Entries) != 0 {
		t.Fatalf("empty successful rig catalog reused stale cache with %d entries", len(got.Entries))
	}
}

func TestEffectiveInjectAssignedSkills(t *testing.T) {
	t.Parallel()
	yes, no := true, false
	cases := []struct {
		name string
		ptr  *bool
		want bool
	}{
		{"nil defaults to true", nil, true},
		{"explicit true", &yes, true},
		{"explicit false", &no, false},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			agent := &config.Agent{InjectAssignedSkills: c.ptr}
			if got := effectiveInjectAssignedSkills(agent); got != c.want {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
	if effectiveInjectAssignedSkills(nil) {
		t.Error("nil agent should not inject")
	}
}

func TestBuildAssignedSkillsPromptFragmentPartitions(t *testing.T) {
	t.Parallel()
	city := &materialize.CityCatalog{
		Entries: []materialize.SkillEntry{
			{Name: "code-review", Source: "/x", Origin: "city", Description: "Review pull requests"},
			{Name: "gc-work", Source: "/y", Origin: "core", Description: "Working with beads"},
			{Name: "planning", Source: "/z", Origin: "city", Description: "Shared planning"},
		},
	}
	agentCat := materialize.AgentCatalog{
		Entries: []materialize.SkillEntry{
			{Name: "mayor-planning", Source: "/a", Origin: "agent", Description: "Mayor-only strategy"},
			// Overrides shared "planning" — should NOT appear in the shared section.
			{Name: "planning", Source: "/b", Origin: "agent", Description: "Mayor's planning override"},
		},
	}
	a := &config.Agent{Name: "mayor", Scope: "city"}
	got := buildAssignedSkillsPromptFragment(a, city, agentCat)

	mustContain := []string{
		"## Skills available to this session",
		"You are `mayor`",
		"### Assigned to you",
		"`mayor-planning` — Mayor-only strategy",
		"`planning` — Mayor's planning override",
		"### Shared in this scope",
		"`code-review` — Review pull requests *(city)*",
		"`gc-work` — Working with beads *(core)*",
	}
	for _, want := range mustContain {
		if !strings.Contains(got, want) {
			t.Errorf("fragment missing %q:\n%s", want, got)
		}
	}

	// Agent-local "planning" must SHADOW the city "planning" from the
	// shared section — agents should see their override, not the
	// conflicting shared entry.
	if strings.Contains(got, "Shared planning") {
		t.Errorf("shared section still lists the shadowed city planning entry:\n%s", got)
	}
}

func TestBuildAssignedSkillsPromptFragmentEmptyInputs(t *testing.T) {
	t.Parallel()
	a := &config.Agent{Name: "x"}
	if got := buildAssignedSkillsPromptFragment(a, nil, materialize.AgentCatalog{}); got != "" {
		t.Errorf("empty inputs should return empty fragment, got: %q", got)
	}
	// City-only (no agent-local) still renders, just without the Assigned section.
	city := &materialize.CityCatalog{
		Entries: []materialize.SkillEntry{
			{Name: "gc-work", Source: "/x", Origin: "core", Description: "Work stuff"},
		},
	}
	got := buildAssignedSkillsPromptFragment(a, city, materialize.AgentCatalog{})
	if got == "" {
		t.Fatal("expected non-empty fragment when city catalog has entries")
	}
	if strings.Contains(got, "### Assigned to you") {
		t.Errorf("should not render Assigned section when no agent-local skills:\n%s", got)
	}
	if !strings.Contains(got, "### Shared") {
		t.Errorf("should render Shared section:\n%s", got)
	}
}

func TestBuildAssignedSkillsPromptFragmentAgentOnlyNoCity(t *testing.T) {
	t.Parallel()
	a := &config.Agent{Name: "solo"}
	agentCat := materialize.AgentCatalog{
		Entries: []materialize.SkillEntry{{Name: "only-mine", Source: "/x", Origin: "agent"}},
	}
	got := buildAssignedSkillsPromptFragment(a, nil, agentCat)
	if got == "" {
		t.Fatal("agent-local-only should still render")
	}
	if !strings.Contains(got, "### Assigned to you") {
		t.Errorf("missing Assigned section:\n%s", got)
	}
	if strings.Contains(got, "### Shared") {
		t.Errorf("Shared section should not render when no city catalog:\n%s", got)
	}
}

func TestBuildAssignedSkillsPromptFragmentOmitsDescriptionWhenMissing(t *testing.T) {
	t.Parallel()
	a := &config.Agent{Name: "x"}
	city := &materialize.CityCatalog{
		Entries: []materialize.SkillEntry{
			{Name: "bare", Source: "/x", Origin: "city"}, // no Description
		},
	}
	got := buildAssignedSkillsPromptFragment(a, city, materialize.AgentCatalog{})
	// Name present, no dash-separator.
	if !strings.Contains(got, "`bare`") {
		t.Errorf("missing bare skill name:\n%s", got)
	}
	if strings.Contains(got, "`bare` — ") {
		t.Errorf("should not render em-dash separator when description is empty:\n%s", got)
	}
}

func TestAppendAgentsctlSyncPreStart(t *testing.T) {
	t.Parallel()
	existing := []string{"mkdir -p .cache", "./setup.sh"}
	workDir := "/worktrees/pole cat-1"
	got := appendAgentsctlSyncPreStart(existing, workDir)
	if len(got) != 3 {
		t.Fatalf("want 3 entries, got %d (%v)", len(got), got)
	}
	// User-configured entries run first; the projection owner reconciles
	// the session workdir last, immediately before the agent starts.
	if got[0] != "mkdir -p .cache" || got[1] != "./setup.sh" {
		t.Errorf("user entries reordered: %v", got)
	}
	// The generated entry is exactly `cd <quoted workdir> && agentsctl sync`:
	// the workdir is shell-quoted (spaces survive) and the command carries
	// no gc binary, agent identity, or catalog snapshot — agentsctl is the
	// sole owner of the projection and resolves everything from the cwd.
	last := got[2]
	want := "cd " + shellquote.Join([]string{workDir}) + agentsctlSyncPreStartSuffix
	if last != want {
		t.Errorf("generated entry = %q, want %q", last, want)
	}
	if strings.Contains(last, "GC_BIN") || strings.Contains(last, "materialize-skills") {
		t.Errorf("agentsctl sync entry must not reference the retired gc materializer: %q", last)
	}
	if !isGeneratedPreStartCommand(last) {
		t.Errorf("generated entry must be recognized for retargeting: %q", last)
	}
	if isGeneratedPreStartCommand("cd /somewhere && ./setup.sh") {
		t.Errorf("user-authored cd command must not be treated as generated")
	}
}

// helpers

// writeSkillSource creates a minimal skill source directory (SKILL.md with
// name derived from the directory) for stage-1 materialization tests.
func writeSkillSource(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + filepath.Base(dir) + "\ndescription: test\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
