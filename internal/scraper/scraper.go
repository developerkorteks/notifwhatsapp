package scraper

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"juraganxl-notif/internal/db"
	"juraganxl-notif/internal/models"
	"juraganxl-notif/internal/whatsapp"

	"github.com/gocolly/colly/v2"
)

type Paket struct {
	Code  string `json:"code"`
	Stock string `json:"stock"`
}

type CircleStock struct {
	Config string `json:"config"`
	Count  int    `json:"count"`
}

func getCSRF() string {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", "https://juraganxl.my.id/api/csrf-token", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "csrf-token" {
			return cookie.Value
		}
	}
	return ""
}

func getCircleStock(csrf string) []CircleStock {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", "https://juraganxl.my.id/api/stocks-circle", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("x-csrf-token", csrf)
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var data []CircleStock
	json.NewDecoder(resp.Body).Decode(&data)
	return data
}

func CheckStockAndNotify() {
	var xdaPackages []Paket

	collector := colly.NewCollector()
	collector.OnRequest(func(r *colly.Request) {
		r.Headers.Set("User-Agent", "Mozilla/5.0")
	})

	collector.OnHTML(".overflow-hidden.border.rounded", func(e *colly.HTMLElement) {
		p := Paket{}
		p.Code = e.ChildText(".text-lg")
		stock := e.ChildText(".text-sm")
		p.Stock = strings.TrimSpace(strings.Replace(stock, "stock :", "", -1))

		if p.Code != "" && strings.HasPrefix(p.Code, "XDA") {
			xdaPackages = append(xdaPackages, p)
		}
	})

	err := collector.Visit("https://juraganxl.my.id/")
	if err != nil {
		log.Println("Scrape home fail:", err)
		return
	}

	csrf := getCSRF()
	xclpPackages := getCircleStock(csrf)

	// Filter XCLP config
	var filteredXCLP []CircleStock
	for _, c := range xclpPackages {
		if strings.HasPrefix(c.Config, "XCLP") {
			filteredXCLP = append(filteredXCLP, c)
		}
	}

	// Compare XDA
	xdaBytes, _ := json.Marshal(xdaPackages)
	xclpBytes, _ := json.Marshal(filteredXCLP)

	xdaChanged := hasChanged("XDA", string(xdaBytes))
	xclpChanged := hasChanged("XCLP", string(xclpBytes))

	if xdaChanged || xclpChanged {
		log.Println("Stock change detected! Formatting message...")
		msg := formatNotificationMessage(xdaPackages, filteredXCLP)

		if msg == "" {
			log.Println("All tracked items have 0 stock. Skipping broadcast.")
			return
		}

		// Broadcast it
		err := whatsapp.BroadcastStockMessage(msg)
		if err != nil {
			log.Println("Failed to broadcast stock message:", err)
		} else {
			log.Println("Stock broadcast sent successfully.")
		}
	} else {
		log.Println("No stock changes.")
	}
}

func hasChanged(key, newData string) bool {
	var mem models.StockMemory
	res := db.DB.First(&mem, "key = ?", key)
	if res.Error != nil {
		// First time seeing this
		newMem := models.StockMemory{Key: key, StockJSON: newData}
		db.DB.Create(&newMem)
		return true // treat first time as change to init
	}

	if mem.StockJSON != newData {
		mem.StockJSON = newData
		db.DB.Save(&mem)
		return true
	}

	return false
}

func formatNotificationMessage(xda []Paket, xclp []CircleStock) string {
	var xdaContent strings.Builder
	for _, p := range xda {
		if p.Stock != "0" && p.Stock != "" {
			xdaContent.WriteString(fmt.Sprintf("- %s = %s\n", p.Code, p.Stock))
		}
	}

	var xclpContent strings.Builder
	for _, p := range xclp {
		if p.Count > 0 {
			xclpContent.WriteString(fmt.Sprintf("- %s = %d\n", p.Config, p.Count))
		}
	}

	if xdaContent.Len() == 0 && xclpContent.Len() == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("📢 *UPDATE STOCK XDA & XCLP*\n")
	sb.WriteString("===========================\n\n")

	if xdaContent.Len() > 0 {
		sb.WriteString("*STOCK XDA:*\n")
		sb.WriteString(xdaContent.String())
	}

	if xclpContent.Len() > 0 {
		if xdaContent.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("*STOCK XCLP:*\n")
		sb.WriteString(xclpContent.String())
	}

	sb.WriteString("\n===========================\n")
	sb.WriteString("JuraganXL Notification Hub")

	return sb.String()
}
