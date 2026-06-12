# Live Result: Sender Candidate Matrix

Date: 2026-06-12 14:50 Asia/Jakarta

Environment:

- Group: `Manual`
- JID: `120363424223887015@g.us`
- Announce-only: `false`
- Authenticated member PN: `6287817739901@s.whatsapp.net`
- Authenticated member LID: `197264089821252@lid`
- Actual response sender: `197264089821252:9@lid`
- Claimed admin: `112584581750837@lid`
- Claimed member: `81669541347408@lid`

Results:

| Case | Result | Actual response sender | Interpretation |
| --- | --- | --- | --- |
| Standard control | ACK | Authenticated session | Control |
| Baseline SWGC | ACK | Authenticated session | Normal SWGC |
| `ContextInfo.Participant` admin | ACK | Authenticated session | Quote/context metadata only |
| Status key admin | ACK | Authenticated session | Referenced status key only |
| `ContextInfo.Participant` member | ACK | Authenticated session | Quote/context metadata only |
| Status key member | ACK | Authenticated session | Referenced status key only |
| Forward/status attribution member | ACK | Authenticated session | Attribution metadata accepted |
| `MessageAssociation` member parent | ACK | Authenticated session | Association metadata accepted |
| Root thread `<meta>` member | Error 420 | None | Server rejected malformed/nonexistent thread context |
| `DeviceSentMessage` member destination | ACK | Authenticated session | Wrapper accepted; no sender transfer |
| Forwarded newsletter attribution | ACK | Authenticated session | Newsletter attribution metadata accepted |

Message IDs:

- Standard: `3EB07C550D9F3AC6BF0296`
- Baseline SWGC: `3EB0AA8106FC38AA27D1B3`
- Context admin: `3EB0329F10730792B197A2`
- Status admin: `3EB0C2E3B85A0C1E82FA57`
- Context member: `3EB07EFEE5A2547A941A23`
- Status member: `3EB00794739BFBAF7C2BF6`
- Forward attribution: `3EB02278C090A28C6BFB77`
- Association: `3EB077149FDB9360398B5E`
- Device-sent wrapper: `3EB0BEBF439688CD66CE65`
- Newsletter attribution: `3EB0F74C75DB313562F107`

The installed whatsmeow source does not define a semantic description for
server error `420`; this result only establishes that the tested root thread
metadata combination was rejected.

Conclusion:

Several payload fields can modify references, forwarding labels, associations,
or rendering context and still receive an ACK. None changed
`SendResponse.Sender`. Sender identity remained bound to the authenticated
device and its Signal Sender Key.

No outgoing echo was observed within 1.5 seconds for the ACK cases. Visual
rendering must be inspected on another participant's device before claiming
that a particular attribution was displayed.
