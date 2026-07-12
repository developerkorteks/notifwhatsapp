---
name: swgc-close-friends
description: How to implement a Close-Friends audience for WhatsApp group-status (SWGC) messages in whatsmeow for this project, including the proto fields, emoji/name customization, the Baileys reference, and the reuse pattern
source: auto-skill
extracted_at: '2026-07-12T11:09:05.170Z'
---

# SWGC Close-Friends Audience (whatsmeow)

When extending SWGC (group status) in `juraganxl-notif`, the **audience** (who can
see it) is a SEPARATE concern from the **media type** and from the **visual style**.
This is the single most common point of confusion — captured here so it is not
rediscovered each time.

## Key gotcha: `statusSourceType` is NOT the audience
`ContextInfo.StatusSourceType` (whatsmeow proto `waE2E`) is an **enum of media
type**, not audience:
```
ContextInfo_IMAGE            = 0
ContextInfo_VIDEO            = 1
ContextInfo_GIF              = 2
ContextInfo_AUDIO            = 3
ContextInfo_TEXT             = 4
ContextInfo_MUSIC_STANDALONE = 5
```
The Baileys bot `swgc.js`/`swgcall.js` sets `statusSourceType` to exactly these
values (image→0, video→1, audio→3, text→4) — it is just tagging the media
type. There is **no `CLOSE_FRIENDS` value** in `StatusSourceType`. Do NOT look
here for audience control.

## The real audience field: `StatusAudienceMetadata`
Close-friends (and custom-list) audience is carried by **`ContextInfo.StatusAudienceMetadata`**
on the **inner content's `ContextInfo`** (e.g. `ExtendedTextMessage.ContextInfo`,
`ImageMessage.ContextInfo`, etc. — NOT the message-level `MessageContextInfo`).

Verified whatsmeow proto fields (`waE2E.WAWebProtobufsE2E.pb.go`):
```
ContextInfo.StatusAudienceMetadata          // getter ~line 8705
ContextInfo_StatusAudienceMetadata {
    AudienceType *ContextInfo_StatusAudienceMetadata_AudienceType  // field 1
    ListName    *string                                          // field 2
    ListEmoji   *string                                          // field 3
}
ContextInfo_StatusAudienceMetadata_UNKNOWN        = 0
ContextInfo_StatusAudienceMetadata_CLOSE_FRIENDS = 1   // <- close friends
ContextInfo_StatusAudienceMetadata_CUSTOM_LIST    = 2   // <- custom list (needs ListName/ListEmoji)
```
Reference implementation that proves this: the npm package **`castleys-community`**
(`groupStatusV2`), which sets `contextInfo.statusAudienceMetadata = { audienceType: 1 }`
for `audience: 'close_friends'` and `{ audienceType: 2, listName, listEmoji }`
for custom lists. That package also confirms the wire shape: inner content gets
`contextInfo.statusAudienceMetadata`, while the **outer** `groupStatusMessageV2.message`
keeps `messageContextInfo.messageSecret` (32 bytes) — same as the existing `swgc` path.

### Visibility note (verified by real-case test — earlier assumption was WRONG)
`audienceType: CLOSE_FRIENDS` DOES gate delivery. Confirmed in testing: when the
sender's close-friends list is empty, **only the sender** sees the SWGC-CF message —
other group members do NOT see it. So the close-friends star is NOT free: to get
BOTH the star/badge AND all-member visibility you must populate the account's
close-friends list with the group members (see next section). A bare `CLOSE_FRIENDS`
audience with an empty list = sender-only. (If you want all-member visibility with
NO star, drop the audience metadata entirely and use the plain `swgc` path.)

## How the project implements it (pattern to reuse)
The feature is a new `msg_type: "swgc_cf"`. It does NOT duplicate the SWGC
builder — it reuses the existing `buildCustomWAMessage(client, msg, "swgc", ...)`
(which already does the green styling + `GroupStatusMessageV2` wrap + `MessageSecret`),
then post-processes the result:

1. `waMsg, _ := buildCustomWAMessage(client, msg, "swgc", nil, fileBytes, mime)`
2. Reach the inner content: `inner := waMsg.GroupStatusMessageV2.Message`.
3. Optional custom background override (text only): if a hex bg is given, set
   `inner.ExtendedTextMessage.BackgroundArgb = proto.Uint32(hexToARGB(bg))` +
   `TextArgb = 0xFFFFFFFF` + `Font = SYSTEM`. (`hexToARGB` converts `#RRGGBB`
   → ARGB uint32: `((0xFF<<24)|(r<<16)|(g<<8)|b)`.)
4. Inject the audience (and optional custom emoji/name) on the **inner content's**
   `ContextInfo`. `buildCloseFriendsMessage` now takes trailing `emoji, listName string`
   params and sets them on the same `StatusAudienceMetadata`:
   ```go
   audType := waE2E.ContextInfo_StatusAudienceMetadata_CLOSE_FRIENDS
   metadata := &waE2E.ContextInfo_StatusAudienceMetadata{ AudienceType: &audType }
   if emoji != ""    { metadata.ListEmoji = proto.String(emoji) }   // e.g. "🔥"/"💎"
   if listName != ""  { metadata.ListName = proto.String(listName) } // e.g. "VIP"
   ci := &waE2E.ContextInfo{ StatusAudienceMetadata: metadata }
   switch {
   case inner.ExtendedTextMessage != nil: inner.ExtendedTextMessage.ContextInfo = ci
   case inner.ImageMessage      != nil: inner.ImageMessage.ContextInfo      = ci
   case inner.VideoMessage     != nil: inner.VideoMessage.ContextInfo     = ci
   case inner.AudioMessage     != nil: inner.AudioMessage.ContextInfo     = ci
   }
   ```
   `ListEmoji`/`ListName` are **per-message display hints**: the close-friends star
   always renders; the custom emoji/name appear next to it when set. (They are NOT
   the global list membership — that is the separate `status_privacy` appstate patch.)
5. Return `waMsg` unchanged at the outer level (the `MessageSecret` is already set
   by the `swgc` builder).

### Wiring (additive, no behavior change to existing types)
- `BroadcastCustomMessage` / `SendCustomMessageToGroup`: extend signature with
  trailing `background, emoji, listName string`, and branch
  `if msgType == "swgc_cf" { buildCloseFriendsMessage(..., background, emoji, listName) } else { buildCustomWAMessage(...) }`.
  Existing `swgc`/other branches are untouched.
- `handlers.go` `/broadcast/custom` & `/broadcast/group`: read
  `c.PostForm("background")`, `c.PostForm("cf_emoji")`, `c.PostForm("cf_name")`
  and pass them through. (These handlers are already msg_type-agnostic — no
  whitelist to update.)
- `web/public/index.html`: add a `msgType` radio `value="swgc_cf"`; inside the
  `cfBgContainer` add a color `<input type="color" id="cfBgColor">` plus two text
  inputs `id="cfEmoji"` (default "🔥", maxlength 4) and `id="cfName"` (maxlength 24).
  In `sendBroadcast()`, when `type === 'swgc_cf'`, append `background`,
  `cf_emoji`, `cf_name` to the FormData.

## Auto-populating the close-friends list (the "auto-isikan CF list" approach)
To make the star visible to everyone, the chosen approach is to programmatically add all
target group members into the bot account's close-friends list BEFORE sending.

### whatsmeow has no builder for this — it is undocumented/experimental
- `cli.GetStatusPrivacy(ctx)` exists (read-only, IQ `status`/`privacy`) but there is
  **no public setter**. whatsmeow also has **no decode handler** for the `status_privacy`
  appstate mutation, so the format below is reverse-engineered.
- The close-friends list is stored as an appstate mutation, NOT via the IQ `privacy`
  list (that IQ only covers contacts/whitelist/blacklist default audience).
- Index constant: `appstate.IndexStatusPrivacy == "status_privacy"`, which lives in the
  `appstate.WAPatchRegularHigh` ("regular_high") collection.
- `StatusPrivacyAction` (proto `waSyncAction`): `{ Mode *StatusPrivacyAction_StatusDistributionMode, UserJID []string }`.
  `Mode` values: `ALLOW_LIST=0, DENY_LIST=1, CONTACTS=2, CLOSE_FRIENDS=3`.

### Patch format used (VERSION 2 is a guess — same value as mute/star mutations)
```go
mode := waSyncAction.StatusPrivacyAction_CLOSE_FRIENDS
patch := appstate.PatchInfo{
    Type: appstate.WAPatchRegularHigh,
    Mutations: []appstate.MutationInfo{{
        Index:   []string{appstate.IndexStatusPrivacy},
        Version: 2,
        Value: &waSyncAction.SyncActionValue{
            StatusPrivacy: &waSyncAction.StatusPrivacyAction{
                Mode:    &mode,
                UserJID: []string{ /* "123456789@s.whatsapp.net", ... */ },
            },
        },
    }},
}
err := client.SendAppState(context.Background(), patch)
```
Push via the exported `(*whatsmeow.Client).SendAppState(ctx, patch)`. It handles the
`w:sync:app:state` IQ, hash/MAC, and 409 conflict-retry internally.

### Global account setting — treat as risky
- Setting mode=CLOSE_FRIENDS makes the account's status audience **default to
  close-friends-only** for the whole account, not just this group. Only the listed
  JIDs (plus the account itself) receive any status / SWGC-CF with that audience.
- There is no whatsmeow getter that reads back the CF list we set (`GetStatusPrivacy`
  returns the IQ `privacy` lists — a different mechanism), so after setting you cannot
  easily "view" it from code. Verify visually on a real device.
- If the server rejects the patch (wrong index/version), `SendAppState` returns an
  error (logged, non-fatal here) and the star will not appear. It should not corrupt
  appstate, but test on a throwaway account first.

### Project implementation (`internal/whatsapp/closefriends.go`)
- `SetCloseFriendsList(accountID, []types.JID)` — pushes the patch above with the
  given JIDs (deduped, `String()` form).
- `CollectGroupMembers(client, []types.JID)` — unions participants across groups;
  prefers `GroupParticipant.PhoneNumber` over `JID` (use PN `@s.whatsapp.net`, not LID),
  via `client.GetGroupInfo(ctx, gJID)`.
- `SyncCloseFriendsForGroups(accountID, []types.JID)` — collect + set.
- `SyncCloseFriendsFromActiveGroups(accountID)` — collect from all `is_custom_active`
  groups (DB) then set. Manual pre-fill / re-sync entry point.
- `ResetCloseFriendsList(accountID)` — push `Mode: CONTACTS, UserJID: []` to roll back.

### Wiring (additive)
- `BroadcastCustomMessage` / `SendCustomMessageToGroup`: when `msgType == "swgc_cf"`,
  BEFORE building/sending, call `SyncCloseFriendsForGroups(accountID, targetGroupJIDs)`
  so the CF list is fresh. Failure is logged, not fatal (message still sends, but the
  star may not show).
- API: `POST /api/wa/close-friends/sync?account_id=N` → `SyncCloseFriendsFromActiveGroups`
  (pre-fill / re-sync). `POST /api/wa/close-friends/reset?account_id=N` → `ResetCloseFriendsList`.

### How to test on a real account
1. `POST /api/wa/close-friends/sync?account_id=N` — watch server logs for errors.
2. Send a `swgc_cf` broadcast.
3. On a member's phone, confirm the star shows AND the message is visible.
4. If wrong: `POST /api/wa/close-friends/reset?account_id=N`, then revisit the
   `Version`/index guess above (the patch format is reverse-engineered).

## Why this matters / when to apply
Use this skill whenever adding audience variants to SWGC (e.g. a future `CUSTOM_LIST`
audience needs `ListName`/`ListEmoji` on the same `StatusAudienceMetadata`), or when
someone assumes `statusSourceType` controls visibility — it does not. The close-friends
look is a side-effect of the audience setting, not a separate style flag. Critically:
a `CLOSE_FRIENDS` audience alone does NOT make a message visible to all members —
the account's close-friends list must be populated for that, and the only whatsmeow
path is the undocumented `SendAppState` patch documented above.
