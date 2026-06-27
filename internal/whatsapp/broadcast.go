package whatsapp

import (
	"context"
	"crypto/rand"
	"errors"
	"juraganxl-notif/internal/db"
	"juraganxl-notif/internal/models"
	"juraganxl-notif/internal/utils"
	"log"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

func buildCustomWAMessage(client *whatsmeow.Client, msg string, msgType string, pollOptions []string, fileBytes []byte, mime string) (*waE2E.Message, error) {
	var waMsg *waE2E.Message

	// Fallback Text-to-Image for View Once/SWGC that lacks a file attachment
	if len(fileBytes) == 0 && msg != "" {
		isFlaming := strings.HasPrefix(msg, "flaming") && strings.Contains(msg, "|")

		if msgType == "view_once" || isFlaming {
			var imgBytes []byte
			var err error

			if isFlaming {
				parts := strings.SplitN(msg, "|", 2)
				style := strings.TrimPrefix(parts[0], "flaming")
				text := parts[1]
				imgBytes, err = utils.CreateFlamingImage(text, style)
			} else if msgType == "view_once" {
				imgBytes, err = utils.CreateTextImage(msg)
			}

			if err == nil && len(imgBytes) > 0 {
				fileBytes = imgBytes
				mime = "image/png"
			} else if msgType == "view_once" {
				// if flaming fails and it's view_once, fallback to text image
				fileBytes, _ = utils.CreateTextImage(msg)
				mime = "image/png"
			}
		}
	}

	// Optional Media Upload
	if len(fileBytes) > 0 && mime != "" {
		var mediaType whatsmeow.MediaType
		var isViewOnce = proto.Bool(msgType == "view_once")

		if strings.HasPrefix(mime, "image") {
			mediaType = whatsmeow.MediaImage
			resp, err := client.Upload(context.Background(), fileBytes, mediaType)
			if err != nil {
				return nil, err
			}

			waMsg = &waE2E.Message{
				ImageMessage: &waE2E.ImageMessage{
					Caption:       proto.String(msg),
					Mimetype:      proto.String(mime),
					URL:           &resp.URL,
					DirectPath:    &resp.DirectPath,
					MediaKey:      resp.MediaKey,
					FileEncSHA256: resp.FileEncSHA256,
					FileSHA256:    resp.FileSHA256,
					FileLength:    &resp.FileLength,
					ViewOnce:      isViewOnce,
				},
			}
		} else if strings.HasPrefix(mime, "video") {
			mediaType = whatsmeow.MediaVideo
			resp, err := client.Upload(context.Background(), fileBytes, mediaType)
			if err != nil {
				return nil, err
			}

			waMsg = &waE2E.Message{
				VideoMessage: &waE2E.VideoMessage{
					Caption:       proto.String(msg),
					Mimetype:      proto.String(mime),
					URL:           &resp.URL,
					DirectPath:    &resp.DirectPath,
					MediaKey:      resp.MediaKey,
					FileEncSHA256: resp.FileEncSHA256,
					FileSHA256:    resp.FileSHA256,
					FileLength:    &resp.FileLength,
					ViewOnce:      isViewOnce,
				},
			}
		} else if strings.HasPrefix(mime, "audio") {
			mediaType = whatsmeow.MediaAudio
			resp, err := client.Upload(context.Background(), fileBytes, mediaType)
			if err != nil {
				return nil, err
			}

			waMsg = &waE2E.Message{
				AudioMessage: &waE2E.AudioMessage{
					Mimetype:      proto.String(mime),
					URL:           &resp.URL,
					DirectPath:    &resp.DirectPath,
					MediaKey:      resp.MediaKey,
					FileEncSHA256: resp.FileEncSHA256,
					FileSHA256:    resp.FileSHA256,
					FileLength:    &resp.FileLength,
					ViewOnce:      isViewOnce,
				},
			}
		}
	}

	if waMsg == nil {
		if msgType == "poll" && len(pollOptions) >= 2 {
			waMsg = client.BuildPollCreation(msg, pollOptions, 1)
		} else {
			extended := &waE2E.ExtendedTextMessage{
				Text: proto.String(msg),
			}
			if msgType == "swgc" {
				fontType := waE2E.ExtendedTextMessage_SYSTEM
				extended.BackgroundArgb = proto.Uint32(0xFF0F8A5F) // WhatsApp green tint
				extended.TextArgb = proto.Uint32(0xFFFFFFFF)
				extended.Font = &fontType
			}

			waMsg = &waE2E.Message{
				ExtendedTextMessage: extended,
			}
		}
	}

	if msgType == "swgc" {
		messageSecret := make([]byte, 32)
		rand.Read(messageSecret)

		innerMsg := waMsg
		if innerMsg.MessageContextInfo == nil {
			innerMsg.MessageContextInfo = &waE2E.MessageContextInfo{}
		}
		innerMsg.MessageContextInfo.MessageSecret = messageSecret

		waMsg = &waE2E.Message{
			MessageContextInfo: &waE2E.MessageContextInfo{
				MessageSecret: messageSecret,
			},
			GroupStatusMessageV2: &waE2E.FutureProofMessage{
				Message: innerMsg,
			},
		}
	}

	return waMsg, nil
}

func attachChannelForwardContext(client *whatsmeow.Client, activeChannel models.ChannelTarget, waMsg *waE2E.Message) error {
	chJID, err := ParseJID(activeChannel.JID)
	if err != nil {
		return err
	}

	resp, err := client.SendMessage(context.Background(), chJID, waMsg)
	if err != nil {
		return err
	}

	ctxInfo := &waE2E.ContextInfo{
		IsForwarded: proto.Bool(true),
		ForwardedNewsletterMessageInfo: &waE2E.ContextInfo_ForwardedNewsletterMessageInfo{
			NewsletterJID:   proto.String(activeChannel.JID),
			NewsletterName:  proto.String(activeChannel.ChannelName),
			ServerMessageID: proto.Int32(int32(resp.ServerID)),
		},
	}

	if waMsg.ExtendedTextMessage != nil {
		waMsg.ExtendedTextMessage.ContextInfo = ctxInfo
	}

	return nil
}

// BroadcastCustomMessage sends msg to Active Channel, then forwards it (sends to) all active Custom Groups
func BroadcastCustomMessage(accountID uint, msg string, msgType string, pollOptions []string, fileBytes []byte, mime string) error {
	client, ok := Clients[accountID]
	if !ok || client == nil || !client.IsConnected() {
		return errors.New("WhatsApp client is not connected")
	}

	var activeChannel models.ChannelTarget
	if err := db.DB.First(&activeChannel, "account_id = ? AND is_active = ?", accountID, true).Error; err != nil {
		return errors.New("No active channel selected")
	}

	waMsg, err := buildCustomWAMessage(client, msg, msgType, pollOptions, fileBytes, mime)
	if err != nil {
		return err
	}

	// 1. Send to Channel ONLY for standard text (Channels strip polls and view once)
	if msgType == "standard" {
		if err := attachChannelForwardContext(client, activeChannel, waMsg); err != nil {
			return err
		}
	}

	// 2. Fetch Target Custom Groups
	var groups []models.GroupTarget
	db.DB.Where("account_id = ? AND is_custom_active = ?", accountID, true).Find(&groups)

	// 3. Loop and send to Groups
	for _, g := range groups {
		gJID, err := ParseJID(g.JID)
		if err == nil {
			client.SendChatPresence(context.Background(), gJID, types.ChatPresenceComposing, types.ChatPresenceMediaText)
			time.Sleep(1 * time.Second)
			client.SendChatPresence(context.Background(), gJID, types.ChatPresencePaused, types.ChatPresenceMediaText)
			client.SendMessage(context.Background(), gJID, waMsg)
		}
	}

	return nil
}

// SendCustomMessageToGroup sends one custom message to a single configured group.
func SendCustomMessageToGroup(accountID uint, groupJID string, msg string, msgType string, pollOptions []string, fileBytes []byte, mime string) error {
	client, ok := Clients[accountID]
	if !ok || client == nil || !client.IsConnected() {
		return errors.New("WhatsApp client is not connected")
	}

	var targetGroup models.GroupTarget
	if err := db.DB.First(&targetGroup, "account_id = ? AND jid = ?", accountID, groupJID).Error; err != nil {
		return errors.New("Target group is not synced for this account")
	}

	gJID, err := ParseJID(groupJID)
	if err != nil {
		return err
	}

	waMsg, err := buildCustomWAMessage(client, msg, msgType, pollOptions, fileBytes, mime)
	if err != nil {
		return err
	}

	// Preserve newsletter attribution for standard messages when an active channel exists.
	if msgType == "standard" {
		var activeChannel models.ChannelTarget
		if err := db.DB.First(&activeChannel, "account_id = ? AND is_active = ?", accountID, true).Error; err == nil {
			if err := attachChannelForwardContext(client, activeChannel, waMsg); err != nil {
				log.Printf("[Account %d] Failed to attach channel attribution via %s: %v. Sending directly to group %s", accountID, activeChannel.JID, err, groupJID)
			}
		}
	}

	client.SendChatPresence(context.Background(), gJID, types.ChatPresenceComposing, types.ChatPresenceMediaText)
	time.Sleep(1 * time.Second)
	client.SendChatPresence(context.Background(), gJID, types.ChatPresencePaused, types.ChatPresenceMediaText)
	_, err = client.SendMessage(context.Background(), gJID, waMsg)
	return err
}

// BroadcastStockMessage sends the periodic diff to all active Accounts -> Active Channel and active Stock Groups
func BroadcastStockMessage(msg string) error {
	for accountID, client := range Clients {
		if client == nil || !client.IsConnected() {
			continue
		}

		var activeChannel models.ChannelTarget
		if err := db.DB.First(&activeChannel, "account_id = ? AND is_active = ?", accountID, true).Error; err != nil {
			continue
		}

		chJID, err := ParseJID(activeChannel.JID)
		if err != nil {
			continue
		}

		// Create Text Message
		waMsg := &waE2E.Message{
			ExtendedTextMessage: &waE2E.ExtendedTextMessage{
				Text: proto.String(msg),
			},
		}

		// 1. Send to Channel
		resp, err := client.SendMessage(context.Background(), chJID, waMsg)
		if err != nil {
			continue
		}

		// 2. Attach ContextInfo to make it Forwarded from Channel
		waMsg.ExtendedTextMessage.ContextInfo = &waE2E.ContextInfo{
			IsForwarded: proto.Bool(true),
			ForwardedNewsletterMessageInfo: &waE2E.ContextInfo_ForwardedNewsletterMessageInfo{
				NewsletterJID:   proto.String(activeChannel.JID),
				NewsletterName:  proto.String(activeChannel.ChannelName),
				ServerMessageID: proto.Int32(int32(resp.ServerID)),
			},
		}

		// 3. Fetch Target Stock Groups
		var groups []models.GroupTarget
		db.DB.Where("account_id = ? AND is_stock_active = ?", accountID, true).Find(&groups)

		// 4. Loop and send to Groups
		for _, g := range groups {
			gJID, err := ParseJID(g.JID)
			if err == nil {
				client.SendChatPresence(context.Background(), gJID, types.ChatPresenceComposing, types.ChatPresenceMediaText)
				time.Sleep(1 * time.Second)
				client.SendChatPresence(context.Background(), gJID, types.ChatPresencePaused, types.ChatPresenceMediaText)
				client.SendMessage(context.Background(), gJID, waMsg)
			}
		}
	}

	return nil
}
