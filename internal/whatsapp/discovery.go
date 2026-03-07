package whatsapp

import (
	"context"
	"errors"
	"juraganxl-notif/internal/db"
	"juraganxl-notif/internal/models"

	"go.mau.fi/whatsmeow/types"
)

// SyncGroups fetches all joined groups and saves them to DB if not exist
func SyncGroups() error {
	if WAClient == nil || !WAClient.IsConnected() {
		return errors.New("WhatsApp client is not connected")
	}

	groups, err := WAClient.GetJoinedGroups(context.Background())
	if err != nil {
		return err
	}

	for _, g := range groups {
		var existing models.GroupTarget
		res := db.DB.First(&existing, "jid = ?", g.JID.String())
		if res.Error != nil { // Not found, create new
			newGroup := models.GroupTarget{
				JID:              g.JID.String(),
				GroupName:        g.Name,
				IsStockActive:    false,
				IsCustomActive:   false,
				IsAntiSwgcActive: false,
			}
			db.DB.Create(&newGroup)
		} else { // Update name in case it changed
			existing.GroupName = g.Name
			db.DB.Save(&existing)
		}
	}
	return nil
}

// SyncChannels fetches all subscribed newsletters and saves them to DB
func SyncChannels() error {
	if WAClient == nil || !WAClient.IsConnected() {
		return errors.New("WhatsApp client is not connected")
	}

	channels, err := WAClient.GetSubscribedNewsletters(context.Background())
	if err != nil {
		return err
	}

	for _, ch := range channels {
		var existing models.ChannelTarget
		res := db.DB.First(&existing, "jid = ?", ch.ID.String())
		if res.Error != nil {
			newCh := models.ChannelTarget{
				JID:         ch.ID.String(),
				ChannelName: ch.ThreadMeta.Name.Text,
				IsActive:    false,
			}
			db.DB.Create(&newCh)
		} else {
			existing.ChannelName = ch.ThreadMeta.Name.Text
			db.DB.Save(&existing)
		}
	}
	return nil
}

// GetDBGroups returns all known groups from db
func GetDBGroups() []models.GroupTarget {
	var groups []models.GroupTarget
	db.DB.Find(&groups)
	return groups
}

// GetDBChannels returns all known channels from db
func GetDBChannels() []models.ChannelTarget {
	var channels []models.ChannelTarget
	db.DB.Find(&channels)
	return channels
}

func UpdateGroupSettings(jid string, isStock, isCustom, isAntiSwgc bool) error {
	var g models.GroupTarget
	if err := db.DB.First(&g, "jid = ?", jid).Error; err != nil {
		return err
	}
	g.IsStockActive = isStock
	g.IsCustomActive = isCustom
	g.IsAntiSwgcActive = isAntiSwgc
	return db.DB.Save(&g).Error
}

func SetActiveChannel(jid string) error {
	// Deactivate all first
	db.DB.Model(&models.ChannelTarget{}).Where("is_active = ?", true).Update("is_active", false)
	// Activate selected
	return db.DB.Model(&models.ChannelTarget{}).Where("jid = ?", jid).Update("is_active", true).Error
}

// ParseJID helper function
func ParseJID(jid string) (types.JID, error) {
	return types.ParseJID(jid)
}
