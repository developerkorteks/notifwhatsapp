# Live Result: SWGC Hidetag in Manual Group

Date: 2026-06-12 15:11 Asia/Jakarta

Target:

- Group: `Manual`
- JID: `120363424223887015@g.us`
- Announce-only: `false`
- Participants: `3`

Authenticated session:

- PN: `6287817739901@s.whatsapp.net`
- LID: `197264089821252@lid`
- Role: member (`admin=false`, `super_admin=false`)

Payload structure:

```text
GroupStatusMessageV2
└── FutureProofMessage.Message
    └── ExtendedTextMessage
        ├── Text: SWGC body without visible @user tokens
        └── ContextInfo.MentionedJID: all 3 group participants
```

Result:

- Status: ACK
- Duration: `669ms`
- Message ID: `3EB01FAE3637104213CD68`
- Response sender: `197264089821252:9@lid`
- Matching outgoing echo: not observed within 1.5 seconds

Comparison:

- Ordinary hidetag message: ACK, message ID `3EB0FB031CFF96B35ADF3A`
- SWGC hidetag message: ACK, message ID `3EB01FAE3637104213CD68`
- Both carried zero visible `@user` tokens and three mention JIDs.
- Both retained the authenticated account as transport sender.

Conclusion:

WhatsApp acknowledged mention metadata inside the SWGC payload. An ACK only
confirms acceptance of the stanza and does not prove that recipient clients
honored the nested metadata as a mention highlight or notification. Visual
behavior must be checked from another participant's WhatsApp client.
