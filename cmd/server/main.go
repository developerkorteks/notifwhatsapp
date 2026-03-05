package main

import (
	"juraganxl-notif/internal/api"
	"juraganxl-notif/internal/db"
	"juraganxl-notif/internal/scraper"
	"juraganxl-notif/internal/whatsapp"
	"log"
)

func main() {
	log.Println("Initializing System...")
	db.InitDB()
	
	log.Println("Initializing WhatsApp Client...")
	whatsapp.InitClient()

	log.Println("Starting Background Workers...")
	scraper.InitCron()

	log.Println("Starting Web Dashboard...")
	api.StartServer()
}
