# Sender-Related Candidate Map

This map is based on the installed whatsmeow source and protobuf schema.

## Transport identity

- `SendMessage` obtains `ownID` from the authenticated device store.
- LID-addressed groups replace it with the authenticated account's own LID.
- `SendResponse.Sender` records that selected identity.

There is no exported `SendRequestExtra` field for replacing this sender.

## Cryptographic identity

Group ciphertext uses a Signal Sender Key created from:

```text
target group JID + authenticated client's own LID Signal address
```

Changing protobuf fields does not provide another participant's Sender Key.

## Payload references tested

- `ContextInfo.Participant`
- `StatusQuotedMessage.OriginalStatusID.Participant`
- `ContextInfo` forwarding and status attribution
- `MessageAssociation.ParentMessageKey.Participant`
- `SendRequestExtra.Meta.ThreadMessageSenderJID`
- `DeviceSentMessage.DestinationJID`
- `ForwardedNewsletterMessageInfo`

These may influence quote, forwarding, attribution, association, thread, or
rendering semantics. They are not transport authentication.

## Deliberately excluded

- Raw root `<message participant=...>` spoofing.
- Reusing or extracting another account's identity keys.
- Importing another participant's Signal Sender Key.
- Modifying whatsmeow internals to mislabel encrypted traffic.

Those are not normal message-format experiments and could cross account or
authorization boundaries.
