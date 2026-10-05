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
mayor              # city-scope session alias
myrig/witness      # rig-qualified agent
human              # the operator (canonical alias; never a session)
```

The sender defaults to `$GC_SESSION_ID`, then `$GC_ALIAS`, then
`$GC_AGENT`, then `"human"`, so a command typed at the city root outside a
session sends **as human**. Use `--from <identity>` to override.

## Sending

```
gc mail send <to> -m "message body"                    # Send a message
gc mail send <to> -s "Subject" -m "message body"       # Send with subject
gc mail send <to> -m "Priority task" --notify          # Send and nudge the recipient
gc mail send --all -m "Status update"                  # Broadcast to live sessions
gc mail reply <id> -m "reply body"                     # Reply to a message
gc mail reply <id> -s "Re: topic" -m "reply body"      # Reply with subject
```

For repeating notifications (a patrol or cooldown order that re-detects the
same condition every run), add `--dedup <key>`: the send is suppressed (exit
0) while a previous message with the same key is still live in the same
mailbox, so the recipient gets at most one copy per stream instead of one per
interval. Once they archive it, the stream may alert again. Dedup needs a
provider that can query its own message history; the built-in one can, and a
provider that cannot sends normally and says so on stderr, so treat
suppression as best effort rather than a guarantee.

```
gc mail send worker -s "disk warning" -m "..." --dedup "disk-warn:hq"
```

Delivery and wake semantics:

- **Unread mail alone never wakes the recipient.** It is read on the
  recipient's next turn.
- `--notify` nudges the recipient. A running session receives the nudge when
  it is idle. A managed session that is not running gets the nudge queued
  and **is woken** to receive it. Do not use `--notify` while the city is
  intentionally at rest: it breaks the rest state.
- `--all` reaches live sessions only; it excludes the sender and human.

## Reading

```
gc mail inbox                          # List unread messages
gc mail count                          # Count unread messages
gc mail peek <id>                      # Preview a message without marking read
gc mail read <id>                      # Read a message (marks as read)
gc mail thread <id>                    # Show full conversation thread
```

`peek` before `read` when triaging a large inbox: `read` marks the message
as read as a side effect.

## Managing

```
gc mail archive <id>                   # IRRECOVERABLE: closes the underlying bead, despite the name
gc mail mark-read <id>                 # Mark as read without displaying
gc mail mark-unread <id>              # Mark as unread
gc mail delete <id>                    # IRRECOVERABLE: alias for archive; closes the underlying bead
gc mail check                          # Check for new mail (used in hooks)
```

`archive` and `delete` are the same operation under two names: both close the
message's underlying bead, which takes it out of every mail view for good.
Neither files it away for later reading, and there is no reversible "put this
away" path. Prefer `mark-read` when you want a message out of the unread count
while it stays readable.

## Coordinating with agents outside the gc registry

Sessions that do not run inside gc (for example background agents started
outside the city) are **not addressable** through `gc mail`. Reach them
through a durable bead in their rig store instead: create it with
`gc bd create "<coordination title>" --rig <rig>`, add to it with
`gc bd comment <id> "..."`, and read their replies there. The bead is the
thread; mail remains the session-addressable lane.
