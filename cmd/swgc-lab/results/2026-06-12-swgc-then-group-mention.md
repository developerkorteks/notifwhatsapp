# Live Result: SWGC Followed by Ordinary Group Mention

Date: 2026-06-12 15:25 Asia/Jakarta

Target:

- Group: `Manual`
- JID: `120363424223887015@g.us`
- Participants mentioned: `3`
- Authenticated sender: `6287817739901@s.whatsapp.net`

Flow:

1. Send one ordinary `GroupStatusMessageV2` to the group.
2. Send one standard `ExtendedTextMessage` to the same group.
3. Put all participant PNs in the second message's
   `ContextInfo.MentionedJID`.

Results:

| Stage | Result | Message ID | Response sender |
| --- | --- | --- | --- |
| SWGC | ACK | `3EB0507E77198A3BB6BB88` | `197264089821252:9@lid` |
| Ordinary group mention | ACK | `3EB043C97A2E52551DB6AC` | `197264089821252:9@lid` |

Conclusion:

The mention stanza was sent directly to the group and accepted by the server.
This is a two-message flow because WhatsApp rejected
`GroupStatusMentionMessage` addressed to the group with error `479`.

Final mention notification behavior must be checked on another participant's
client.
