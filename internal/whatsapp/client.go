package whatsapp

import (
	"context"
	"fmt"
	"juraganxl-notif/internal/db"
	"juraganxl-notif/internal/models"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

var (
	Clients map[uint]*whatsmeow.Client = make(map[uint]*whatsmeow.Client)
)

// InitClient tries to initialize WAClients based on saved session db paths
func InitClient() {
	var accounts []models.Account
	db.DB.Find(&accounts)

	for _, acc := range accounts {
		var conf models.AppConfig
		db.DB.First(&conf, "account_id = ? AND key = ?", acc.ID, "whatsmeow_db_path")

		if conf.Value != "" && fileExists(conf.Value) {
			startClient(acc.ID, conf.Value)
		}
	}
}

// GenerateQR will create a fresh session DB for an account
func GenerateQR(accountID uint) (<-chan whatsmeow.QRChannelItem, error) {
	err := os.MkdirAll("sessions", 0755)
	if err != nil {
		return nil, err
	}

	if client, ok := Clients[accountID]; ok && client != nil {
		client.Disconnect()
		delete(Clients, accountID)
	}

	sessionFile := filepath.Join("sessions", fmt.Sprintf("wa_session_acc%d_%d.db", accountID, time.Now().Unix()))

	// Save the new DB path
	conf := models.AppConfig{
		AccountID: accountID,
		Key:       "whatsmeow_db_path",
		Value:     sessionFile,
	}
	db.DB.Save(&conf)

	client, err := createNewClient(accountID, sessionFile)
	if err != nil {
		return nil, err
	}
	Clients[accountID] = client

	qrChan, _ := client.GetQRChannel(context.Background())
	err = client.Connect()
	if err != nil {
		return nil, err
	}

	return qrChan, nil
}

// Logout disconnecting client and removing DB for a specific account
func Logout(accountID uint) {
	if client, ok := Clients[accountID]; ok && client != nil {
		client.Logout(context.Background())
		client.Disconnect()
		delete(Clients, accountID)
	}

	var accounts models.Account
	if err := db.DB.First(&accounts, accountID).Error; err == nil {
		accounts.IsConnected = false
		db.DB.Save(&accounts)
	}

	var conf models.AppConfig
	db.DB.First(&conf, "account_id = ? AND key = ?", accountID, "whatsmeow_db_path")
	if conf.Value != "" {
		os.Remove(conf.Value)
		db.DB.Delete(&models.AppConfig{}, "account_id = ? AND key = ?", accountID, "whatsmeow_db_path")
	}
}

func startClient(accountID uint, dbPath string) {
	client, err := createNewClient(accountID, dbPath)
	if err != nil {
		log.Printf("[Account %d] Failed to restore WA client: %v", accountID, err)
		return
	}
	Clients[accountID] = client
	if client.Store.ID != nil {
		err = client.Connect()
		if err != nil {
			log.Printf("[Account %d] Failed to connect WA client: %v", accountID, err)
		} else {
			log.Printf("[Account %d] WA Connected automatically.", accountID)
			db.DB.Model(&models.Account{}).Where("id = ?", accountID).Update("is_connected", true)
		}
	}
}

func createNewClient(accountID uint, dbPath string) (*whatsmeow.Client, error) {
	dbLog := waLog.Stdout(fmt.Sprintf("DB-%d", accountID), "WARN", true)
	container, err := sqlstore.New(context.Background(), "sqlite3", fmt.Sprintf("file:%s?_foreign_keys=on", dbPath), dbLog)
	if err != nil {
		return nil, err
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return nil, err
	}

	clientLog := waLog.Stdout(fmt.Sprintf("WA-%d", accountID), "WARN", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)

	// Inject AccountID into eventHandler
	client.AddEventHandler(func(evt interface{}) {
		eventHandler(accountID, evt)
	})

	return client, nil
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
