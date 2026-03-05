package whatsapp

import (
	"context"
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
