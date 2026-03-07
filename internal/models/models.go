package models

import "gorm.io/gorm"

type Account struct {
	ID          uint   `gorm:"primaryKey;autoIncrement"`
	SessionName string `gorm:"unique"`
	IsConnected bool
}

type AppConfig struct {
	AccountID uint   `gorm:"primaryKey"`
	Key       string `gorm:"primaryKey"`
	Value     string
}

type GroupTarget struct {
	AccountID        uint   `gorm:"primaryKey"`
	JID              string `gorm:"column:jid;primaryKey"`
	GroupName        string
	IsStockActive    bool
	IsCustomActive   bool
	IsAntiSwgcActive bool
}

type ChannelTarget struct {
	AccountID   uint   `gorm:"primaryKey"`
	JID         string `gorm:"column:jid;primaryKey"`
	ChannelName string
	IsActive    bool
}

type StockMemory struct {
	gorm.Model
	Key       string `gorm:"uniqueIndex"`
	StockJSON string
}
