package config

import (
	"strings"
	"testing"

	"github.com/gastownhall/gascity/internal/beadmeta"
)

// ephemeralMessageAndWorkFixture mirrors two `bd query --json
// 'ephemeral=true AND status=open'` rows assigned to the same pool agent: a
// mail message (beadmail creates every message as an ephemeral bead whose
// assignee is the recipient) and a real unblocked work step.
//
// Measured 2026-10-09 (gct-lv2pe): `gc hook bd.dog` returned a broadcast mail
// copy addressed to the dog as its only ready work. The claim path skips
// messages (hookClaimCandidateIsMessage), so every dog session drained without
// claiming and the pool respawned roughly every four minutes, never serving the
// routed work behind it.
const ephemeralMessageAndWorkFixture = `[
  {
    "id": "gc-wisp-mail1",
    "title": "[coord] broadcast copy",
    "status": "open",
    "issue_type": "` + beadmeta.IssueTypeMessage + `",
    "assignee": "bd.dog",
    "created_at": "2026-10-03T03:01:13Z",
    "ephemeral": true,
    "dependency_count": 0
  },
  {
    "id": "gc-wisp-work1",
    "title": "Dolt maintenance step",
    "status": "open",
    "issue_type": "task",
    "assignee": "bd.dog",
    "created_at": "2026-10-09T14:00:00Z",
    "ephemeral": true,
    "dependency_count": 0
  }
]`

// TestEphemeralReadyProbeNeverServesMailAsWork pins that the ephemeral
// assigned-ready tier serves the work step and never the mail message
// assigned to the same identity.
func TestEphemeralReadyProbeNeverServesMailAsWork(t *testing.T) {
	filter := legacyEphemeralReadyFilterJQ(`select((.assignee // "") == $id)`, 0, false)

	got := runJQFilter(t, filter, ephemeralMessageAndWorkFixture, "--arg", "id", "bd.dog", "-c")
	got = runJQFilter(t, "map(.id)", got, "-c")

	if got != `["gc-wisp-work1"]` {
		t.Errorf("ephemeral ready probe served mail as work.\nwant: [\"gc-wisp-work1\"]\ngot:  %s", got)
	}
}

// TestEphemeralDependencyCandidateProbeNeverServesMail pins the slow-path
// twin: a mail message with a nonzero dependency_count must not reach the
// dependency-enrichment candidates either.
func TestEphemeralDependencyCandidateProbeNeverServesMail(t *testing.T) {
	filter := ephemeralReadyDependencyCandidateFilterJQ(`select((.assignee // "") == $id)`, 0, false)
	payload := strings.ReplaceAll(ephemeralMessageAndWorkFixture, `"dependency_count": 0`, `"dependency_count": 1`)

	got := runJQFilter(t, filter, payload, "--arg", "id", "bd.dog", "-c")
	got = runJQFilter(t, "map(.id)", got, "-c")

	if got != `["gc-wisp-work1"]` {
		t.Errorf("ephemeral dependency-candidate probe surfaced mail.\nwant: [\"gc-wisp-work1\"]\ngot:  %s", got)
	}
}
