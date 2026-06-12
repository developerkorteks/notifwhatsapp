# Live Result: Ordinary Status Mentioning Manual Group Admin

Date: 2026-06-12 15:18 Asia/Jakarta

Authenticated session:

- PN: `6287817739901@s.whatsapp.net`
- LID: `197264089821252@lid`

Mention target:

- Source: first admin of group `Manual`
- Admin PN: `6285117557905@s.whatsapp.net`
- Admin LID: `112584581750837@lid`
- Present in current Status privacy audience: `true`
- Current Status audience size: `336`

Flow:

1. Sent an ordinary text Status to `status@broadcast`.
2. Added `ContextInfo.MentionedJID` containing the admin PN.
3. Added a `meta/mentioned_users/to` node containing the admin PN.
4. Sent a direct `StatusMentionMessage` to the admin.
5. Its `ProtocolMessage` used type `STATUS_MENTION_MESSAGE` and referenced
   the Status message ID.

Results:

| Stage | Result | Message ID | Response sender |
| --- | --- | --- | --- |
| Ordinary Status with mention metadata | ACK | `3EB04777FF265E1A2AFE99` | `6287817739901:9@s.whatsapp.net` |
| Direct status-mention notification | ACK | `3EB0D386B8375556FCD418` | `6287817739901:9@s.whatsapp.net` |

Warnings:

- Several recipients in the 336-contact Status audience had no established
  Signal session and could not be encrypted.
- WhatsApp returned a different participant-list hash for the Status
  broadcast, so some audience devices may not have received it.
- The direct mention notification to the admin was acknowledged without an
  error.
- No matching outgoing echo was observed for either message within 1.5
  seconds.

Conclusion:

Both the Status and its dedicated mention notification were accepted by the
server. Final rendering of the mention must be checked on the admin account.
An ACK does not prove that the receiving WhatsApp client displayed the mention
notification.
