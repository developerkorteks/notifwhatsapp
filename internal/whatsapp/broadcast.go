package whatsapp

import (
	"context"
	"crypto/rand"
	"errors"
	"strconv"
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

// buildCloseFriendsMessage builds a group status (SWGC) with the CLOSE_FRIENDS
// audience. This is what renders the "close friends" star/badge in WhatsApp.
//
// The audience restriction means only members in the sender account's
// close-friends list can see the message (the sender always sees it). A custom
// background hex (#RRGGBB) overrides the default green. emoji and listName set
// the close-friends list's custom emoji and label shown alongside the star.
func buildCloseFriendsMessage(client *whatsmeow.Client, msg string, fileBytes []byte, mime string, background string, emoji string, listName string) (*waE2E.Message, error) {
	waMsg, err := buildCustomWAMessage(client, msg, "swgc", nil, fileBytes, mime)
	if err != nil {
		return nil, err
	}
	if waMsg == nil || waMsg.GroupStatusMessageV2 == nil {
		return waMsg, nil
	}
	inner := waMsg.GroupStatusMessageV2.Message
	if inner == nil {
		return waMsg, nil
	}

	if background != "" && inner.ExtendedTextMessage != nil {
		font := waE2E.ExtendedTextMessage_SYSTEM
		inner.ExtendedTextMessage.BackgroundArgb = proto.Uint32(hexToARGB(background))
		inner.ExtendedTextMessage.TextArgb = proto.Uint32(0xFFFFFFFF)
		inner.ExtendedTextMessage.Font = &font
	}

	audType := waE2E.ContextInfo_StatusAudienceMetadata_CLOSE_FRIENDS
	metadata := &waE2E.ContextInfo_StatusAudienceMetadata{ AudienceType: &audType }
	if emoji != "" {
		metadata.ListEmoji = proto.String(emoji)
	}
	if listName != "" {
		metadata.ListName = proto.String(listName)
	}
	ci := &waE2E.ContextInfo{ StatusAudienceMetadata: metadata }
	switch {
	case inner.ExtendedTextMessage != nil:
		inner.ExtendedTextMessage.ContextInfo = ci
	case inner.ImageMessage != nil:
		inner.ImageMessage.ContextInfo = ci
	case inner.VideoMessage != nil:
		inner.VideoMessage.ContextInfo = ci
	case inner.AudioMessage != nil:
		inner.AudioMessage.ContextInfo = ci
	}

	return waMsg, nil
}

func hexToARGB(hex string) uint32 {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) == 3 {
		b := make([]byte, 6)
		for i := 0; i < 3; i++ {
			b[i*2], b[i*2+1] = hex[i], hex[i]
		}
		hex = string(b)
	}
	if len(hex) != 6 {
		return 0xFF0F8A5F
	}
	r, _ := strconv.ParseUint(hex[0:2], 16, 32)
	g, _ := strconv.ParseUint(hex[2:4], 16, 32)
	b, _ := strconv.ParseUint(hex[4:6], 16, 32)
	return uint32(0xFF)<<24 | uint32(r)<<16 | uint32(g)<<8 | uint32(b)
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
func BroadcastCustomMessage(accountID uint, msg string, msgType string, pollOptions []string, fileBytes []byte, mime string, background string, emoji string, listName string) error {
	client, ok := Clients[accountID]
	if !ok || client == nil || !client.IsConnected() {
		return errors.New("WhatsApp client is not connected")
	}

	var activeChannel models.ChannelTarget
	if err := db.DB.First(&activeChannel, "account_id = ? AND is_active = ?", accountID, true).Error; err != nil {
		return errors.New("No active channel selected")
	}

	var waMsg *waE2E.Message
	var err error
	if msgType == "swgc_cf" {
		waMsg, err = buildCloseFriendsMessage(client, msg, fileBytes, mime, background, emoji, listName)
	} else {
		waMsg, err = buildCustomWAMessage(client, msg, msgType, pollOptions, fileBytes, mime)
	}
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

	// 2b. For SWGC-CF, pre-populate the account's close-friends list with the
	// union of all target group members so the close-friends star is visible to
	// everyone. This is a global account setting.
	if msgType == "swgc_cf" {
		var gJIDs []types.JID
		for _, g := range groups {
			if j, err := ParseJID(g.JID); err == nil {
				gJIDs = append(gJIDs, j)
			}
		}
		if len(gJIDs) > 0 {
			if err := SyncCloseFriendsForGroups(accountID, gJIDs); err != nil {
				log.Printf("[Account %d] Failed to sync close-friends list for SWGC-CF: %v", accountID, err)
			}
		}
	}

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
func SendCustomMessageToGroup(accountID uint, groupJID string, msg string, msgType string, pollOptions []string, fileBytes []byte, mime string, background string, emoji string, listName string) error {
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

	// For SWGC-CF, pre-populate the account's close-friends list with the target
	// group's members so the close-friends star is visible to everyone.
	if msgType == "swgc_cf" {
		if err := SyncCloseFriendsForGroups(accountID, []types.JID{gJID}); err != nil {
			log.Printf("[Account %d] Failed to sync close-friends list for SWGC-CF to %s: %v", accountID, groupJID, err)
		}
	}

	var waMsg *waE2E.Message
	if msgType == "swgc_cf" {
		waMsg, err = buildCloseFriendsMessage(client, msg, fileBytes, mime, background, emoji, listName)
	} else {
		waMsg, err = buildCustomWAMessage(client, msg, msgType, pollOptions, fileBytes, mime)
	}
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
