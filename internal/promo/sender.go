package promo

import (
	"context"
	"crypto/rand"
	"juraganxl-notif/internal/db"
	"juraganxl-notif/internal/models"
	"juraganxl-notif/internal/utils"
	"juraganxl-notif/internal/whatsapp"
	"log"
	"math/big"
	mrand "math/rand"
	"os"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

func SendPromoToGroups(accountID uint, promo models.PromoMessage) {
	client, ok := whatsapp.Clients[accountID]
	if !ok || client == nil || !client.IsConnected() {
		log.Printf("[Promo %d] Client not connected, skipping", accountID)
		return
	}

	var activeChannel models.ChannelTarget
	if err := db.DB.First(&activeChannel, "account_id = ? AND is_active = ?", accountID, true).Error; err != nil {
		log.Printf("[Promo %d] No active channel, skipping", accountID)
		return
	}

	msg := promo.Message
	msgType := promo.MsgType
	var pollOptions []string
	if promo.PollOptions != "" {
		pollOptions = strings.Split(promo.PollOptions, "||")
	}

	var fileBytes []byte
	mime := promo.MimeType
	if promo.MediaPath != "" {
		data, err := os.ReadFile(promo.MediaPath)
		if err != nil {
			log.Printf("[Promo %d] Failed to read media file %s: %v", accountID, promo.MediaPath, err)
		} else {
			fileBytes = data
		}
	}

	// Text-to-image fallback for view_once
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
				fileBytes, _ = utils.CreateTextImage(msg)
				mime = "image/png"
			}
		}
	}

	// Build WA message
	var waMsg *waE2E.Message

	if len(fileBytes) > 0 && mime != "" {
		isViewOnce := proto.Bool(msgType == "view_once")
		if strings.HasPrefix(mime, "image") {
			resp, err := client.Upload(context.Background(), fileBytes, whatsmeow.MediaImage)
			if err != nil {
				log.Printf("[Promo %d] Upload failed: %v", accountID, err)
				return
			}
			waMsg = &waE2E.Message{
				ImageMessage: &waE2E.ImageMessage{
					Caption: proto.String(msg), Mimetype: proto.String(mime),
					URL: &resp.URL, DirectPath: &resp.DirectPath,
					MediaKey: resp.MediaKey, FileEncSHA256: resp.FileEncSHA256,
					FileSHA256: resp.FileSHA256, FileLength: &resp.FileLength,
					ViewOnce: isViewOnce,
				},
			}
		} else if strings.HasPrefix(mime, "video") {
			resp, err := client.Upload(context.Background(), fileBytes, whatsmeow.MediaVideo)
			if err != nil {
				log.Printf("[Promo %d] Upload failed: %v", accountID, err)
				return
			}
			waMsg = &waE2E.Message{
				VideoMessage: &waE2E.VideoMessage{
					Caption: proto.String(msg), Mimetype: proto.String(mime),
					URL: &resp.URL, DirectPath: &resp.DirectPath,
					MediaKey: resp.MediaKey, FileEncSHA256: resp.FileEncSHA256,
					FileSHA256: resp.FileSHA256, FileLength: &resp.FileLength,
					ViewOnce: isViewOnce,
				},
			}
		} else if strings.HasPrefix(mime, "audio") {
			resp, err := client.Upload(context.Background(), fileBytes, whatsmeow.MediaAudio)
			if err != nil {
				log.Printf("[Promo %d] Upload failed: %v", accountID, err)
				return
			}
			waMsg = &waE2E.Message{
				AudioMessage: &waE2E.AudioMessage{
					Mimetype: proto.String(mime),
					URL: &resp.URL, DirectPath: &resp.DirectPath,
					MediaKey: resp.MediaKey, FileEncSHA256: resp.FileEncSHA256,
					FileSHA256: resp.FileSHA256, FileLength: &resp.FileLength,
				},
			}
		}
	}

	if waMsg == nil {
		if msgType == "poll" && len(pollOptions) >= 2 {
			waMsg = client.BuildPollCreation(msg, pollOptions, 1)
		} else {
			extended := &waE2E.ExtendedTextMessage{Text: proto.String(msg)}
			if msgType == "swgc" {
				fontType := waE2E.ExtendedTextMessage_SYSTEM
				extended.BackgroundArgb = proto.Uint32(0xFF0F8A5F)
				extended.TextArgb = proto.Uint32(0xFFFFFFFF)
				extended.Font = &fontType
			}
			waMsg = &waE2E.Message{ExtendedTextMessage: extended}
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
			MessageContextInfo: &waE2E.MessageContextInfo{MessageSecret: messageSecret},
			GroupStatusMessageV2: &waE2E.FutureProofMessage{Message: innerMsg},
		}
	}

	// Send to channel first (standard text only)
	if msgType == "standard" {
		chJID, err := whatsapp.ParseJID(activeChannel.JID)
		if err == nil {
			resp, err := client.SendMessage(context.Background(), chJID, waMsg)
			if err == nil {
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
		}
	}

	// Get custom-active groups and shuffle
	var groups []models.GroupTarget
	db.DB.Where("account_id = ? AND is_custom_active = ?", accountID, true).Find(&groups)

	if len(groups) == 0 {
		log.Printf("[Promo %d] No custom-active groups to send to", accountID)
		return
	}

	mrand.Shuffle(len(groups), func(i, j int) { groups[i], groups[j] = groups[j], groups[i] })

	log.Printf("[Promo %d] Sending promo (%s) to %d groups one-by-one", accountID, msgType, len(groups))

	for i, g := range groups {
		gJID, err := whatsapp.ParseJID(g.JID)
		if err != nil {
			continue
		}

		client.SendChatPresence(context.Background(), gJID, types.ChatPresenceComposing, types.ChatPresenceMediaText)
		time.Sleep(2 * time.Second)
		client.SendChatPresence(context.Background(), gJID, types.ChatPresencePaused, types.ChatPresenceMediaText)

		_, err = client.SendMessage(context.Background(), gJID, waMsg)
		if err != nil {
			log.Printf("[Promo %d] Failed to send to group %s: %v", accountID, g.GroupName, err)
		} else {
			log.Printf("[Promo %d] Sent to group %d/%d: %s", accountID, i+1, len(groups), g.GroupName)
		}

		if i < len(groups)-1 {
			delay := randomDelay(5*60, 15*60)
			log.Printf("[Promo %d] Waiting %d seconds before next group", accountID, delay)
			time.Sleep(time.Duration(delay) * time.Second)
		}
	}

	log.Printf("[Promo %d] Promo round complete", accountID)
}

func randomDelay(minSec, maxSec int) int {
	n, _ := big.NewInt(0).SetString("0", 10)
	n, _ = rand.Int(rand.Reader, big.NewInt(int64(maxSec-minSec+1)))
	return minSec + int(n.Int64())
}
