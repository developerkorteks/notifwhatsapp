# Live Result: Hidetag in Manual Group

Date: 2026-06-12 15:09 Asia/Jakarta

Target:

- Group: `Manual`
- JID: `120363424223887015@g.us`
- Announce-only: `false`
- Participants: `3`

Authenticated session:

- PN: `6287817739901@s.whatsapp.net`
- LID: `197264089821252@lid`
- Role: member (`admin=false`, `super_admin=false`)

Payload:

- Visible text: `[SWGC LAB HIDETAG] Tes mention metadata tanpa tag yang terlihat`
- Visible `@user` tokens: `0`
- `ContextInfo.MentionedJID` entries: `3`
- Mention targets: every participant returned by current group metadata

Result:

- Status: ACK
- Duration: `922ms`
- Message ID: `3EB0FB031CFF96B35ADF3A`
- Response sender: `197264089821252:9@lid`
- Matching outgoing echo: not observed within 1.5 seconds

Conclusion:

WhatsApp accepted an ordinary group text message whose visible body contained
no `@user` tokens while `ContextInfo.MentionedJID` contained all three group
participants. The transport sender remained the authenticated member device.

An ACK confirms stanza acknowledgement, not whether each receiving client
rendered a mention highlight or generated a notification. That behavior must
be checked on another participant's WhatsApp client.
