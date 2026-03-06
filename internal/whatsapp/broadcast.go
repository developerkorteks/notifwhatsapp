package whatsapp

import (
	"context"
	"crypto/rand"
	"errors"
	"juraganxl-notif/internal/db"
	"juraganxl-notif/internal/models"
	"juraganxl-notif/internal/utils"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// BroadcastCustomMessage sends msg to Active Channel, then forwards it (sends to) all active Custom Groups
func BroadcastCustomMessage(msg string, msgType string, pollOptions []string, fileBytes []byte, mime string) error {
	if WAClient == nil || !WAClient.IsConnected() {
		return errors.New("WhatsApp client is not connected")
	}

	var activeChannel models.ChannelTarget
	if err := db.DB.First(&activeChannel, "is_active = ?", true).Error; err != nil {
		return errors.New("No active channel selected")
	}

	chJID, err := ParseJID(activeChannel.JID)
	if err != nil {
		return err
	}

	var waMsg *waE2E.Message

	// Fallback Text-to-Image for View Once that lacks a file attachment
	if msgType == "view_once" && len(fileBytes) == 0 {
		imgBytes, err := utils.CreateTextImage(msg)
		if err == nil {
			fileBytes = imgBytes
			mime = "image/png"
		}
	}

	// Optional Media Upload
	if len(fileBytes) > 0 && mime != "" {
		var mediaType whatsmeow.MediaType
		var isViewOnce = proto.Bool(msgType == "view_once")

		if strings.HasPrefix(mime, "image") {
			mediaType = whatsmeow.MediaImage
			resp, err := WAClient.Upload(context.Background(), fileBytes, mediaType)
			if err != nil {
				return err
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
			resp, err := WAClient.Upload(context.Background(), fileBytes, mediaType)
			if err != nil {
				return err
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
			resp, err := WAClient.Upload(context.Background(), fileBytes, mediaType)
			if err != nil {
				return err
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
			waMsg = WAClient.BuildPollCreation(msg, pollOptions, 1)
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

	// 1. Send to Channel ONLY for standard text (Channels strip polls and view once)
	if msgType == "standard" {
		resp, err := WAClient.SendMessage(context.Background(), chJID, waMsg)
		if err != nil {
			return err
		}

		// 2. Attach ContextInfo to make it Forwarded from Channel
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
	}

	// 3. Fetch Target Custom Groups
	var groups []models.GroupTarget
	db.DB.Where("is_custom_active = ?", true).Find(&groups)

	// 4. Loop and send to Groups
	for _, g := range groups {
		gJID, err := ParseJID(g.JID)
		if err == nil {
			WAClient.SendChatPresence(context.Background(), gJID, types.ChatPresenceComposing, types.ChatPresenceMediaText)
			time.Sleep(1 * time.Second)
			WAClient.SendChatPresence(context.Background(), gJID, types.ChatPresencePaused, types.ChatPresenceMediaText)
			WAClient.SendMessage(context.Background(), gJID, waMsg)
		}
	}

	return nil
}

// BroadcastStockMessage sends the periodic diff to Active Channel and active Stock Groups
func BroadcastStockMessage(msg string) error {
	if WAClient == nil || !WAClient.IsConnected() {
		return errors.New("WhatsApp client is not connected")
	}

	var activeChannel models.ChannelTarget
	if err := db.DB.First(&activeChannel, "is_active = ?", true).Error; err != nil {
		return errors.New("No active channel selected")
	}

	chJID, err := ParseJID(activeChannel.JID)
	if err != nil {
		return err
	}

	// Create Text Message
	waMsg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String(msg),
		},
	}

	// 1. Send to Channel
	resp, err := WAClient.SendMessage(context.Background(), chJID, waMsg)
	if err != nil {
		return err
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
	db.DB.Where("is_stock_active = ?", true).Find(&groups)

	// 4. Loop and send to Groups
	for _, g := range groups {
		gJID, err := ParseJID(g.JID)
		if err == nil {
			WAClient.SendChatPresence(context.Background(), gJID, types.ChatPresenceComposing, types.ChatPresenceMediaText)
			time.Sleep(1 * time.Second)
			WAClient.SendChatPresence(context.Background(), gJID, types.ChatPresencePaused, types.ChatPresenceMediaText)
			WAClient.SendMessage(context.Background(), gJID, waMsg)
		}
	}

	return nil
}
