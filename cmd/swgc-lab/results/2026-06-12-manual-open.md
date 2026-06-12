# Live Result: Manual Group, Open Permission

Date: 2026-06-12 14:38 Asia/Jakarta

Target:

- Group: `Manual`
- JID: `120363424223887015@g.us`
- Announce-only: `false`
- Participants: `2`

Authenticated session:

- PN: `6281125066076@s.whatsapp.net`
- LID: `81669541347408@lid`
- Role: member (`admin=false`, `super_admin=false`)

Payload-only claimed admin:

- JID: `112584581750837@lid`

Results:

| Case | Payload claim | Result | Response sender | Echo |
| --- | --- | --- | --- | --- |
| Standard text control | None | ACK | `81669541347408:4@lid` | Not observed |
| Baseline SWGC | None | ACK | `81669541347408:4@lid` | Not observed |
| `ContextInfo.Participant` | Admin JID | ACK | `81669541347408:4@lid` | Not observed |
| `StatusQuotedMessage.OriginalStatusID.Participant` | Admin JID | ACK | `81669541347408:4@lid` | Not observed |

Message IDs:

- Standard: `3EB0E0AA984E4E207A3C2E`
- Baseline SWGC: `3EB099D93C2FD93201F497`
- Context claim: `3EB06E58C0247DE907641B`
- Status-key claim: `3EB08D9B8F16241F427CCD`

Conclusion:

Both admin-claim fields are payload references. They did not replace the
transport sender, which remained the authenticated member device in every
case. No matching outgoing event echo was observed within 1.5 seconds, so the
test does not use echo metadata as evidence.

An ACK confirms stanza acknowledgement, not final rendering. Visual delivery
should be checked from the admin device.
