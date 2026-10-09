package beadmeta

// IssueTypeMessage is the bead issue type every mail message carries. It is the
// single spelling of the message-bead class: mail creates beads of this type
// addressed to the recipient through their assignee, and every work-discovery
// surface (bd ready exclusions, the ephemeral ready tiers of the generated
// work_query, the hook claim path) must exclude it, because mail is read, never
// claimed as work.
const IssueTypeMessage = "message"
