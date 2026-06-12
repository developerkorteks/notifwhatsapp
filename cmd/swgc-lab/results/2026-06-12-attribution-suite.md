# Live Result: Attribution-Only Suite

Date: 2026-06-12 14:55 Asia/Jakarta

Target:

- Group: `Manual`
- JID: `120363424223887015@g.us`
- Announce-only: `false`
- Participants: `3`

Authenticated sender:

- PN: `6287817739901@s.whatsapp.net`
- LID: `197264089821252@lid`
- Response sender: `197264089821252:9@lid`
- Role: member

Referenced other member:

- LID: `81669541347408@lid`

Results:

| Case | Result | Message ID | Actual sender |
| --- | --- | --- | --- |
| Quote context referencing another member | ACK | `3EB08CED7CA1AEC8CA63EC` | Authenticated session |
| Forwarded/status attribution | ACK | `3EB07C00CAE1AC3C922800` | Authenticated session |
| Synthetic channel/newsletter attribution | ACK | `3EB052E95BDCCD56453188` | Authenticated session |
| Parent/status association referencing another member | ACK | `3EB0E5DD7B84CA527A6783` | Authenticated session |

No matching outgoing event echo was observed within 1.5 seconds.

These results prove that the server acknowledged the four payload structures.
They do not prove how every attribution was rendered. Compare the four labelled
messages from another participant's WhatsApp client.

None of the structures changed the transport sender.
