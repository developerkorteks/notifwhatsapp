package scraper

import (
	"log"

	"github.com/robfig/cron/v3"
)

var c *cron.Cron

func InitCron() {
	c = cron.New()
	
	// Run every 5 minutes
	// Adjust as needed: "*/5 * * * *"
	_, err := c.AddFunc("*/2 * * * *", func() {
		log.Println("Cron: Checking JuraganXL Stock...")
		CheckStockAndNotify()
	})
	if err != nil {
		log.Fatalf("Failed to add cron job: %v", err)
	}

	c.Start()
	log.Println("Cron job started. Will check every 2 minutes.")
}
