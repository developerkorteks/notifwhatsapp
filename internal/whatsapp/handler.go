package whatsapp

import (
	"context"
	"juraganxl-notif/internal/db"
	"juraganxl-notif/internal/models"
	"log"
	"regexp"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

var OnRequestStock func() string

func eventHandler(accountID uint, evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:

		msg := v.Message.GetConversation()
		if msg == "" {
			msg = v.Message.GetExtendedTextMessage().GetText()
		}

		msgLower := strings.ToLower(strings.TrimSpace(msg))
		log.Printf("[EVENT] Incoming message from %s: '%s'", v.Info.Chat.String(), msgLower)
		if msgLower == "/stok" || msgLower == "/xda" || msgLower == "/xclp" {
			log.Printf("[Account %d] Handling command '%s'", accountID, msgLower)
			go handleStockCommand(accountID, v.Info.Chat, v.Info.ID, v.Info.Sender)
		}

		// Auto-Join: detect GroupInviteMessage (invite card)
		if groupInv := v.Message.GetGroupInviteMessage(); groupInv != nil {
			go handleAutoJoinInvite(accountID, groupInv.GetInviteCode(), groupInv.GetGroupName())
		}

		// Auto-Join: detect text links (chat.whatsapp.com/XXX)
		if codes := extractInviteCodes(msg); len(codes) > 0 {
			for _, code := range codes {
				go handleAutoJoinInvite(accountID, code, "")
			}
		}

		// Anti-SWGC Check (Delete GroupStatusMessageV2 / SWGC)
		isSwgc := v.Message.GetGroupStatusMessageV2() != nil

		// DEBUG logging for group messages
		if v.Info.IsGroup {
			log.Printf("[DEBUG-GROUP-MSG] Sender: %s, FromMe: %t, IsSwgc: %t, Type: %T",
				v.Info.Sender.String(), v.Info.IsFromMe, isSwgc, v.Message)
			if v.Message.GetExtendedTextMessage() != nil {
				log.Printf("[DEBUG-EXT] ContextInfo: %+v", v.Message.GetExtendedTextMessage().GetContextInfo())
			}
			if v.Message.GetImageMessage() != nil {
				log.Printf("[DEBUG-IMG] ContextInfo: %+v", v.Message.GetImageMessage().GetContextInfo())
			}
			if v.Message.GetVideoMessage() != nil {
				log.Printf("[DEBUG-VID] ContextInfo: %+v", v.Message.GetVideoMessage().GetContextInfo())
			}
		}

		if v.Info.IsGroup && isSwgc {
			// Do not delete messages from ourselves (our own bot's SWGC)
			if !v.Info.IsFromMe {
				go handleAntiSwgc(accountID, v.Info.Chat, v.Info.Sender, v.Info.ID)
			}
		}
	}
}

func handleAntiSwgc(accountID uint, chatJID types.JID, senderJID types.JID, msgID string) {
	client, ok := Clients[accountID]
	if !ok || client == nil {
		return
	}

	var g models.GroupTarget
	if err := db.DB.First(&g, "account_id = ? AND jid = ?", accountID, chatJID.String()).Error; err != nil {
		return // Group not found or not configured
	}

	if g.IsAntiSwgcActive {
		log.Printf("[Account %d] Detected SWGC from %s in group %s. Revoking message %s", accountID, senderJID.String(), chatJID.String(), msgID)

		// Revoke the message for everyone
		client.SendMessage(context.Background(), chatJID, client.BuildRevoke(chatJID, senderJID, msgID))
	}
}

func AutoReaction(accountID uint, jid types.JID, msgID types.MessageID, sender types.JID, emoji string) {
	client, ok := Clients[accountID]
	if !ok || client == nil {
		return
	}
	reaction := &waE2E.ReactionMessage{
		Key: &waCommon.MessageKey{
			RemoteJID:   proto.String(jid.String()),
			ID:          proto.String(msgID),
			FromMe:      proto.Bool(false),
			Participant: proto.String(sender.String()),
		},
		Text:              proto.String(emoji),
		SenderTimestampMS: proto.Int64(time.Now().UnixMilli()),
	}
	msg := &waE2E.Message{
		ReactionMessage: reaction,
	}
	client.SendMessage(context.Background(), jid, msg)
}

func handleStockCommand(accountID uint, chatJID types.JID, msgID types.MessageID, senderJID types.JID) {
	client, ok := Clients[accountID]
	if !ok || client == nil {
		return
	}

	// 1. Send Reaction "⏳"
	AutoReaction(accountID, chatJID, msgID, senderJID, "⏳")

	// 2. Typing indicator
	client.SendChatPresence(context.Background(), chatJID, types.ChatPresenceComposing, types.ChatPresenceMediaText)
	time.Sleep(2 * time.Second)

	// 3. Unset typing and send message
	client.SendChatPresence(context.Background(), chatJID, types.ChatPresencePaused, types.ChatPresenceMediaText)

	var reply string
	if OnRequestStock != nil {
		reply = OnRequestStock()
	}

	if reply == "" {
		reply = "Opps! Stok kosong atau gagal mengambil data."
	}

	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String(reply),
		},
	}

	client.SendMessage(context.Background(), chatJID, msg)
}

var inviteLinkRegex = regexp.MustCompile(`https?://chat\.whatsapp\.com/([A-Za-z0-9]{10,})`)

func extractInviteCodes(text string) []string {
	matches := inviteLinkRegex.FindAllStringSubmatch(text, -1)
	var codes []string
	for _, m := range matches {
		if len(m) >= 2 {
			codes = append(codes, m[1])
		}
	}
	return codes
}

func isAutoJoinEnabled(accountID uint) bool {
	var conf models.AppConfig
	if err := db.DB.First(&conf, "account_id = ? AND key = ?", accountID, "auto_join_enabled").Error; err != nil {
		return false
	}
	return conf.Value == "true"
}

func handleAutoJoinInvite(accountID uint, inviteCode string, groupName string) {
	if inviteCode == "" {
		return
	}

	if !isAutoJoinEnabled(accountID) {
		return
	}

	client, ok := Clients[accountID]
	if !ok || client == nil || !client.IsConnected() {
		return
	}

	log.Printf("[Account %d] Auto-Join: attempting to join group via invite code %s", accountID, inviteCode)

	joinedJID, err := client.JoinGroupWithLink(context.Background(), inviteCode)
	if err != nil {
		log.Printf("[Account %d] Auto-Join: failed to join group: %v", accountID, err)
		return
	}

	log.Printf("[Account %d] Auto-Join: successfully joined group %s", accountID, joinedJID.String())

	if groupName == "" {
		groupInfo, err := client.GetGroupInfo(context.Background(), joinedJID)
		if err == nil {
			groupName = groupInfo.Name
		}
	}

	InsertJoinedGroup(accountID, joinedJID, groupName)
}
