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
	WAClient *whatsmeow.Client
)

// InitClient tries to initialize WAClient based on saved session db path
func InitClient() {
	var conf models.AppConfig
	db.DB.First(&conf, "key =?", "whatsmeow_db_path")

	if conf.Value != "" && fileExists(conf.Value) {
		startClient(conf.Value)
	}
}

// GenerateQR will create a fresh session DB, disconnect old client if any, and return QR strings channel
func GenerateQR() (<-chan whatsmeow.QRChannelItem, error) {
	err := os.MkdirAll("sessions", 0755)
	if err != nil {
		return nil, err
	}

	if WAClient != nil {
		WAClient.Disconnect()
		WAClient = nil
	}

	sessionFile := filepath.Join("sessions", fmt.Sprintf("wa_session_%d.db", time.Now().Unix()))

	// Save the new DB path
	conf := models.AppConfig{
		Key:   "whatsmeow_db_path",
		Value: sessionFile,
	}
	db.DB.Save(&conf)

	client, err := createNewClient(sessionFile)
	if err != nil {
		return nil, err
	}
	WAClient = client

	qrChan, _ := WAClient.GetQRChannel(context.Background())
	err = WAClient.Connect()
	if err != nil {
		return nil, err
	}

	return qrChan, nil
}

// Logout disconnecting client and removing DB
func Logout() {
	if WAClient != nil {
		WAClient.Logout(context.Background())
		WAClient.Disconnect()
		WAClient = nil
	}

	var conf models.AppConfig
	db.DB.First(&conf, "key =?", "whatsmeow_db_path")
	if conf.Value != "" {
		os.Remove(conf.Value)
		db.DB.Delete(&models.AppConfig{}, "key =?", "whatsmeow_db_path")
	}
}

func startClient(dbPath string) {
	client, err := createNewClient(dbPath)
	if err != nil {
		log.Printf("Failed to restore WA client: %v", err)
		return
	}
	WAClient = client
	if WAClient.Store.ID != nil {
		err = WAClient.Connect()
		if err != nil {
			log.Printf("Failed to connect WA client: %v", err)
		} else {
			log.Println("WA Connected automatically.")
		}
	}
}

func createNewClient(dbPath string) (*whatsmeow.Client, error) {
	dbLog := waLog.Stdout("Database", "WARN", true)
	container, err := sqlstore.New(context.Background(), "sqlite3", fmt.Sprintf("file:%s?_foreign_keys=on", dbPath), dbLog)
	if err != nil {
		return nil, err
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return nil, err
	}

	clientLog := waLog.Stdout("Client", "WARN", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)

	client.AddEventHandler(eventHandler)
	return client, nil
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
