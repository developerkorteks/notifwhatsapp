# API Documentation - WhatsApp Notification Broadcast System

**Base URL:** `https://notif.humanmade.my.id`

## Overview

API ini menyediakan sistem broadcast notifikasi WhatsApp dengan berbagai fitur:
- Standard Text Message
- View Once (Secret Media) - Media yang hilang setelah dibuka
- SWGC (Status WhatsApp to Group Chat) - Pesan dengan style story/status WhatsApp
- Interactive Poll - Polling dengan multiple pilihan
- Upload Media - Support image, video, audio
- Broadcast ke semua grup aktif atau ke satu grup tertentu secara dinamis
- Auto-prune data grup/channel saat sync agar tidak memakai target stale dari akun WhatsApp lama

---

## Table of Contents

1. [Authentication & Setup](#authentication--setup)
2. [Account Management](#account-management)
3. [WhatsApp Connection](#whatsapp-connection)
4. [Group Management](#group-management)
5. [Channel Management](#channel-management)
6. [Broadcast Messages](#broadcast-messages)
    - [Standard Text](#1-standard-text)
    - [View Once (Secret Media)](#2-view-once-secret-media)
    - [SWGC (Status to Group)](#3-swgc-status-whatsapp-to-group-chat)
    - [Interactive Poll](#4-interactive-poll)
    - [Upload Media](#5-upload-media)
7. [Single Group Broadcast](#single-group-broadcast)
8. [Code Examples](#code-examples)

---

## Authentication & Setup

API ini tidak menggunakan authentication token. Setiap request memerlukan `account_id` untuk identifikasi akun WhatsApp yang digunakan.

### Flow Setup Awal:

1. **Buat Account** → POST `/api/accounts`
2. **Generate QR Code** → GET `/api/wa/qr?account_id=X`
3. **Scan QR dengan WhatsApp**
4. **Cek Status** → GET `/api/wa/status?account_id=X`
5. **Sync Groups** → POST `/api/wa/groups/sync?account_id=X`
6. **Sync Channels** → POST `/api/wa/channels/sync?account_id=X`
7. **Aktifkan Target** → POST `/api/wa/groups/settings` atau `/api/wa/channels/active`
8. **Kirim Broadcast Semua Grup Aktif** → POST `/api/broadcast/custom`
9. **Kirim ke Satu Grup** → POST `/api/broadcast/group`

---

## Account Management

### GET /api/accounts

Mengambil daftar semua akun WhatsApp yang terdaftar.

**Request:**
```bash
curl https://notif.humanmade.my.id/api/accounts
```

**Response:**
```json
[
  {
    "ID": 1,
    "SessionName": "my_wa_session",
    "IsConnected": true
  },
  {
    "ID": 2,
    "SessionName": "backup_session",
    "IsConnected": false
  }
]
```

---

### POST /api/accounts

Membuat akun WhatsApp baru.

**Request:**
```bash
curl -X POST https://notif.humanmade.my.id/api/accounts \
  -H "Content-Type: application/json" \
  -d '{
    "session_name": "my_wa_session"
  }'
```

**Response:**
```json
{
  "ID": 1,
  "SessionName": "my_wa_session",
  "IsConnected": false
}
```

---

### DELETE /api/accounts/:id

Menghapus akun WhatsApp beserta semua data terkait (groups, channels).

**Request:**
```bash
curl -X DELETE https://notif.humanmade.my.id/api/accounts/1
```

**Response:**
```json
{
  "message": "Account deleted"
}
```

---

## WhatsApp Connection

### GET /api/wa/status

Mengecek status koneksi WhatsApp.

**Request:**
```bash
curl "https://notif.humanmade.my.id/api/wa/status?account_id=1"
```

**Response:**
```json
{
  "status": "connected"
}
```
atau
```json
{
  "status": "disconnected"
}
```

---

### GET /api/wa/qr

Generate QR code untuk login WhatsApp.

**Request:**
```bash
curl "https://notif.humanmade.my.id/api/wa/qr?account_id=1"
```

**Response:**
```json
{
  "qr_code": "2@xxxxxxxxxxxxxxxxxxxxxxxxxxx"
}
```

**Catatan:** QR code ini harus di-scan menggunakan WhatsApp di smartphone.

**Catatan session baru:** Saat QR baru dibuat untuk `account_id` yang sama, aplikasi akan menghapus session DB lama dan membersihkan daftar grup/channel lama untuk akun tersebut. Ini mencegah dashboard memakai target dari WhatsApp identity sebelumnya.

---

### POST /api/wa/logout

Logout dari WhatsApp.

**Request:**
```bash
curl -X POST "https://notif.humanmade.my.id/api/wa/logout?account_id=1"
```

**Response:**
```json
{
  "message": "Logged out successfully"
}
```

---

## Group Management

### GET /api/wa/groups

Mengambil daftar grup WhatsApp yang tersinkronisasi.

**Request:**
```bash
curl "https://notif.humanmade.my.id/api/wa/groups?account_id=1"
```

**Response:**
```json
[
  {
    "AccountID": 1,
    "JID": "120363123456789@g.us",
    "GroupName": "Grup Promo",
    "IsStockActive": false,
    "IsCustomActive": true,
    "IsAntiSwgcActive": false
  },
  {
    "AccountID": 1,
    "JID": "120363987654321@g.us",
    "GroupName": "Grup Info",
    "IsStockActive": true,
    "IsCustomActive": false,
    "IsAntiSwgcActive": false
  }
]
```

---

### POST /api/wa/groups/sync

Sinkronisasi grup dari WhatsApp ke database.

**Request:**
```bash
curl -X POST "https://notif.humanmade.my.id/api/wa/groups/sync?account_id=1"
```

**Response:**
```json
{
  "message": "Groups synced"
}
```

**Perilaku terbaru:** Sync akan melakukan upsert grup yang masih ada dan menghapus grup stale yang tidak lagi ditemukan di WhatsApp account yang sedang connected.

---

### GET /api/wa/groups/stats

Mengambil jumlah grup total dan jumlah grup yang aktif untuk custom broadcast.

**Request:**
```bash
curl "https://notif.humanmade.my.id/api/wa/groups/stats?account_id=1"
```

**Response:**
```json
{
  "custom_active": 12,
  "total": 30
}
```

---

### POST /api/wa/groups/toggle-all-custom

Mengaktifkan atau menonaktifkan flag custom broadcast untuk semua grup pada satu akun.

**Request:**
```bash
curl -X POST https://notif.humanmade.my.id/api/wa/groups/toggle-all-custom \
  -H "Content-Type: application/json" \
  -d '{
    "account_id": 1,
    "enabled": true
  }'
```

**Response:**
```json
{
  "message": "All groups custom updated",
  "enabled": true
}
```

---

### POST /api/wa/groups/settings

Update pengaturan grup (aktifkan/nonaktifkan untuk broadcast).

**Request:**
```bash
curl -X POST https://notif.humanmade.my.id/api/wa/groups/settings \
  -H "Content-Type: application/json" \
  -d '{
    "account_id": 1,
    "jid": "120363123456789@g.us",
    "is_stock_active": false,
    "is_custom_active": true,
    "is_anti_swgc_active": false
  }'
```

**Response:**
```json
{
  "message": "Settings updated"
}
```

**Parameter Explanation:**
- `is_stock_active`: Aktifkan untuk menerima broadcast stock otomatis
- `is_custom_active`: Aktifkan untuk menerima broadcast custom manual
- `is_anti_swgc_active`: Aktifkan untuk memblokir pesan SWGC

---

## Channel Management

### GET /api/wa/channels

Mengambil daftar channel WhatsApp yang tersinkronisasi.

**Request:**
```bash
curl "https://notif.humanmade.my.id/api/wa/channels?account_id=1"
```

**Response:**
```json
[
  {
    "AccountID": 1,
    "JID": "120363123456789@newsletter",
    "ChannelName": "Channel Promo",
    "IsActive": true
  },
  {
    "AccountID": 1,
    "JID": "120363987654321@newsletter",
    "ChannelName": "Channel Info",
    "IsActive": false
  }
]
```

---

### POST /api/wa/channels/sync

Sinkronisasi channel dari WhatsApp ke database.

**Request:**
```bash
curl -X POST "https://notif.humanmade.my.id/api/wa/channels/sync?account_id=1"
```

**Response:**
```json
{
  "message": "Channels synced"
}
```

**Perilaku terbaru:** Sync akan melakukan upsert channel yang masih ada dan menghapus channel stale yang tidak lagi ditemukan di WhatsApp account yang sedang connected.

---

### POST /api/wa/channels/active

Set channel aktif untuk broadcast (hanya 1 channel yang bisa aktif per akun).

**Request:**
```bash
curl -X POST https://notif.humanmade.my.id/api/wa/channels/active \
  -H "Content-Type: application/json" \
  -d '{
    "account_id": 1,
    "jid": "120363123456789@newsletter"
  }'
```

**Response:**
```json
{
  "message": "Active channel updated"
}
```

**Validasi:** Channel harus sudah ada di daftar channel akun tersebut. Jika `jid` tidak ditemukan, API mengembalikan error dan channel aktif lama tidak akan dikosongkan.

**Error:**
```json
{
  "error": "Channel not found for this account"
}
```

---

## Broadcast Messages

Broadcast ke semua grup custom-active menggunakan endpoint: **POST /api/broadcast/custom**

Broadcast ke satu grup tertentu menggunakan endpoint: **POST /api/broadcast/group**. Detail khusus single group ada di bagian [Single Group Broadcast](#single-group-broadcast).

Format request: `multipart/form-data` (untuk support upload file)

### Parameter Umum:

| Parameter | Type | Required | Deskripsi |
|-----------|------|----------|-----------|
| `account_id` | string | ✅ | ID akun WhatsApp |
| `jid` | string | ⚠️ | Wajib hanya untuk `/api/broadcast/group`, JID grup target |
| `message` | string | ⚠️ | Teks pesan (opsional untuk media tanpa caption) |
| `msg_type` | string | ✅ | Tipe pesan: `standard`, `view_once`, `swgc`, `poll` |
| `poll_options` | string | ⚠️ | Wajib jika `msg_type=poll`, format: `opt1\|\|opt2\|\|opt3` |
| `media` | file | ⚠️ | File upload (image/video/audio), max 10MB |

---

### 1. Standard Text

Mengirim pesan teks biasa ke channel (jika `msg_type=standard`) lalu forward ke semua grup yang `is_custom_active=true`.

**Request:**
```bash
curl -X POST https://notif.humanmade.my.id/api/broadcast/custom \
  -F "account_id=1" \
  -F "message=Halo semua! Ini adalah pesan broadcast standard." \
  -F "msg_type=standard"
```

**Response:**
```json
{
  "message": "Broadcast sent"
}
```

**Karakteristik:**
- Dikirim ke channel terlebih dahulu
- Kemudian di-forward ke semua grup aktif
- Pesan akan menampilkan badge "Forwarded from [Channel Name]"
- Support semua jenis media

**Catatan:** Untuk `/api/broadcast/custom`, standard text membutuhkan active channel. Jika active channel tidak ada atau WhatsApp menolak kirim ke channel, request dapat gagal sebelum dikirim ke grup. Untuk `/api/broadcast/group`, channel attribution bersifat best-effort: jika channel gagal, pesan tetap dikirim langsung ke grup target.

---

### 2. View Once (Secret Media)

Mengirim media yang hanya bisa dilihat sekali (hilang setelah dibuka). Cocok untuk konten rahasia atau terbatas.

**Request dengan Media:**
```bash
curl -X POST https://notif.humanmade.my.id/api/broadcast/custom \
  -F "account_id=1" \
  -F "message=Ini adalah pesan rahasia" \
  -F "msg_type=view_once" \
  -F "media=@/path/to/secret_image.jpg"
```

**Request Text Only (akan dikonversi ke image):**
```bash
curl -X POST https://notif.humanmade.my.id/api/broadcast/custom \
  -F "account_id=1" \
  -F "message=Kode rahasia: ABC123XYZ" \
  -F "msg_type=view_once"
```

**Response:**
```json
{
  "message": "Broadcast sent"
}
```

**Karakteristik:**
- Media hilang setelah dibuka oleh penerima
- Jika tidak ada media, text akan otomatis dikonversi menjadi image
- **TIDAK dikirim ke channel**, langsung ke grup
- Support image dan video
- Cocok untuk: kode promo, password, informasi sensitif

**Fitur Khusus - Flaming Text:**

Jika message dimulai dengan `flaming` diikuti style dan `|`, akan membuat image dengan efek api:

```bash
curl -X POST https://notif.humanmade.my.id/api/broadcast/custom \
  -F "account_id=1" \
  -F "message=flaming_red|FLASH SALE 50% OFF!" \
  -F "msg_type=view_once"
```

Style yang tersedia: `_red`, `_blue`, `_green`, `_purple`, `_orange`

---

### 3. SWGC (Status WhatsApp to Group Chat)

Mengirim pesan dengan tampilan **story/status WhatsApp** ke grup. Pesan akan muncul dengan background hijau WhatsApp dan font system, seperti status yang dikirim ke grup.

**SWGC Text Only:**
```bash
curl -X POST https://notif.humanmade.my.id/api/broadcast/custom \
  -F "account_id=1" \
  -F "message=🔥 FLASH SALE! Diskon 50% hari ini saja! Buruan order!" \
  -F "msg_type=swgc"
```

**SWGC dengan Image:**
```bash
curl -X POST https://notif.humanmade.my.id/api/broadcast/custom \
  -F "account_id=1" \
  -F "message=Produk terbaru! Stok terbatas!" \
  -F "msg_type=swgc" \
  -F "media=@/path/to/product_image.jpg"
```

**SWGC dengan Video:**
```bash
curl -X POST https://notif.humanmade.my.id/api/broadcast/custom \
  -F "account_id=1" \
  -F "message=Tutorial lengkap cara order!" \
  -F "msg_type=swgc" \
  -F "media=@/path/to/tutorial_video.mp4"
```

**Response:**
```json
{
  "message": "Broadcast sent"
}
```

**Karakteristik:**
- Tampilan seperti story/status WhatsApp
- Background hijau WhatsApp (#0F8A5F)
- Text putih (#FFFFFF)
- Font system (seperti status WA)
- **TIDAK dikirim ke channel**, langsung ke grup
- Bisa muncul di story member grup
- Support text, image, dan video
- Cocok untuk: promo mendesak, pengumuman penting, viral marketing

**Cara Kerja SWGC:**
1. Pesan dibungkus dalam `GroupStatusMessageV2` dengan `MessageSecret`
2. Menggunakan `ExtendedTextMessage` dengan styling khusus
3. Dikirim langsung ke semua grup yang `is_custom_active=true`
4. Member grup bisa melihat di chat grup dan mungkin muncul di story mereka

---

### 4. Interactive Poll

Mengirim polling dengan multiple pilihan. User bisa memilih salah satu opsi.

**Request:**
```bash
curl -X POST https://notif.humanmade.my.id/api/broadcast/custom \
  -F "account_id=1" \
  -F "message=Pilih menu favorit kamu?" \
  -F "msg_type=poll" \
  -F "poll_options=Nasi Goreng||Mie Goreng||Sate||Bakso||Rendang"
```

**Response:**
```json
{
  "message": "Broadcast sent"
}
```

**Format poll_options:**
- Pisahkan setiap opsi dengan `||` (double pipe)
- Minimal 2 opsi
- Maksimal tidak dibatasi (tapi WhatsApp biasanya limit ~12 opsi)

**Contoh Lain:**

**Poll Yes/No:**
```bash
curl -X POST https://notif.humanmade.my.id/api/broadcast/custom \
  -F "account_id=1" \
  -F "message=Apakah kamu setuju dengan kebijakan baru?" \
  -F "msg_type=poll" \
  -F "poll_options=Setuju||Tidak Setuju"
```

**Poll Rating:**
```bash
curl -X POST https://notif.humanmade.my.id/api/broadcast/custom \
  -F "account_id=1" \
  -F "message=Berapa rating untuk layanan kami?" \
  -F "msg_type=poll" \
  -F "poll_options=⭐ (Buruk)||⭐⭐ (Kurang)||⭐⭐⭐ (Cukup)||⭐⭐⭐⭐ (Bagus)||⭐⭐⭐⭐⭐ (Sangat Bagus)"
```

**Karakteristik:**
- **TIDAK dikirim ke channel** (channel tidak support poll)
- Langsung dikirim ke semua grup yang `is_custom_active=true`
- User hanya bisa pilih 1 opsi (single choice)
- Cocok untuk: survey, voting, feedback, engagement

---

### 5. Upload Media

Mengirim berbagai jenis media dengan 3 mode berbeda: Standard, View Once, atau SWGC.

#### 5.1 Standard Media

Media biasa yang bisa dilihat berkali-kali.

**Image:**
```bash
curl -X POST https://notif.humanmade.my.id/api/broadcast/custom \
  -F "account_id=1" \
  -F "message=Lihat produk terbaru kami!" \
  -F "msg_type=standard" \
  -F "media=@/path/to/product.jpg"
```

**Video:**
```bash
curl -X POST https://notif.humanmade.my.id/api/broadcast/custom \
  -F "account_id=1" \
  -F "message=Video tutorial cara order" \
  -F "msg_type=standard" \
  -F "media=@/path/to/tutorial.mp4"
```

**Audio:**
```bash
curl -X POST https://notif.humanmade.my.id/api/broadcast/custom \
  -F "account_id=1" \
  -F "message=Podcast episode terbaru" \
  -F "msg_type=standard" \
  -F "media=@/path/to/podcast.mp3"
```

**Tanpa Caption:**
```bash
curl -X POST https://notif.humanmade.my.id/api/broadcast/custom \
  -F "account_id=1" \
  -F "msg_type=standard" \
  -F "media=@/path/to/image.jpg"
```

---

#### 5.2 View Once Media

Media yang hilang setelah dibuka (secret media).

**Image View Once:**
```bash
curl -X POST https://notif.humanmade.my.id/api/broadcast/custom \
  -F "account_id=1" \
  -F "message=Kode promo eksklusif!" \
  -F "msg_type=view_once" \
  -F "media=@/path/to/promo_code.jpg"
```

**Video View Once:**
```bash
curl -X POST https://notif.humanmade.my.id/api/broadcast/custom \
  -F "account_id=1" \
  -F "message=Sneak peek produk baru" \
  -F "msg_type=view_once" \
  -F "media=@/path/to/sneak_peek.mp4"
```

---

#### 5.3 SWGC Media

Media dengan tampilan story/status WhatsApp.

**Image SWGC:**
```bash
curl -X POST https://notif.humanmade.my.id/api/broadcast/custom \
  -F "account_id=1" \
  -F "message=🔥 PROMO GILA-GILAAN!" \
  -F "msg_type=swgc" \
  -F "media=@/path/to/promo_banner.jpg"
```

**Video SWGC:**
```bash
curl -X POST https://notif.humanmade.my.id/api/broadcast/custom \
  -F "account_id=1" \
  -F "message=Update terbaru!" \
  -F "msg_type=swgc" \
  -F "media=@/path/to/update_video.mp4"
```

---

### Supported Media Types:

| Media Type | MIME Types | Max Size | Standard | View Once | SWGC |
|------------|------------|----------|----------|-----------|------|
| **Image** | image/jpeg, image/png, image/gif | 10MB | ✅ | ✅ | ✅ |
| **Video** | video/mp4, video/3gpp, video/quicktime | 10MB | ✅ | ✅ | ✅ |
| **Audio** | audio/mpeg, audio/ogg, audio/aac | 10MB | ✅ | ✅ | ❌ |

**Catatan:** Document/PDF belum didukung oleh builder pesan saat ini. Gunakan image, video, atau audio.

---

## Perbandingan Tipe Broadcast

| Fitur | Standard | View Once | SWGC | Poll |
|-------|----------|-----------|------|------|
| **Dikirim ke Channel** | ✅ Ya | ❌ Tidak | ❌ Tidak | ❌ Tidak |
| **Forward Badge** | ✅ Ya | ❌ Tidak | ❌ Tidak | ❌ Tidak |
| **Tampilan** | Normal | Secret (blur) | Story style | Interactive |
| **Media Support** | All | Image, Video | Image, Video | Text only |
| **Bisa dibuka berulang** | ✅ Ya | ❌ Tidak | ✅ Ya | ✅ Ya |
| **Background khusus** | ❌ Tidak | ❌ Tidak | ✅ Hijau WA | ❌ Tidak |
| **Muncul di story** | ❌ Tidak | ❌ Tidak | ⚠️ Mungkin | ❌ Tidak |
| **Use Case** | General | Secret/Promo | Viral/Urgent | Survey/Vote |

---

## Single Group Broadcast

Endpoint **POST `/api/broadcast/group`** mengirim pesan ke satu grup tertentu yang dipilih dinamis berdasarkan `jid`.

Format request tetap `multipart/form-data`, sama seperti `/api/broadcast/custom`, tetapi wajib menambahkan parameter `jid`.

### Parameter Khusus

| Parameter | Type | Required | Deskripsi |
|-----------|------|----------|-----------|
| `account_id` | string | ✅ | ID akun WhatsApp |
| `jid` | string | ✅ | JID grup target, harus sudah tersync di `/api/wa/groups` untuk account tersebut |
| `message` | string | ⚠️ | Teks pesan, wajib jika tidak upload media |
| `msg_type` | string | ✅ | `standard`, `view_once`, `swgc`, atau `poll` |
| `poll_options` | string | ⚠️ | Wajib untuk `poll`, format `opt1||opt2` minimal 2 opsi |
| `media` | file | ⚠️ | Optional image/video/audio, max 10MB |

### Standard Text ke Satu Grup

```bash
curl -X POST https://notif.humanmade.my.id/api/broadcast/group \
  -F "account_id=1" \
  -F "jid=120363123456789@g.us" \
  -F "message=Halo grup ini saja" \
  -F "msg_type=standard"
```

**Response:**
```json
{
  "message": "Message sent to group"
}
```

### View Once ke Satu Grup

```bash
curl -X POST https://notif.humanmade.my.id/api/broadcast/group \
  -F "account_id=1" \
  -F "jid=120363123456789@g.us" \
  -F "message=Kode rahasia: ABC123" \
  -F "msg_type=view_once"
```

### SWGC ke Satu Grup

```bash
curl -X POST https://notif.humanmade.my.id/api/broadcast/group \
  -F "account_id=1" \
  -F "jid=120363123456789@g.us" \
  -F "message=🔥 Promo khusus grup ini" \
  -F "msg_type=swgc"
```

### Poll ke Satu Grup

```bash
curl -X POST https://notif.humanmade.my.id/api/broadcast/group \
  -F "account_id=1" \
  -F "jid=120363123456789@g.us" \
  -F "message=Pilih paket?" \
  -F "msg_type=poll" \
  -F "poll_options=XDA||XCLP||Provider V2"
```

### Media ke Satu Grup

```bash
curl -X POST https://notif.humanmade.my.id/api/broadcast/group \
  -F "account_id=1" \
  -F "jid=120363123456789@g.us" \
  -F "message=Media khusus grup ini" \
  -F "msg_type=standard" \
  -F "media=@/path/to/image.jpg"
```

**Karakteristik:**
- Tidak membutuhkan `is_custom_active=true` pada grup target. Syaratnya hanya grup sudah tersync untuk account tersebut.
- Support tipe pesan yang sama dengan broadcast custom: `standard`, `view_once`, `swgc`, `poll`.
- Untuk `standard`, channel attribution dicoba jika ada active channel. Jika WhatsApp menolak kirim ke channel, pesan tetap dikirim langsung ke grup.
- Jika `jid` tidak ditemukan pada account tersebut, API mengembalikan error `Target group is not synced for this account`.

---

## Error Responses

Semua endpoint akan mengembalikan error dalam format:

```json
{
  "error": "Error message here"
}
```

**Common Errors:**

| HTTP Code | Error Message | Penyebab |
|-----------|---------------|----------|
| 400 | "Failed to parse form data" | Format request salah |
| 400 | "Key: 'CreateAccountReq.SessionName' Error:Field validation..." | Field required tidak diisi |
| 400 | "Target group jid is required" | `/api/broadcast/group` dipanggil tanpa parameter `jid` |
| 400 | "Message or media is required" | Broadcast single group tanpa teks dan tanpa media |
| 400 | "Poll must have at least 2 options" | Poll dikirim dengan kurang dari 2 opsi |
| 500 | "WhatsApp client is not connected" | WhatsApp belum login/terputus |
| 500 | "No active channel selected" | Belum ada channel yang diaktifkan |
| 500 | "Channel not found for this account" | JID channel tidak ada pada akun tersebut |
| 500 | "Target group is not synced for this account" | JID grup tidak ada pada akun tersebut |
| 500 | "Failed to upload image/video" | Gagal upload media ke WhatsApp |

---

## Code Examples

### JavaScript / Node.js (Fetch API)

```javascript
const BASE_URL = 'https://notif.humanmade.my.id';

// 1. Create Account
async function createAccount(sessionName) {
  const response = await fetch(`${BASE_URL}/api/accounts`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ session_name: sessionName })
  });
  return await response.json();
}

// 2. Get QR Code
async function getQRCode(accountId) {
  const response = await fetch(`${BASE_URL}/api/wa/qr?account_id=${accountId}`);
  return await response.json();
}

// 3. Check Status
async function checkStatus(accountId) {
  const response = await fetch(`${BASE_URL}/api/wa/status?account_id=${accountId}`);
  return await response.json();
}

// 4. Sync Groups
async function syncGroups(accountId) {
  const response = await fetch(`${BASE_URL}/api/wa/groups/sync?account_id=${accountId}`, {
    method: 'POST'
  });
  return await response.json();
}

// 5. Update Group Settings
async function updateGroupSettings(accountId, jid, isCustomActive) {
  const response = await fetch(`${BASE_URL}/api/wa/groups/settings`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      account_id: accountId,
      jid: jid,
      is_stock_active: false,
      is_custom_active: isCustomActive,
      is_anti_swgc_active: false
    })
  });
  return await response.json();
}

// 6. Set Active Channel
async function setActiveChannel(accountId, jid) {
  const response = await fetch(`${BASE_URL}/api/wa/channels/active`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      account_id: accountId,
      jid: jid
    })
  });
  return await response.json();
}

// 7. Send Standard Text
async function sendStandardText(accountId, message) {
  const formData = new FormData();
  formData.append('account_id', accountId);
  formData.append('message', message);
  formData.append('msg_type', 'standard');
  
  const response = await fetch(`${BASE_URL}/api/broadcast/custom`, {
    method: 'POST',
    body: formData
  });
  return await response.json();
}

// 8. Send View Once (Secret)
async function sendViewOnce(accountId, message, file = null) {
  const formData = new FormData();
  formData.append('account_id', accountId);
  formData.append('message', message);
  formData.append('msg_type', 'view_once');
  if (file) {
    formData.append('media', file);
  }
  
  const response = await fetch(`${BASE_URL}/api/broadcast/custom`, {
    method: 'POST',
    body: formData
  });
  return await response.json();
}

// 9. Send SWGC (Status to Group)
async function sendSWGC(accountId, message, file = null) {
  const formData = new FormData();
  formData.append('account_id', accountId);
  formData.append('message', message);
  formData.append('msg_type', 'swgc');
  if (file) {
    formData.append('media', file);
  }
  
  const response = await fetch(`${BASE_URL}/api/broadcast/custom`, {
    method: 'POST',
    body: formData
  });
  return await response.json();
}

// 10. Send Poll
async function sendPoll(accountId, question, options) {
  const formData = new FormData();
  formData.append('account_id', accountId);
  formData.append('message', question);
  formData.append('msg_type', 'poll');
  formData.append('poll_options', options.join('||'));
  
  const response = await fetch(`${BASE_URL}/api/broadcast/custom`, {
    method: 'POST',
    body: formData
  });
  return await response.json();
}

// 11. Send Media (Standard/View Once/SWGC)
async function sendMedia(accountId, message, file, msgType = 'standard') {
  const formData = new FormData();
  formData.append('account_id', accountId);
  formData.append('message', message);
  formData.append('msg_type', msgType);
  formData.append('media', file);
  
  const response = await fetch(`${BASE_URL}/api/broadcast/custom`, {
    method: 'POST',
    body: formData
  });
  return await response.json();
}

// 12. Send to Single Group
async function sendToGroup(accountId, groupJid, message, msgType = 'standard', options = []) {
  const formData = new FormData();
  formData.append('account_id', accountId);
  formData.append('jid', groupJid);
  formData.append('message', message);
  formData.append('msg_type', msgType);
  if (msgType === 'poll') {
    formData.append('poll_options', options.join('||'));
  }

  const response = await fetch(`${BASE_URL}/api/broadcast/group`, {
    method: 'POST',
    body: formData
  });
  return await response.json();
}

// Example Usage:
async function main() {
  try {
    // Setup
    const account = await createAccount('my_session');
    console.log('Account created:', account);
    
    const qr = await getQRCode(account.ID);
    console.log('Scan this QR:', qr.qr_code);
    
    // Wait for user to scan...
    await new Promise(resolve => setTimeout(resolve, 10000));
    
    const status = await checkStatus(account.ID);
    console.log('Status:', status);
    
    if (status.status === 'connected') {
      // Sync groups and channels
      await syncGroups(account.ID);
      await syncChannels(account.ID);
      
      // Send broadcasts
      await sendStandardText(account.ID, 'Hello World!');
      await sendSWGC(account.ID, '🔥 FLASH SALE 50% OFF!');
      await sendPoll(account.ID, 'Pilih warna favorit?', ['Merah', 'Biru', 'Hijau']);

      // Send to one group only
      const groups = await fetch(`${BASE_URL}/api/wa/groups?account_id=${account.ID}`).then(r => r.json());
      if (groups.length > 0) {
        await sendToGroup(account.ID, groups[0].JID, 'Hello single group!');
      }
    }
  } catch (error) {
    console.error('Error:', error);
  }
}
```

---

### Python (requests)

```python
import requests
import time

BASE_URL = 'https://notif.humanmade.my.id'

class WhatsAppBroadcast:
    def __init__(self):
        self.base_url = BASE_URL
    
    # 1. Create Account
    def create_account(self, session_name):
        response = requests.post(
            f'{self.base_url}/api/accounts',
            json={'session_name': session_name}
        )
        return response.json()
    
    # 2. Get QR Code
    def get_qr_code(self, account_id):
        response = requests.get(
            f'{self.base_url}/api/wa/qr',
            params={'account_id': account_id}
        )
        return response.json()
    
    # 3. Check Status
    def check_status(self, account_id):
        response = requests.get(
            f'{self.base_url}/api/wa/status',
            params={'account_id': account_id}
        )
        return response.json()
    
    # 4. Sync Groups
    def sync_groups(self, account_id):
        response = requests.post(
            f'{self.base_url}/api/wa/groups/sync',
            params={'account_id': account_id}
        )
        return response.json()
    
    # 5. Get Groups
    def get_groups(self, account_id):
        response = requests.get(
            f'{self.base_url}/api/wa/groups',
            params={'account_id': account_id}
        )
        return response.json()
    
    # 6. Update Group Settings
    def update_group_settings(self, account_id, jid, is_custom_active=True):
        response = requests.post(
            f'{self.base_url}/api/wa/groups/settings',
            json={
                'account_id': account_id,
                'jid': jid,
                'is_stock_active': False,
                'is_custom_active': is_custom_active,
                'is_anti_swgc_active': False
            }
        )
        return response.json()
    
    # 7. Sync Channels
    def sync_channels(self, account_id):
        response = requests.post(
            f'{self.base_url}/api/wa/channels/sync',
            params={'account_id': account_id}
        )
        return response.json()
    
    # 8. Set Active Channel
    def set_active_channel(self, account_id, jid):
        response = requests.post(
            f'{self.base_url}/api/wa/channels/active',
            json={
                'account_id': account_id,
                'jid': jid
            }
        )
        return response.json()
    
    # 9. Send Standard Text
    def send_standard_text(self, account_id, message):
        data = {
            'account_id': str(account_id),
            'message': message,
            'msg_type': 'standard'
        }
        response = requests.post(
            f'{self.base_url}/api/broadcast/custom',
            data=data
        )
        return response.json()
    
    # 10. Send View Once
    def send_view_once(self, account_id, message, file_path=None):
        data = {
            'account_id': str(account_id),
            'message': message,
            'msg_type': 'view_once'
        }
        files = None
        if file_path:
            files = {'media': open(file_path, 'rb')}
        
        response = requests.post(
            f'{self.base_url}/api/broadcast/custom',
            data=data,
            files=files
        )
        
        if files:
            files['media'].close()
        
        return response.json()
    
    # 11. Send SWGC
    def send_swgc(self, account_id, message, file_path=None):
        data = {
            'account_id': str(account_id),
            'message': message,
            'msg_type': 'swgc'
        }
        files = None
        if file_path:
            files = {'media': open(file_path, 'rb')}
        
        response = requests.post(
            f'{self.base_url}/api/broadcast/custom',
            data=data,
            files=files
        )
        
        if files:
            files['media'].close()
        
        return response.json()
    
    # 12. Send Poll
    def send_poll(self, account_id, question, options):
        data = {
            'account_id': str(account_id),
            'message': question,
            'msg_type': 'poll',
            'poll_options': '||'.join(options)
        }
        response = requests.post(
            f'{self.base_url}/api/broadcast/custom',
            data=data
        )
        return response.json()
    
    # 13. Send Media
    def send_media(self, account_id, message, file_path, msg_type='standard'):
        data = {
            'account_id': str(account_id),
            'message': message,
            'msg_type': msg_type
        }
        files = {'media': open(file_path, 'rb')}
        
        response = requests.post(
            f'{self.base_url}/api/broadcast/custom',
            data=data,
            files=files
        )
        
        files['media'].close()
        return response.json()

    # 14. Send to Single Group
    def send_to_group(self, account_id, group_jid, message, msg_type='standard', options=None, file_path=None):
        data = {
            'account_id': str(account_id),
            'jid': group_jid,
            'message': message,
            'msg_type': msg_type
        }
        if msg_type == 'poll' and options:
            data['poll_options'] = '||'.join(options)

        files = None
        if file_path:
            files = {'media': open(file_path, 'rb')}

        response = requests.post(
            f'{self.base_url}/api/broadcast/group',
            data=data,
            files=files
        )

        if files:
            files['media'].close()

        return response.json()


# Example Usage:
if __name__ == '__main__':
    wa = WhatsAppBroadcast()
    
    # Setup
    account = wa.create_account('my_python_session')
    print('Account created:', account)
    
    account_id = account['ID']
    
    # Get QR
    qr = wa.get_qr_code(account_id)
    print('Scan this QR code:', qr['qr_code'])
    
    # Wait for scan
    print('Waiting 15 seconds for QR scan...')
    time.sleep(15)
    
    # Check status
    status = wa.check_status(account_id)
    print('Status:', status)
    
    if status['status'] == 'connected':
        # Sync
        wa.sync_groups(account_id)
        wa.sync_channels(account_id)
        
        # Get groups
        groups = wa.get_groups(account_id)
        print('Groups:', groups)
        
        # Activate first group
        if groups:
            wa.update_group_settings(account_id, groups[0]['JID'], True)
            print(wa.send_to_group(account_id, groups[0]['JID'], 'Hello single group from Python!'))
        
        # Send broadcasts
        print(wa.send_standard_text(account_id, 'Hello from Python!'))
        print(wa.send_swgc(account_id, '🔥 PROMO GILA-GILAAN!'))
        print(wa.send_poll(account_id, 'Pilih menu favorit?', 
                          ['Nasi Goreng', 'Mie Goreng', 'Sate']))
        
        # Send media
        # print(wa.send_media(account_id, 'Check this out!', 
        #                    '/path/to/image.jpg', 'standard'))
```

---

### PHP (cURL)

```php
<?php

class WhatsAppBroadcast {
    private $baseUrl = 'https://notif.humanmade.my.id';
    
    // 1. Create Account
    public function createAccount($sessionName) {
        $ch = curl_init($this->baseUrl . '/api/accounts');
        curl_setopt($ch, CURLOPT_POST, true);
        curl_setopt($ch, CURLOPT_POSTFIELDS, json_encode([
            'session_name' => $sessionName
        ]));
        curl_setopt($ch, CURLOPT_HTTPHEADER, ['Content-Type: application/json']);
        curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
        
        $response = curl_exec($ch);
        curl_close($ch);
        
        return json_decode($response, true);
    }
    
    // 2. Get QR Code
    public function getQRCode($accountId) {
        $ch = curl_init($this->baseUrl . '/api/wa/qr?account_id=' . $accountId);
        curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
        
        $response = curl_exec($ch);
        curl_close($ch);
        
        return json_decode($response, true);
    }
    
    // 3. Check Status
    public function checkStatus($accountId) {
        $ch = curl_init($this->baseUrl . '/api/wa/status?account_id=' . $accountId);
        curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
        
        $response = curl_exec($ch);
        curl_close($ch);
        
        return json_decode($response, true);
    }
    
    // 4. Send Standard Text
    public function sendStandardText($accountId, $message) {
        $ch = curl_init($this->baseUrl . '/api/broadcast/custom');
        curl_setopt($ch, CURLOPT_POST, true);
        curl_setopt($ch, CURLOPT_POSTFIELDS, [
            'account_id' => $accountId,
            'message' => $message,
            'msg_type' => 'standard'
        ]);
        curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
        
        $response = curl_exec($ch);
        curl_close($ch);
        
        return json_decode($response, true);
    }
    
    // 5. Send View Once
    public function sendViewOnce($accountId, $message, $filePath = null) {
        $ch = curl_init($this->baseUrl . '/api/broadcast/custom');
        
        $postFields = [
            'account_id' => $accountId,
            'message' => $message,
            'msg_type' => 'view_once'
        ];
        
        if ($filePath && file_exists($filePath)) {
            $postFields['media'] = new CURLFile($filePath);
        }
        
        curl_setopt($ch, CURLOPT_POST, true);
        curl_setopt($ch, CURLOPT_POSTFIELDS, $postFields);
        curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
        
        $response = curl_exec($ch);
        curl_close($ch);
        
        return json_decode($response, true);
    }
    
    // 6. Send SWGC
    public function sendSWGC($accountId, $message, $filePath = null) {
        $ch = curl_init($this->baseUrl . '/api/broadcast/custom');
        
        $postFields = [
            'account_id' => $accountId,
            'message' => $message,
            'msg_type' => 'swgc'
        ];
        
        if ($filePath && file_exists($filePath)) {
            $postFields['media'] = new CURLFile($filePath);
        }
        
        curl_setopt($ch, CURLOPT_POST, true);
        curl_setopt($ch, CURLOPT_POSTFIELDS, $postFields);
        curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
        
        $response = curl_exec($ch);
        curl_close($ch);
        
        return json_decode($response, true);
    }
    
    // 7. Send Poll
    public function sendPoll($accountId, $question, $options) {
        $ch = curl_init($this->baseUrl . '/api/broadcast/custom');
        curl_setopt($ch, CURLOPT_POST, true);
        curl_setopt($ch, CURLOPT_POSTFIELDS, [
            'account_id' => $accountId,
            'message' => $question,
            'msg_type' => 'poll',
            'poll_options' => implode('||', $options)
        ]);
        curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
        
        $response = curl_exec($ch);
        curl_close($ch);
        
        return json_decode($response, true);
    }
    
    // 8. Send Media
    public function sendMedia($accountId, $message, $filePath, $msgType = 'standard') {
        $ch = curl_init($this->baseUrl . '/api/broadcast/custom');
        
        $postFields = [
            'account_id' => $accountId,
            'message' => $message,
            'msg_type' => $msgType,
            'media' => new CURLFile($filePath)
        ];
        
        curl_setopt($ch, CURLOPT_POST, true);
        curl_setopt($ch, CURLOPT_POSTFIELDS, $postFields);
        curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
        
        $response = curl_exec($ch);
        curl_close($ch);
        
        return json_decode($response, true);
    }
}

// Example Usage:
$wa = new WhatsAppBroadcast();

// Create account
$account = $wa->createAccount('my_php_session');
echo "Account created: " . print_r($account, true) . "\n";

$accountId = $account['ID'];

// Get QR
$qr = $wa->getQRCode($accountId);
echo "QR Code: " . $qr['qr_code'] . "\n";

// Wait and check status
sleep(15);
$status = $wa->checkStatus($accountId);
echo "Status: " . $status['status'] . "\n";

if ($status['status'] === 'connected') {
    // Send broadcasts
    echo print_r($wa->sendStandardText($accountId, 'Hello from PHP!'), true);
    echo print_r($wa->sendSWGC($accountId, '🔥 FLASH SALE!'), true);
    echo print_r($wa->sendPoll($accountId, 'Pilih warna?', ['Merah', 'Biru', 'Hijau']), true);
}

?>
```

---

### cURL (Command Line)

```bash
#!/bin/bash

BASE_URL="https://notif.humanmade.my.id"

# 1. Create Account
curl -X POST $BASE_URL/api/accounts \
  -H "Content-Type: application/json" \
  -d '{"session_name": "my_session"}'

# 2. Get QR Code (replace 1 with your account_id)
curl "$BASE_URL/api/wa/qr?account_id=1"

# 3. Check Status
curl "$BASE_URL/api/wa/status?account_id=1"

# 4. Sync Groups
curl -X POST "$BASE_URL/api/wa/groups/sync?account_id=1"

# 5. Get Groups
curl "$BASE_URL/api/wa/groups?account_id=1"

# 6. Update Group Settings
curl -X POST $BASE_URL/api/wa/groups/settings \
  -H "Content-Type: application/json" \
  -d '{
    "account_id": 1,
    "jid": "120363123456789@g.us",
    "is_stock_active": false,
    "is_custom_active": true,
    "is_anti_swgc_active": false
  }'

# 7. Set Active Channel
curl -X POST $BASE_URL/api/wa/channels/active \
  -H "Content-Type: application/json" \
  -d '{
    "account_id": 1,
    "jid": "120363123456789@newsletter"
  }'

# 8. Send Standard Text
curl -X POST $BASE_URL/api/broadcast/custom \
  -F "account_id=1" \
  -F "message=Hello World!" \
  -F "msg_type=standard"

# 9. Send View Once (Text only)
curl -X POST $BASE_URL/api/broadcast/custom \
  -F "account_id=1" \
  -F "message=Kode rahasia: ABC123" \
  -F "msg_type=view_once"

# 10. Send View Once (With Image)
curl -X POST $BASE_URL/api/broadcast/custom \
  -F "account_id=1" \
  -F "message=Lihat ini sekali saja!" \
  -F "msg_type=view_once" \
  -F "media=@/path/to/secret.jpg"

# 11. Send SWGC (Text only)
curl -X POST $BASE_URL/api/broadcast/custom \
  -F "account_id=1" \
  -F "message=🔥 FLASH SALE 50% OFF!" \
  -F "msg_type=swgc"

# 12. Send SWGC (With Image)
curl -X POST $BASE_URL/api/broadcast/custom \
  -F "account_id=1" \
  -F "message=Promo terbatas!" \
  -F "msg_type=swgc" \
  -F "media=@/path/to/promo.jpg"

# 13. Send Poll
curl -X POST $BASE_URL/api/broadcast/custom \
  -F "account_id=1" \
  -F "message=Pilih menu favorit?" \
  -F "msg_type=poll" \
  -F "poll_options=Nasi Goreng||Mie Goreng||Sate||Bakso"

# 14. Send Standard Media (Image)
curl -X POST $BASE_URL/api/broadcast/custom \
  -F "account_id=1" \
  -F "message=Produk terbaru!" \
  -F "msg_type=standard" \
  -F "media=@/path/to/product.jpg"

# 15. Send Standard Media (Video)
curl -X POST $BASE_URL/api/broadcast/custom \
  -F "account_id=1" \
  -F "message=Tutorial lengkap" \
  -F "msg_type=standard" \
  -F "media=@/path/to/tutorial.mp4"

# 16. Send Standard Text to One Group Only
curl -X POST $BASE_URL/api/broadcast/group \
  -F "account_id=1" \
  -F "jid=120363123456789@g.us" \
  -F "message=Hello single group!" \
  -F "msg_type=standard"

# 17. Send Poll to One Group Only
curl -X POST $BASE_URL/api/broadcast/group \
  -F "account_id=1" \
  -F "jid=120363123456789@g.us" \
  -F "message=Pilih paket?" \
  -F "msg_type=poll" \
  -F "poll_options=XDA||XCLP"
```

---

## Best Practices

### 1. Setup Flow
```
Create Account → Get QR → Scan → Check Status → Sync Groups/Channels → 
Activate Targets → Send Broadcast
```

### 2. Error Handling
Selalu cek response dan handle error:
```javascript
const result = await sendBroadcast();
if (result.error) {
  console.error('Broadcast failed:', result.error);
  // Handle error (retry, notify admin, etc)
} else {
  console.log('Broadcast success:', result.message);
}
```

### 3. Rate Limiting
Untuk menghindari spam/ban dari WhatsApp:
- Jangan kirim broadcast terlalu sering (minimal jeda 5-10 detik)
- Batasi jumlah broadcast per hari
- Gunakan typing indicator (sudah built-in di API)

### 4. Media Optimization
- Compress image sebelum upload (max 10MB)
- Gunakan format yang didukung WhatsApp
- Video sebaiknya < 5MB untuk loading cepat

### 5. Message Type Selection
- **Standard**: Untuk pesan umum, katalog, info produk
- **View Once**: Untuk kode promo, password, info sensitif
- **SWGC**: Untuk promo urgent, viral marketing, pengumuman penting
- **Poll**: Untuk survey, voting, engagement

### 6. Group Management
- Aktifkan hanya grup yang relevan (`is_custom_active=true`)
- Gunakan `is_anti_swgc_active=true` untuk grup yang tidak mau menerima SWGC
- Sync groups secara berkala untuk update target dan otomatis membersihkan grup stale
- Gunakan `/api/broadcast/group` jika hanya ingin mengirim ke satu grup tanpa mengaktifkan `is_custom_active`

---

## Validation Notes

Dokumentasi ini diperbarui berdasarkan source code dan smoke test lokal pada 2026-06-27.

Validasi yang dijalankan:
- `go test ./...` berhasil untuk semua package.
- `GET /api/accounts` mengembalikan response 200.
- `GET /api/wa/status?account_id=5` mengembalikan response 200.
- `POST /api/broadcast/group` tanpa multipart form mengembalikan error validasi 400, membuktikan route aktif.

Smoke test tidak mengirim broadcast live ke WhatsApp group untuk menghindari pengiriman pesan tidak disengaja.

---

## FAQ

**Q: Apakah perlu authentication token?**  
A: Tidak, API ini menggunakan `account_id` untuk identifikasi.

**Q: Berapa maksimal file size untuk upload media?**  
A: 10MB (sudah di-set di code: `10 << 20`)

**Q: Apakah bisa kirim ke nomor personal (bukan grup)?**  
A: Tidak, API ini khusus untuk broadcast ke grup dan channel.

**Q: Kenapa SWGC tidak dikirim ke channel?**  
A: SWGC menggunakan `GroupStatusMessageV2` yang tidak didukung oleh channel WhatsApp.

**Q: Berapa maksimal opsi untuk poll?**  
A: Tidak ada limit di API, tapi WhatsApp biasanya limit ~12 opsi.

**Q: Apakah view once bisa di-screenshot?**  
A: Ya, WhatsApp tidak bisa mencegah screenshot. View once hanya mencegah membuka ulang.

**Q: Bagaimana cara mengetahui broadcast berhasil?**  
A: Cek response API. Jika `{"message": "Broadcast sent"}` berarti berhasil dikirim ke server WhatsApp.

**Q: Apakah bisa schedule broadcast?**  
A: Bisa untuk promo scheduler melalui endpoint `/api/promo/*`. Custom broadcast manual tetap dikirim saat endpoint dipanggil.

**Q: Apakah bisa kirim ke satu grup saja?**  
A: Bisa. Gunakan `POST /api/broadcast/group` dengan parameter `jid` grup target.

**Q: Apakah single group harus `is_custom_active=true`?**  
A: Tidak. Single group hanya mensyaratkan `jid` grup sudah tersync untuk `account_id` tersebut.

**Q: Bagaimana cara logout?**  
A: POST ke `/api/wa/logout?account_id=X`

**Q: Apakah bisa multiple account?**  
A: Ya, buat account baru dengan session name berbeda.

---

## Support

Untuk pertanyaan atau issue, silakan hubungi developer atau buat issue di repository.

**API Version:** 1.1  
**Last Updated:** 2026-06-27  
**Base URL:** https://notif.humanmade.my.id

---

## Changelog

### v1.1 (2026-06-27)
- Added `/api/broadcast/group` untuk kirim pesan ke satu grup dinamis.
- Single group broadcast support `standard`, `view_once`, `swgc`, `poll`, dan media image/video/audio.
- Single group standard message memakai channel attribution best-effort; jika channel ditolak WhatsApp, pesan tetap dikirim ke grup.
- Sync groups/channels sekarang menghapus stale target yang tidak ditemukan lagi pada WhatsApp account aktif.
- Generate QR session baru membersihkan target grup/channel lama untuk mencegah cache akun sebelumnya.
- Set active channel sekarang validasi JID channel sebelum mengganti active channel.
- Dokumentasi media dikoreksi menjadi image/video/audio sesuai source code saat ini.

### v1.0 (2026-04-27)
- Initial release
- Support Standard Text, View Once, SWGC, Poll
- Support Image, Video, Audio upload
- Multi-account support
- Group and Channel management

---

**Happy Broadcasting! 🚀**
