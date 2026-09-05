package doctor

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/gastownhall/gascity/internal/config"
)

// HQPackCoverageCheck warns when a pack declares city-scope always-mode
// named_sessions (e.g. mayor, deacon, boot) but is not imported at
// city/HQ scope. Being imported by some rig is not enough: a rig-level
// import materializes the pack's formulas into that rig's own
// .beads/formulas, never the city root's, so an HQ-scope session
// invoking the documented `bd formula show` / `gc bd mol wisp` surface
// from the city root cannot find its own formulas even though the pack
// looks "known" to the city via that rig.
//
// RigPackCoverageCheck ("rig-pack-coverage") is the mirror image: it
// starts from packs already imported at city scope and checks every
// active rig covers their rig-scoped sessions. This check starts from
// every pack the city knows about anywhere and checks the city itself
// covers their city-scope sessions. Evidence: gc-o1u9e.
type HQPackCoverageCheck struct {
	cfg *config.City
}

// NewHQPackCoverageCheck creates a check for city/HQ-scope pack coverage.
func NewHQPackCoverageCheck(cfg *config.City) *HQPackCoverageCheck {
	return &HQPackCoverageCheck{cfg: cfg}
}

// Name returns the check identifier.
func (c *HQPackCoverageCheck) Name() string { return "hq-pack-coverage" }

// CanFix returns false — the fix is a pack.toml import change via the
// canonical gc import surface, not something this check can perform.
func (c *HQPackCoverageCheck) CanFix() bool { return false }

// Fix is a no-op.
func (c *HQPackCoverageCheck) Fix(_ *CheckContext) error { return nil }

// Run checks that every pack declaring city-scope always-mode
// named_sessions, anywhere it is known to the city (imported at city
// scope or at any rig's scope), is also imported at city/HQ scope.
// Unreadable or malformed pack.toml files are surfaced as warnings
// alongside any coverage gaps, rather than silently skipped, so a
// misconfigured pack does not hide its own diagnostic.
func (c *HQPackCoverageCheck) Run(_ *CheckContext) *CheckResult {
	r := &CheckResult{Name: c.Name()}

	cityPacks := make(map[string]bool, len(c.cfg.PackDirs))
	for _, dir := range c.cfg.PackDirs {
		cityPacks[absOrClean(dir)] = true
	}

	candidates := c.knownPackDirs()

	var issues []string
	for _, packDir := range candidates {
		sessions, err := namedSessionsByScope(packDir, "city")
		if err != nil {
			issues = append(issues, fmt.Sprintf(
				"pack %q: %v (unable to evaluate city-scope named_session coverage)",
				packDir, err))
			continue
		}
		if len(sessions) == 0 || cityPacks[absOrClean(packDir)] {
			continue
		}

		packName := sessions[0].packName
		if packName == "" {
			packName = filepath.Base(packDir)
		}
		for _, s := range sessions {
			issues = append(issues, fmt.Sprintf(
				"pack %q declares city-scope named_session %q (mode=always) but is not imported at city/HQ scope",
				packName, s.template))
		}
	}

	if len(issues) == 0 {
		r.Status = StatusOK
		r.Message = "all city-scope named_sessions covered by city/HQ imports"
		return r
	}
	sort.Strings(issues)
	r.Status = StatusWarning
	r.Message = fmt.Sprintf("%d city-scope named_session(s) not covered by city/HQ imports", len(issues))
	r.Details = issues
	r.FixHint = "add [imports.<pack>] to pack.toml at city/HQ scope via gc import add"
	return r
}

// knownPackDirs returns every pack directory the city knows about —
// its own city-scope imports plus every rig's imports — deduplicated
// by absolute path and sorted for deterministic Details ordering.
func (c *HQPackCoverageCheck) knownPackDirs() []string {
	var dirs []string
	seen := make(map[string]bool)
	add := func(dir string) {
		key := absOrClean(dir)
		if !seen[key] {
			seen[key] = true
			dirs = append(dirs, dir)
		}
	}
	for _, dir := range c.cfg.PackDirs {
		add(dir)
	}
	for _, rigDirs := range c.cfg.RigPackDirs {
		for _, dir := range rigDirs {
			add(dir)
		}
	}
	sort.Strings(dirs)
	return dirs
}
