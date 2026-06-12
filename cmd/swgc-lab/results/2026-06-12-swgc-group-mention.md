# Live Result: SWGC Mentioning All Manual Group Members

Date: 2026-06-12 15:21 Asia/Jakarta

Target:

- Group: `Manual`
- JID: `120363424223887015@g.us`
- Participants: `3`
- Authenticated sender: `6287817739901@s.whatsapp.net`

Mention targets:

- Sender: `6287817739901@s.whatsapp.net`
- Admin: `6285117557905@s.whatsapp.net`
- Other member: `6281125066076@s.whatsapp.net`

Working flow:

1. Send one `GroupStatusMessageV2` to the group.
2. Put all three participant PNs in the inner
   `ExtendedTextMessage.ContextInfo.MentionedJID`.
3. For each participant other than the sender, send a direct
   `GroupStatusMentionMessage`.
4. Its `ProtocolMessage` uses `STATUS_MENTION_MESSAGE` and references the
   group JID, SWGC message ID, and authenticated sender LID.
5. Add `meta is_status_mention="true"` to each direct notification.

Results:

| Stage | Target | Result | Message ID |
| --- | --- | --- | --- |
| SWGC | Group `Manual` | ACK | `3EB0DCDD2E0E9B102024DE` |
| Group-status mention notification | Admin | ACK | `3EB078101214D3E7AA8E30` |
| Group-status mention notification | Other member | ACK | `3EB057E5298B04A73F73A9` |

Rejected variants:

- Attaching `meta/mentioned_users` directly to the SWGC stanza was rejected
  with server error `479`.
- Sending `GroupStatusMentionMessage` to the group JID was rejected with
  server error `479`.

Conclusion:

The server accepted the SWGC plus one direct group-status mention notification
per other participant. This mirrors the ordinary Status mention flow: the
content and notification are separate messages.

ACK confirms server acceptance. Whether each recipient client displayed the
SWGC mention notification must be checked on the admin and other member
devices.

## Repeat Run

Date: 2026-06-12 15:23 Asia/Jakarta

| Stage | Target | Result | Message ID |
| --- | --- | --- | --- |
| SWGC | Group `Manual` | ACK | `3EB05CA9CB5275F99ABB3C` |
| Group-status mention notification | Admin | ACK | `3EB09FE08CD079CE22AA3C` |
| Group-status mention notification | Other member | ACK | `3EB03B473701F4D2BFF40D` |

Both expected direct mention notifications were acknowledged.
