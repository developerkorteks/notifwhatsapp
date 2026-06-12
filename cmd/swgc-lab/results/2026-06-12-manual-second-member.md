# Live Result: Second Member Session

Date: 2026-06-12 14:45 Asia/Jakarta

Target:

- Group: `Manual`
- JID: `120363424223887015@g.us`
- Announce-only: `false`
- Participants: `3`

Authenticated session:

- PN: `6287817739901@s.whatsapp.net`
- LID: `197264089821252@lid`
- Device sender returned by WhatsApp: `197264089821252:9@lid`
- Role: member (`admin=false`, `super_admin=false`)

Payload-only identities:

- Admin claim: `112584581750837@lid`
- Other member claim: `81669541347408@lid`

Results:

| Case | Claimed sender | Result | Response sender | Echo |
| --- | --- | --- | --- | --- |
| Standard text control | None | ACK | `197264089821252:9@lid` | Not observed |
| Baseline SWGC | None | ACK | `197264089821252:9@lid` | Not observed |
| `ContextInfo.Participant` | Admin | ACK | `197264089821252:9@lid` | Not observed |
| Admin `OriginalStatusID.Participant` | Admin | ACK | `197264089821252:9@lid` | Not observed |
| `ContextInfo.Participant` | Other member | ACK | `197264089821252:9@lid` | Not observed |
| Member `OriginalStatusID.Participant` | Other member | ACK | `197264089821252:9@lid` | Not observed |

Message IDs:

- Standard: `3EB088B68D07D5337F2B82`
- Baseline SWGC: `3EB09B6EEF360480AC5C17`
- Context admin claim: `3EB09857C01E8C17EB7BC5`
- Status-key admin claim: `3EB0F26618DEA5C49A09F1`
- Context member claim: `3EB032615CFA82D3DFB831`
- Status-key member claim: `3EB000EEDEF8C858A8C331`

Conclusion:

Claims for both an admin and another ordinary member remained protobuf payload
references. Every server response identified the authenticated second member
device as the sender.

This also confirms the result across two independent member sessions. Changing
the claimed participant does not transfer the transport identity or Signal
Sender Key of another participant.

An ACK confirms stanza acknowledgement, not final rendering. Visual delivery
should be checked from another group participant's device.
