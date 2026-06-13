package promo

import (
	"context"
	"crypto/rand"
	"juraganxl-notif/internal/db"
	"juraganxl-notif/internal/models"
	"log"
	"math/big"
	mrand "math/rand"
	"strconv"
	"sync"
	"time"
)

const (
	startHour = 8
	endHour   = 21
	defaultSendsPerDay = 5
)

var (
	schedulers   = make(map[uint]context.CancelFunc)
	schedulersMu sync.Mutex
	sendingFlags = make(map[uint]bool)
	sendingMu    sync.Mutex
)

func StartScheduler() {
	var accounts []models.Account
	db.DB.Find(&accounts)

	for _, acc := range accounts {
		if isPromoEnabled(acc.ID) {
			startAccountScheduler(acc.ID)
		}
	}

	log.Println("[Promo] Scheduler initialized")
}

func RestartAccountScheduler(accountID uint) {
	StopAccountScheduler(accountID)
	if isPromoEnabled(accountID) {
		startAccountScheduler(accountID)
	}
}

func StopAccountScheduler(accountID uint) {
	schedulersMu.Lock()
	defer schedulersMu.Unlock()

	if cancel, ok := schedulers[accountID]; ok {
		cancel()
		delete(schedulers, accountID)
		log.Printf("[Promo %d] Scheduler stopped", accountID)
	}
}

func startAccountScheduler(accountID uint) {
	schedulersMu.Lock()
	defer schedulersMu.Unlock()

	if _, ok := schedulers[accountID]; ok {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	schedulers[accountID] = cancel

	go runSchedulerLoop(ctx, accountID)
	log.Printf("[Promo %d] Scheduler started", accountID)
}

func runSchedulerLoop(ctx context.Context, accountID uint) {
	for {
		sendsPerDay := getSendsPerDay(accountID)
		times := generateRandomTimes(sendsPerDay)

		log.Printf("[Promo %d] Generated %d send times for today: %v", accountID, len(times), formatTimes(times))

		for _, t := range times {
			now := time.Now()
			if t.Before(now) {
				continue
			}

			wait := time.Until(t)
			log.Printf("[Promo %d] Next send at %s (in %s)", accountID, t.Format("15:04"), wait.Round(time.Minute))

			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
				if !isPromoEnabled(accountID) {
					log.Printf("[Promo %d] Promo disabled, skipping", accountID)
					continue
				}

				sendingMu.Lock()
				isSending := sendingFlags[accountID]
				sendingMu.Unlock()

				if isSending {
					log.Printf("[Promo %d] Previous send still in progress, skipping", accountID)
					continue
				}

				go executeRound(accountID)
			}
		}

		// Sleep until tomorrow 08:00
		tomorrow := nextStartTime()
		wait := time.Until(tomorrow)
		log.Printf("[Promo %d] Day complete. Next day starts at %s (in %s)", accountID, tomorrow.Format("2006-01-02 15:04"), wait.Round(time.Minute))

		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

func executeRound(accountID uint) {
	sendingMu.Lock()
	sendingFlags[accountID] = true
	sendingMu.Unlock()

	defer func() {
		sendingMu.Lock()
		sendingFlags[accountID] = false
		sendingMu.Unlock()
	}()

	// Pick a random active promo message
	var promos []models.PromoMessage
	db.DB.Where("account_id = ? AND is_active = ?", accountID, true).Find(&promos)

	if len(promos) == 0 {
		log.Printf("[Promo %d] No active promo messages in pool, skipping", accountID)
		return
	}

	idx := mrand.Intn(len(promos))
	selected := promos[idx]

	log.Printf("[Promo %d] Selected promo #%d (%s): %.50s...", accountID, selected.ID, selected.MsgType, selected.Message)
	SendPromoToGroups(accountID, selected)
}

func generateRandomTimes(n int) []time.Time {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	startMin := startHour * 60
	endMin := endHour * 60
	rangeMin := endMin - startMin

	times := make([]time.Time, 0, n)
	for i := 0; i < n; i++ {
		rn, _ := rand.Int(rand.Reader, big.NewInt(int64(rangeMin)))
		minuteOffset := startMin + int(rn.Int64())
		t := today.Add(time.Duration(minuteOffset) * time.Minute)
		times = append(times, t)
	}

	// Sort ascending
	for i := 0; i < len(times); i++ {
		for j := i + 1; j < len(times); j++ {
			if times[j].Before(times[i]) {
				times[i], times[j] = times[j], times[i]
			}
		}
	}

	return times
}

func nextStartTime() time.Time {
	now := time.Now()
	tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, startHour, 0, 0, 0, now.Location())
	return tomorrow
}

func isPromoEnabled(accountID uint) bool {
	var conf models.AppConfig
	if err := db.DB.First(&conf, "account_id = ? AND key = ?", accountID, "promo_enabled").Error; err != nil {
		return false
	}
	return conf.Value == "true"
}

func getSendsPerDay(accountID uint) int {
	var conf models.AppConfig
	if err := db.DB.First(&conf, "account_id = ? AND key = ?", accountID, "promo_sends_per_day").Error; err != nil {
		return defaultSendsPerDay
	}
	n, err := strconv.Atoi(conf.Value)
	if err != nil || n < 1 {
		return defaultSendsPerDay
	}
	return n
}

func formatTimes(times []time.Time) string {
	s := ""
	for i, t := range times {
		if i > 0 {
			s += ", "
		}
		s += t.Format("15:04")
	}
	return s
}

func IsSending(accountID uint) bool {
	sendingMu.Lock()
	defer sendingMu.Unlock()
	return sendingFlags[accountID]
}

func IsSchedulerRunning(accountID uint) bool {
	schedulersMu.Lock()
	defer schedulersMu.Unlock()
	_, ok := schedulers[accountID]
	return ok
}
