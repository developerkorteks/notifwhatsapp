---
name: whatsapp-broadcast
description: How to build and send WhatsApp broadcast messages (standard, view_once, swgc, poll, media) via whatsmeow in this project, including channel-forward attribution and gotchas
source: auto-skill
extracted_at: '2026-07-11T16:41:30.213Z'
---

# WhatsApp Broadcast Message Construction (whatsmeow)

This project (`juraganxl-notif`) is a WhatsApp multi-account broadcast engine built on
`go.mau.fi/whatsmeow`. When adding/editing broadcast features, reuse the existing
message-building logic instead of inventing your own. The canonical implementation lives in
`internal/whatsapp/broadcast.go` and `internal/promo/sender.go` (the promo sender is a near
dupe of broadcast.go — keep them in sync if you change message logic).

## Entry point is git-ignored — do NOT rely on file search
`cmd/server/main.go` exists on disk but is excluded by `.gitignore` (the `server` ignore
pattern matches the dir name). `glob **/*.go` will NOT return it. Read it directly with
`read_file` at `/mnt/data_d/project/notifwhatsapp/cmd/server/main.go` to see startup order:
`db.InitDB()` → `whatsapp.InitClient()` → `scraper.InitCron()` → `promo.StartScheduler()` → `api.StartServer()` (Gin on `:59021`).

## Core builder: buildCustomWAMessage(client, msg, msgType, pollOptions, fileBytes, mime)
Follow this precedence when extending it:

1. **Text-to-image fallback** (no file attached):
   - `view_once` with text → render to PNG via `utils.CreateTextImage(msg)`.
   - message prefixed `flaming<color>|text` (e.g. `flaming_red|FLASH SALE`) → fetch a fiery
     text PNG from external `api.cuki.biz.id` via `utils.CreateFlamingImage(text, style)`.
     Colors: `_red`, `_blue`, `_green`, `_purple`, `_orange`.
2. **Media upload** (image/video/audio) via `client.Upload(ctx, fileBytes, whatsmeow.MediaImage|Video|Audio)`.
   Set `ViewOnce = proto.Bool(msgType == "view_once")`.
3. **No media left** → fallback:
   - `poll` with >=2 `pollOptions` → `client.BuildPollCreation(msg, pollOptions, 1)` (single choice).
   - else `ExtendedTextMessage{Text: ...}`.
4. **SWGC wrapping** (only when `msgType == "swgc"`): wrap the inner message in
   `GroupStatusMessageV2` (`waE2E.FutureProofMessage`) and attach a random 32-byte
   `MessageSecret` in `MessageContextInfo`. For the green "status" look, set on the
   `ExtendedTextMessage`:
   ```go
   fontType := waE2E.ExtendedTextMessage_SYSTEM
   extended.BackgroundArgb = proto.Uint32(0xFF0F8A5F) // WhatsApp green
   extended.TextArgb     = proto.Uint32(0xFFFFFFFF)
   extended.Font         = &fontType
   ```

## Channel-forward attribution (the "Forwarded from [Channel]" badge)
Only `standard` messages get sent to the active channel first. Flow in
`attachChannelForwardContext` / `BroadcastCustomMessage`:
1. Find active channel: `db.DB.First(&activeChannel, "account_id = ? AND is_active = ?", accountID, true)`.
   If none → error `"No active channel selected"`.
2. `client.SendMessage(ctx, chJID, waMsg)` to channel.
3. Copy `resp.ServerID` into `ContextInfo{IsForwarded: true, ForwardedNewsletterMessageInfo{...}}`
   on `waMsg.ExtendedTextMessage`.
4. Then loop target groups (`is_custom_active == true`), each preceded by a 1s typing
   indicator (`SendChatPresence` Composing → sleep → Paused).

Routing rules (per API_DOCUMENTATION.md):
- `standard` → channel + groups. `view_once` / `swgc` / `poll` → groups only (channel strips them).
- `BroadcastCustomMessage` requires active channel for standard; `SendCustomMessageToGroup` is
  best-effort (falls back to direct group send if channel attach fails).

## Anti-ban / pacing conventions
- Promo sender (`promo/sender.go`) shuffles groups (`mrand.Shuffle`) and waits a **random
  5–15 min** between groups; custom broadcast uses only a 1s typing pause.
- Stock broadcast (`BroadcastStockMessage`) runs per connected account, channel + `is_stock_active` groups.

## Event-driven extras (internal/whatsapp/handler.go)
- Chat commands `/stok`, `/xda`, `/xclp` → reply with `scraper.FetchCurrentStock()`.
- Auto-join: `GroupInviteMessage` or `chat.whatsapp.com/XXX` links → join if
  `AppConfig[auto_join_enabled]=="true"` → insert `GroupTarget{IsCustomActive: true}`.
- Anti-SWGC: incoming `GroupStatusMessageV2` from others in a group with `IsAntiSwgcActive`
  is revoked via `client.BuildRevoke(...)`.

## Useful gotchas
- `flaming` and `CreateFlamingImage` depend on a third-party API (`api.cuki.biz.id`) — treat as
  best-effort; code falls back to `CreateTextImage` on failure.
- `test_pretty.go` at repo root is a standalone `package main` demo of `CreateTextImage` — it will
  conflict with `cmd/server/main.go` if both are in the same build; it is NOT part of the server
  build (separate package dir). Do not move it into `internal`.
- Session DBs live in `sessions/wa_session_acc<N>_<unix>.db`; regenerating QR deletes the old one
  and **wipes that account's GroupTarget + ChannelTarget** to avoid stale targets.
