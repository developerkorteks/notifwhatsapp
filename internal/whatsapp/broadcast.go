package whatsapp

import (
	"context"
	"errors"
	"juraganxl-notif/internal/db"
	"juraganxl-notif/internal/models"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

// BroadcastCustomMessage sends msg to Active Channel, then forwards it (sends to) all active Custom Groups
func BroadcastCustomMessage(msg string) error {
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

	// 3. Fetch Target Custom Groups
	var groups []models.GroupTarget
	db.DB.Where("is_custom_active = ?", true).Find(&groups)

	// 4. Loop and send to Groups
	for _, g := range groups {
		gJID, err := ParseJID(g.JID)
		if err == nil {
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
			WAClient.SendMessage(context.Background(), gJID, waMsg)
		}
	}

	return nil
}
