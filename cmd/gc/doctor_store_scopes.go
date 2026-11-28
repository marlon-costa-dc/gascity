package main

import (
	"strings"

	"github.com/gastownhall/gascity/internal/config"
	"github.com/gastownhall/gascity/internal/fsys"
	"github.com/gastownhall/gascity/internal/suspensionstate"
)

// doctorStoreScope is one bead store a store-scanning doctor check reads.
type doctorStoreScope struct {
	label string
	path  string
	rig   string
}

// doctorStoreScopes returns the city followed by every path-bearing rig a
// store-scanning check reads. A check about live routed work skips a rig the
// operator paused at runtime (skipPaused), because nothing claims work there
// until it resumes. A repair check (skipPaused false) still reads it, because
// its data stays broken while paused and `gc doctor --fix` must reach it; only
// a rig suspended in city.toml is left out.
func doctorStoreScopes(cfg *config.City, cityPath string, skipPaused bool) []doctorStoreScope {
	scopes := []doctorStoreScope{{label: "city", path: cityPath}}
	if cfg == nil {
		return scopes
	}
	var suspState suspensionstate.State
	if skipPaused {
		suspState, _ = loadSuspensionState(fsys.OSFS{}, cityPath)
	}
	for _, rig := range cfg.Rigs {
		if strings.TrimSpace(rig.Path) == "" {
			continue
		}
		if skipPaused && suspensionstate.EffectiveRigSuspended(suspState, rig.Name, rig.EffectiveSuspendedOnStart()) {
			continue
		}
		if !skipPaused && rig.Suspended {
			continue
		}
		scopes = append(scopes, doctorStoreScope{label: "rig " + rig.Name, path: rig.Path, rig: rig.Name})
	}
	return scopes
}
