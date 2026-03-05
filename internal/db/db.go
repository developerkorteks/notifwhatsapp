package db

import (
	"juraganxl-notif/internal/models"
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() {
	var err error
	DB, err = gorm.Open(sqlite.Open("notif.db"), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to local database: %v", err)
	}

	// Auto Migrate the schema
	err = DB.AutoMigrate(
		&models.AppConfig{},
		&models.GroupTarget{},
		&models.ChannelTarget{},
		&models.StockMemory{},
	)
	if err != nil {
		log.Fatalf("failed to auto migrate database schema: %v", err)
	}

	log.Println("Database initialized and migrated successfully.")}
