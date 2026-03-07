package models

import "gorm.io/gorm"

type AppConfig struct {
	Key   string `gorm:"primaryKey"`
	Value string
}

type GroupTarget struct {
	JID            string `gorm:"column:jid;primaryKey"`
	GroupName      string
	IsStockActive  bool
	IsCustomActive bool
	IsAntiSwgcActive bool
}

type ChannelTarget struct {
	JID         string `gorm:"column:jid;primaryKey"`
	ChannelName string
	IsActive    bool
}

type StockMemory struct {
	gorm.Model
	Key       string `gorm:"uniqueIndex"`
	StockJSON string
}
