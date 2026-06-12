# SWGC Sender Lab

This manual harness documents which parts of an outgoing SWGC message can and
cannot affect sender identity.

It sends a matrix of clearly labelled messages:

1. A standard text control.
2. A baseline `GroupStatusMessageV2`.
3. SWGC with `ContextInfo.Participant` claiming the first group admin.
4. SWGC with `StatusQuotedMessage.OriginalStatusID.Participant` claiming that
   admin.
5. SWGC with `ContextInfo.Participant` claiming another non-admin member.
6. SWGC with `StatusQuotedMessage.OriginalStatusID.Participant` claiming that
   member.
7. Forward/status attribution claiming another member.
8. `MessageAssociation.ParentMessageKey` claiming another member.
9. Root `<meta>` thread sender claiming another member.
10. `DeviceSentMessage.DestinationJID` claiming another member.
11. Synthetic forwarded-newsletter attribution.

The claim cases only change protobuf payload metadata. Whatsmeow still chooses
the transport sender from the authenticated session and encrypts the group
message with that session's Signal Sender Key. The harness also waits briefly
for a matching event echo and prints its parsed `MessageInfo` if observed.

The member cases are skipped when the target group has no other non-admin
member.

These cases intentionally stay inside exported whatsmeow APIs and protobuf
payloads. The lab does not alter raw message-envelope attributes, session
identity keys, or another participant's Signal Sender Key.

Dry run:

```sh
go run ./cmd/swgc-lab \
  -session sessions/your-session.db \
  -group 120363000000000000@g.us \
  -expect-name Manual
```

Live run:

```sh
go run ./cmd/swgc-lab \
  -session sessions/your-session.db \
  -group 120363000000000000@g.us \
  -expect-name Manual \
  -live \
  -confirm I_UNDERSTAND_THIS_SENDS_TO_WHATSAPP
```

Attribution-only live run:

```sh
go run ./cmd/swgc-lab \
  -session sessions/your-session.db \
  -group 120363000000000000@g.us \
  -expect-name Manual \
  -suite attribution \
  -live \
  -confirm I_UNDERSTAND_THIS_SENDS_TO_WHATSAPP
```

Hidetag live run:

```sh
go run ./cmd/swgc-lab \
  -session sessions/your-session.db \
  -group 120363000000000000@g.us \
  -expect-name Manual \
  -suite hidetag \
  -live \
  -confirm I_UNDERSTAND_THIS_SENDS_TO_WHATSAPP
```

The hidetag suite sends one ordinary text message without visible `@user`
tokens. Every current group participant JID is included in
`ContextInfo.MentionedJID`.

SWGC hidetag live run:

```sh
go run ./cmd/swgc-lab \
  -session sessions/your-session.db \
  -group 120363000000000000@g.us \
  -expect-name Manual \
  -suite swgc-hidetag \
  -live \
  -confirm I_UNDERSTAND_THIS_SENDS_TO_WHATSAPP
```

This variant places the same mention metadata on the inner
`ExtendedTextMessage` wrapped by `GroupStatusMessageV2`.

Status mention live run:

```sh
go run ./cmd/swgc-lab \
  -session sessions/your-session.db \
  -group 120363000000000000@g.us \
  -expect-name Manual \
  -suite status-mention \
  -live \
  -confirm I_UNDERSTAND_THIS_SENDS_TO_WHATSAPP
```

This suite posts one ordinary WhatsApp Status using the account's existing
Status privacy audience, marks the first group admin in `mentioned_users`, then
sends that admin a `StatusMentionMessage` referencing the Status message ID.
It refuses to send when the admin is not in the current Status audience.

SWGC group mention live run:

```sh
go run ./cmd/swgc-lab \
  -session sessions/your-session.db \
  -group 120363000000000000@g.us \
  -expect-name Manual \
  -suite swgc-group-mention \
  -live \
  -confirm I_UNDERSTAND_THIS_SENDS_TO_WHATSAPP
```

This suite sends one `GroupStatusMessageV2` with every current participant in
`ContextInfo.MentionedJID`. It then sends each other participant a direct
`GroupStatusMentionMessage` that references the SWGC ID. The direct messages
are protocol notifications rather than visible chat text.

SWGC followed by an ordinary group mention:

```sh
go run ./cmd/swgc-lab \
  -session sessions/your-session.db \
  -group 120363000000000000@g.us \
  -expect-name Manual \
  -suite swgc-then-group-mention \
  -live \
  -confirm I_UNDERSTAND_THIS_SENDS_TO_WHATSAPP
```

This sends two group messages: one SWGC followed by one standard
`ExtendedTextMessage` whose `ContextInfo.MentionedJID` contains every current
participant.

Stop the main application before a live run so two clients do not use the same
session concurrently. An ACK means the stanza was accepted; verify rendering
from another device because ACK alone does not prove delivery or display.
