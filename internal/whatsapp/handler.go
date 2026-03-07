package whatsapp

import (
	"context"
	"juraganxl-notif/internal/db"
	"juraganxl-notif/internal/models"
	"log"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

var OnRequestStock func() string

func eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:

		msg := v.Message.GetConversation()
		if msg == "" {
			msg = v.Message.GetExtendedTextMessage().GetText()
		}

		msgLower := strings.ToLower(strings.TrimSpace(msg))
		log.Printf("[EVENT] Incoming message from %s: '%s'", v.Info.Chat.String(), msgLower)
		if msgLower == "/stok" || msgLower == "/xda" || msgLower == "/xclp" {
			log.Printf("[EVENT] Handling command '%s'", msgLower)
			go handleStockCommand(v.Info.Chat, v.Info.ID, v.Info.Sender)
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
				go handleAntiSwgc(v.Info.Chat, v.Info.Sender, v.Info.ID)
			}
		}
	}
}

func handleAntiSwgc(chatJID types.JID, senderJID types.JID, msgID string) {
	if WAClient == nil {
		return
	}

	var g models.GroupTarget
	if err := db.DB.First(&g, "jid = ?", chatJID.String()).Error; err != nil {
		return // Group not found or not configured
	}

	if g.IsAntiSwgcActive {
		log.Printf("[ANTI-SWGC] Detected SWGC (Group Status Message) from %s in group %s. Revoking message %s", senderJID.String(), chatJID.String(), msgID)

		// Revoke the message for everyone
		WAClient.SendMessage(context.Background(), chatJID, WAClient.BuildRevoke(chatJID, senderJID, msgID))
	}
}

func AutoReaction(jid types.JID, msgID types.MessageID, sender types.JID, emoji string) {
	if WAClient == nil {
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
	WAClient.SendMessage(context.Background(), jid, msg)
}

func handleStockCommand(chatJID types.JID, msgID types.MessageID, senderJID types.JID) {
	// 1. Send Reaction "⏳"
	AutoReaction(chatJID, msgID, senderJID, "⏳")

	// 2. Typing indicator
	WAClient.SendChatPresence(context.Background(), chatJID, types.ChatPresenceComposing, types.ChatPresenceMediaText)
	time.Sleep(2 * time.Second)

	// 3. Unset typing and send message
	WAClient.SendChatPresence(context.Background(), chatJID, types.ChatPresencePaused, types.ChatPresenceMediaText)

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

	WAClient.SendMessage(context.Background(), chatJID, msg)
}
