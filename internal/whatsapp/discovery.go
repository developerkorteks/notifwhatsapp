package whatsapp

import (
	"context"
	"errors"
	"juraganxl-notif/internal/db"
	"juraganxl-notif/internal/models"
	"log"

	"go.mau.fi/whatsmeow/types"
)

// SyncGroups fetches all joined groups and saves them to DB if not exist
func SyncGroups(accountID uint) error {
	client, ok := Clients[accountID]
	if !ok || client == nil || !client.IsConnected() {
		return errors.New("WhatsApp client is not connected")
	}

	groups, err := client.GetJoinedGroups(context.Background())
	if err != nil {
		return err
	}

	for _, g := range groups {
		var existing models.GroupTarget
		res := db.DB.First(&existing, "account_id = ? AND jid = ?", accountID, g.JID.String())
		if res.Error != nil { // Not found, create new
			newGroup := models.GroupTarget{
				AccountID:        accountID,
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
func SyncChannels(accountID uint) error {
	client, ok := Clients[accountID]
	if !ok || client == nil || !client.IsConnected() {
		return errors.New("WhatsApp client is not connected")
	}

	channels, err := client.GetSubscribedNewsletters(context.Background())
	if err != nil {
		return err
	}

	for _, ch := range channels {
		var existing models.ChannelTarget
		res := db.DB.First(&existing, "account_id = ? AND jid = ?", accountID, ch.ID.String())
		if res.Error != nil {
			newCh := models.ChannelTarget{
				AccountID:   accountID,
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
func GetDBGroups(accountID uint) []models.GroupTarget {
	var groups []models.GroupTarget
	db.DB.Where("account_id = ?", accountID).Find(&groups)
	return groups
}

// GetDBChannels returns all known channels from db
func GetDBChannels(accountID uint) []models.ChannelTarget {
	var channels []models.ChannelTarget
	db.DB.Where("account_id = ?", accountID).Find(&channels)
	return channels
}

func UpdateGroupSettings(accountID uint, jid string, isStock, isCustom, isAntiSwgc bool) error {
	var g models.GroupTarget
	if err := db.DB.First(&g, "account_id = ? AND jid = ?", accountID, jid).Error; err != nil {
		return err
	}
	g.IsStockActive = isStock
	g.IsCustomActive = isCustom
	g.IsAntiSwgcActive = isAntiSwgc
	return db.DB.Save(&g).Error
}

func SetActiveChannel(accountID uint, jid string) error {
	// Deactivate all first
	db.DB.Model(&models.ChannelTarget{}).Where("account_id = ? AND is_active = ?", accountID, true).Update("is_active", false)
	// Activate selected
	return db.DB.Model(&models.ChannelTarget{}).Where("account_id = ? AND jid = ?", accountID, jid).Update("is_active", true).Error
}

// ParseJID helper function
func ParseJID(jid string) (types.JID, error) {
	return types.ParseJID(jid)
}

func InsertJoinedGroup(accountID uint, jid types.JID, groupName string) {
	var existing models.GroupTarget
	res := db.DB.First(&existing, "account_id = ? AND jid = ?", accountID, jid.String())
	if res.Error != nil {
		newGroup := models.GroupTarget{
			AccountID:        accountID,
			JID:              jid.String(),
			GroupName:        groupName,
			IsStockActive:    false,
			IsCustomActive:   true,
			IsAntiSwgcActive: false,
		}
		db.DB.Create(&newGroup)
		log.Printf("[Account %d] Auto-Join: group %s (%s) added to database with custom active", accountID, groupName, jid.String())
	}
}

func GetGroupStats(accountID uint) (customActive int64, total int64) {
	db.DB.Model(&models.GroupTarget{}).Where("account_id = ?", accountID).Count(&total)
	db.DB.Model(&models.GroupTarget{}).Where("account_id = ? AND is_custom_active = ?", accountID, true).Count(&customActive)
	return
}

func SetAllCustomActive(accountID uint, enabled bool) error {
	return db.DB.Model(&models.GroupTarget{}).Where("account_id = ?", accountID).Update("is_custom_active", enabled).Error
}
