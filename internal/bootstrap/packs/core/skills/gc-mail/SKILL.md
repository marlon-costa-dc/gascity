---
name: gc-mail
description: Sending and reading messages between agents
---

# Messaging (Mail)

Mail is bead-based messaging between agents. Messages are beads with
type=message, stored in the bead store.

## Addressing

Recipients are **session aliases**, rig-qualified agents, or the human:

```
mayor              # city-scope session alias (the qualified gastown.mayor also resolves)
myrig/witness      # rig-qualified agent
human              # the operator (canonical alias; never a session)
```

The sender defaults to `$GC_SESSION_ID`, then `$GC_ALIAS`, then
`$GC_AGENT`, then `"human"` — a command typed at the city root outside a
session sends **as human**. Use `--from <identity>` to override.

## Sending

```
gc mail send <to> -m "message body"                    # Send a message
gc mail send <to> -s "Subject" -m "message body"       # Send with subject
gc mail send polecat "Priority task" --notify          # request a recipient turn
gc mail send --all "Status update"                     # broadcast to LIVE sessions
gc mail reply <id> -m "reply body"                     # Reply to a message
gc mail reply <id> -s "Re: topic" -m "reply body"      # Reply with subject
```

Wake semantics (managed city):

- **Unread mail alone never wakes the recipient.** It is read on the
  recipient's next wake.
- `--notify` requests a recipient turn and **can wake a non-running
  recipient**. Do not use `--notify` while the city is intentionally at
  rest — it breaks the rest state.
- `--all` covers live sessions only (excludes sender and human).

## Reading

```
gc mail inbox                          # List unread messages
gc mail count                          # Count unread messages
gc mail peek <id>                      # Preview a message without marking read
gc mail read <id>                      # Read a message (marks as read)
gc mail thread <id>                    # Show full conversation thread
```

`peek` before `read` when triaging a large inbox — `read` marks as read
as a side effect.

## Managing

```
gc mail archive <id>                   # IRRECOVERABLE: deletes the underlying bead, despite the name
gc mail mark-read <id>                 # Mark as read without displaying
gc mail mark-unread <id>              # Mark as unread
gc mail delete <id>                    # IRRECOVERABLE: alias for archive; deletes the underlying bead
gc mail check                          # Check for new mail (used in hooks)
```

`archive` and `delete` are the same operation under two names — both delete
the message's underlying bead outright; neither files it away for later
reading. There is no reversible "put this away" path. Prefer `mark-read` when
you want a message out of the unread count without destroying it. Never
archive coordination threads.

## Coordinating with agents outside the gc registry

Sessions that do not run inside gc (e.g. background claude agents) are
**not addressable** through `gc mail`. Reach them through durable beads in
their rig store instead (`gc bd create "<coordination title>" --rig <rig>`,
then `gc bd comment <prefix-id> ...` on that bead), and read their replies
there. The bead is the thread; mail remains the session-addressable lane.
