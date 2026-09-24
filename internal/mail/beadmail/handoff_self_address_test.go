package beadmail

import (
	"testing"

	"github.com/gastownhall/gascity/internal/beads"
	"github.com/gastownhall/gascity/internal/mail"
	"github.com/gastownhall/gascity/internal/session"
)

// newPooledFleet returns a store holding the production shape of a pool
// member -- a session bead with a session_name ("builder-<id>") but no alias,
// so the only address its own commands can name it by is the raw session ID --
// next to an aliased session that must never see the pool member's mail.
func newPooledFleet(t *testing.T) (beads.Store, beads.Bead) {
	t.Helper()
	store := beads.NewMemStore()
	builder, err := store.Create(beads.Bead{
		Type:     session.BeadType,
		Labels:   []string{session.LabelSession},
		Metadata: map[string]string{"session_name": "builder-pool"},
	})
	if err != nil {
		t.Fatalf("Create pooled session: %v", err)
	}
	if _, err := store.Create(beads.Bead{
		Type:   session.BeadType,
		Labels: []string{session.LabelSession},
		Metadata: map[string]string{
			"alias":        "mayor",
			"session_name": "mayor",
		},
	}); err != nil {
		t.Fatalf("Create mayor session: %v", err)
	}
	return store, builder
}

// TestSendHandoffSelfAddressedPersistsOneAddress proves that a self-handoff --
// the shape gc handoff --auto sends from the PreCompact hook, where the caller
// passes ONE address as both From and To -- must not persist as two different
// addresses.
//
// SendHandoff normalizes the From side through resolveSenderRoute ->
// senderDisplayAddress. For a pooled session with no alias, addressed by its
// own raw session ID, that falls through to the session_name branch and
// rewrites From to "builder-<...>", while intent.To used to be stored verbatim
// as the bead Assignee. Both strings name the same session and delivery is
// unaffected, but the persisted row read as agent-to-agent mail to every
// consumer that compares the two fields.
func TestSendHandoffSelfAddressedPersistsOneAddress(t *testing.T) {
	store, builder := newPooledFleet(t)
	p := New(store)

	msg, err := p.SendHandoff(mail.HandoffIntent{
		From:        builder.ID,
		To:          builder.ID,
		Subject:     "context cycle",
		ThreadID:    "thread-deadbeef",
		ExtraLabels: []string{mail.AutoHandoffLabel, mail.ArchiveAfterInjectLabel},
	})
	if err != nil {
		t.Fatalf("SendHandoff: %v", err)
	}

	b, err := store.Get(msg.ID)
	if err != nil {
		t.Fatalf("Get %s: %v", msg.ID, err)
	}
	if b.From != b.Assignee {
		t.Errorf("self-handoff persisted as cross-agent mail: bead From = %q, Assignee = %q; want both to name the same address",
			b.From, b.Assignee)
	}
	if msg.From != msg.To {
		t.Errorf("self-handoff message From = %q, To = %q; want both to name the same address", msg.From, msg.To)
	}
}

// TestSelfHandoffFromPooledSessionStaysInItsOwnInbox is the delivery guard:
// the marker reaches exactly the session that sent it, under either rendering
// of its address, and no other mailbox sees it. It passes before and after the
// fix above, proving that change affects only how the row reads.
func TestSelfHandoffFromPooledSessionStaysInItsOwnInbox(t *testing.T) {
	store, builder := newPooledFleet(t)
	p := New(store)

	msg, err := p.SendHandoff(mail.HandoffIntent{
		From:        builder.ID,
		To:          builder.ID,
		Subject:     "context cycle",
		ThreadID:    "thread-deadbeef",
		ExtraLabels: []string{mail.AutoHandoffLabel, mail.ArchiveAfterInjectLabel},
	})
	if err != nil {
		t.Fatalf("SendHandoff: %v", err)
	}

	for _, selector := range []string{builder.ID, builder.Metadata["session_name"]} {
		got, err := p.Inbox(selector)
		if err != nil {
			t.Fatalf("Inbox(%q): %v", selector, err)
		}
		if len(got) != 1 || got[0].ID != msg.ID {
			t.Errorf("Inbox(%q) = %d message(s), want exactly the marker %s", selector, len(got), msg.ID)
		}
	}

	others, err := p.Inbox("mayor")
	if err != nil {
		t.Fatalf("Inbox(mayor): %v", err)
	}
	if len(others) != 0 {
		t.Errorf("Inbox(mayor) = %d message(s), want 0; a self-addressed compaction marker leaked into another agent's inbox", len(others))
	}
}
