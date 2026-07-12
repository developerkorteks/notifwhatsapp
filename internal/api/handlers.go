package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"juraganxl-notif/internal/db"
	"juraganxl-notif/internal/models"
	"juraganxl-notif/internal/promo"
	"juraganxl-notif/internal/whatsapp"

	"github.com/gin-gonic/gin"
)

func RegisterHandlers(r *gin.Engine) {
	api := r.Group("/api")
	{
		api.GET("/accounts", getAccounts)
		api.POST("/accounts", createAccount)
		api.DELETE("/accounts/:id", deleteAccount)

		api.GET("/wa/status", getStatus)
		api.GET("/wa/qr", generateQR)
		api.POST("/wa/logout", logoutWA)

		api.GET("/wa/groups", getGroups)
		api.POST("/wa/groups/sync", syncGroups)
		api.POST("/wa/groups/settings", updateGroupSettings)

		api.GET("/wa/channels", getChannels)
		api.POST("/wa/channels/sync", syncChannels)
		api.POST("/wa/channels/active", setActiveChannel)

		api.POST("/broadcast/custom", sendCustomBroadcast)
		api.POST("/broadcast/group", sendGroupBroadcast)

		api.POST("/wa/close-friends/sync", syncCloseFriends)
		api.POST("/wa/close-friends/reset", resetCloseFriends)

		api.GET("/settings/auto-join", getAutoJoinSetting)
		api.POST("/settings/auto-join", setAutoJoinSetting)

		api.GET("/wa/groups/stats", getGroupStats)
		api.POST("/wa/groups/toggle-all-custom", toggleAllCustom)

		api.GET("/promo/messages", getPromoMessages)
		api.POST("/promo/messages", createPromoMessage)
		api.DELETE("/promo/messages/:id", deletePromoMessage)
		api.POST("/promo/messages/:id/toggle", togglePromoMessage)
		api.PUT("/promo/messages/:id", updatePromoMessage)
		api.GET("/promo/settings", getPromoSettings)
		api.POST("/promo/settings", setPromoSettings)
		api.POST("/promo/test", testPromoSend)
	}

	// Serve static files
	r.Static("/static", "./web/public")
	r.GET("/", func(c *gin.Context) {
		c.File("./web/public/index.html")
	})
}

func getAccounts(c *gin.Context) {
	var accounts []models.Account
	db.DB.Find(&accounts)
	c.JSON(http.StatusOK, accounts)
}

type CreateAccountReq struct {
	SessionName string `json:"session_name" binding:"required"`
}

func createAccount(c *gin.Context) {
	var req CreateAccountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	acc := models.Account{SessionName: req.SessionName}
	if err := db.DB.Create(&acc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, acc)
}

func deleteAccount(c *gin.Context) {
	idStr := c.Param("id")
	id, _ := strconv.ParseUint(idStr, 10, 32)
	accountID := uint(id)

	whatsapp.Logout(accountID)
	db.DB.Delete(&models.Account{}, accountID)
	db.DB.Where("account_id = ?", accountID).Delete(&models.GroupTarget{})
	db.DB.Where("account_id = ?", accountID).Delete(&models.ChannelTarget{})

	c.JSON(http.StatusOK, gin.H{"message": "Account deleted"})
}

func getStatus(c *gin.Context) {
	accountIDStr := c.Query("account_id")
	accountID, _ := strconv.ParseUint(accountIDStr, 10, 32)

	client, ok := whatsapp.Clients[uint(accountID)]
	if ok && client != nil && client.IsConnected() && client.IsLoggedIn() {
		c.JSON(http.StatusOK, gin.H{"status": "connected"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "disconnected"})
}

func generateQR(c *gin.Context) {
	accountIDStr := c.Query("account_id")
	accountID, _ := strconv.ParseUint(accountIDStr, 10, 32)

	qrChan, err := whatsapp.GenerateQR(uint(accountID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Wait for the first code item
	for evt := range qrChan {
		if evt.Event == "code" {
			c.JSON(http.StatusOK, gin.H{"qr_code": evt.Code})
			return
		}
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate QR"})
}

func logoutWA(c *gin.Context) {
	accountIDStr := c.Query("account_id") // or POST payload, but frontend uses query or we'll pass it in POST body
	if accountIDStr == "" {
		accountIDStr = c.PostForm("account_id")
	}
	accountID, _ := strconv.ParseUint(accountIDStr, 10, 32)

	whatsapp.Logout(uint(accountID))
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

func getGroups(c *gin.Context) {
	accountIDStr := c.Query("account_id")
	accountID, _ := strconv.ParseUint(accountIDStr, 10, 32)

	groups := whatsapp.GetDBGroups(uint(accountID))
	c.JSON(http.StatusOK, groups)
}

func syncGroups(c *gin.Context) {
	accountIDStr := c.Query("account_id")
	if accountIDStr == "" {
		accountIDStr = c.PostForm("account_id")
	}
	accountID, _ := strconv.ParseUint(accountIDStr, 10, 32)

	if err := whatsapp.SyncGroups(uint(accountID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Groups synced"})
}

type GroupSettingReq struct {
	AccountID        uint   `json:"account_id"`
	JID              string `json:"jid"`
	IsStockActive    bool   `json:"is_stock_active"`
	IsCustomActive   bool   `json:"is_custom_active"`
	IsAntiSwgcActive bool   `json:"is_anti_swgc_active"`
}

func updateGroupSettings(c *gin.Context) {
	var req GroupSettingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := whatsapp.UpdateGroupSettings(req.AccountID, req.JID, req.IsStockActive, req.IsCustomActive, req.IsAntiSwgcActive); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Settings updated"})
}

func getChannels(c *gin.Context) {
	accountIDStr := c.Query("account_id")
	accountID, _ := strconv.ParseUint(accountIDStr, 10, 32)

	channels := whatsapp.GetDBChannels(uint(accountID))
	c.JSON(http.StatusOK, channels)
}

func syncChannels(c *gin.Context) {
	accountIDStr := c.Query("account_id")
	if accountIDStr == "" {
		accountIDStr = c.PostForm("account_id")
	}
	accountID, _ := strconv.ParseUint(accountIDStr, 10, 32)

	if err := whatsapp.SyncChannels(uint(accountID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Channels synced"})
}

type ActiveChannelReq struct {
	AccountID uint   `json:"account_id"`
	JID       string `json:"jid"`
}

func setActiveChannel(c *gin.Context) {
	var req ActiveChannelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := whatsapp.SetActiveChannel(req.AccountID, req.JID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Active channel updated"})
}

func sendCustomBroadcast(c *gin.Context) {
	if err := c.Request.ParseMultipartForm(10 << 20); err != nil { // limit 10MB
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form data"})
		return
	}

	accountIDStr := c.PostForm("account_id")
	accountID, _ := strconv.ParseUint(accountIDStr, 10, 32)

	msg := c.PostForm("message")
	msgType := c.PostForm("msg_type")
	background := c.PostForm("background")
	cfEmoji := c.PostForm("cf_emoji")
	cfName := c.PostForm("cf_name")
	pollOptsRaw := c.PostForm("poll_options")

	var pollOptions []string
	if pollOptsRaw != "" {
		pollOptions = strings.Split(pollOptsRaw, "||")
	}

	if msg == "" && msgType != "standard" { // relaxed check
		// message can be empty if it's media alone, unless we have constraints
	}

	var fileBytes []byte
	var mimeType string

	file, header, err := c.Request.FormFile("media")
	if err == nil {
		defer file.Close()
		fileBytes, _ = io.ReadAll(file)
		mimeType = header.Header.Get("Content-Type")
	}

	err = whatsapp.BroadcastCustomMessage(uint(accountID), msg, msgType, pollOptions, fileBytes, mimeType, background, cfEmoji, cfName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Broadcast sent"})
}

func sendGroupBroadcast(c *gin.Context) {
	if err := c.Request.ParseMultipartForm(10 << 20); err != nil { // limit 10MB
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form data"})
		return
	}

	accountIDStr := c.PostForm("account_id")
	accountID, _ := strconv.ParseUint(accountIDStr, 10, 32)
	jid := c.PostForm("jid")
	msg := c.PostForm("message")
	msgType := c.PostForm("msg_type")
	if msgType == "" {
		msgType = "standard"
	}
	background := c.PostForm("background")
	cfEmoji := c.PostForm("cf_emoji")
	cfName := c.PostForm("cf_name")
	pollOptsRaw := c.PostForm("poll_options")

	if jid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Target group jid is required"})
		return
	}

	var pollOptions []string
	if pollOptsRaw != "" {
		pollOptions = strings.Split(pollOptsRaw, "||")
	}
	if msgType == "poll" && len(pollOptions) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Poll must have at least 2 options"})
		return
	}

	var fileBytes []byte
	var mimeType string

	file, header, err := c.Request.FormFile("media")
	if err == nil {
		defer file.Close()
		fileBytes, _ = io.ReadAll(file)
		mimeType = header.Header.Get("Content-Type")
	}

	if msg == "" && len(fileBytes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Message or media is required"})
		return
	}

	err = whatsapp.SendCustomMessageToGroup(uint(accountID), jid, msg, msgType, pollOptions, fileBytes, mimeType, background, cfEmoji, cfName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Message sent to group"})
}

func syncCloseFriends(c *gin.Context) {
	accountIDStr := c.PostForm("account_id")
	if accountIDStr == "" {
		accountIDStr = c.Query("account_id")
	}
	accountID, _ := strconv.ParseUint(accountIDStr, 10, 32)

	if err := whatsapp.SyncCloseFriendsFromActiveGroups(uint(accountID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Close-friends list synced from active groups"})
}

func resetCloseFriends(c *gin.Context) {
	accountIDStr := c.PostForm("account_id")
	if accountIDStr == "" {
		accountIDStr = c.Query("account_id")
	}
	accountID, _ := strconv.ParseUint(accountIDStr, 10, 32)

	if err := whatsapp.ResetCloseFriendsList(uint(accountID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Close-friends list reset to default (contacts)"})
}

func getAutoJoinSetting(c *gin.Context) {
	accountIDStr := c.Query("account_id")
	accountID, _ := strconv.ParseUint(accountIDStr, 10, 32)

	var conf models.AppConfig
	enabled := false
	if err := db.DB.First(&conf, "account_id = ? AND key = ?", uint(accountID), "auto_join_enabled").Error; err == nil {
		enabled = conf.Value == "true"
	}

	c.JSON(http.StatusOK, gin.H{"enabled": enabled})
}

type AutoJoinReq struct {
	AccountID uint `json:"account_id"`
	Enabled   bool `json:"enabled"`
}

func setAutoJoinSetting(c *gin.Context) {
	var req AutoJoinReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	value := "false"
	if req.Enabled {
		value = "true"
	}

	upsertAppConfig(req.AccountID, "auto_join_enabled", value)

	c.JSON(http.StatusOK, gin.H{"message": "Auto-join setting updated", "enabled": req.Enabled})
}

func getGroupStats(c *gin.Context) {
	accountIDStr := c.Query("account_id")
	accountID, _ := strconv.ParseUint(accountIDStr, 10, 32)

	customActive, total := whatsapp.GetGroupStats(uint(accountID))
	c.JSON(http.StatusOK, gin.H{"custom_active": customActive, "total": total})
}

type ToggleAllCustomReq struct {
	AccountID uint `json:"account_id"`
	Enabled   bool `json:"enabled"`
}

func toggleAllCustom(c *gin.Context) {
	var req ToggleAllCustomReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := whatsapp.SetAllCustomActive(req.AccountID, req.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "All groups custom updated", "enabled": req.Enabled})
}

// --- Promo Handlers ---

func getPromoMessages(c *gin.Context) {
	accountIDStr := c.Query("account_id")
	accountID, _ := strconv.ParseUint(accountIDStr, 10, 32)

	var messages []models.PromoMessage
	db.DB.Where("account_id = ?", uint(accountID)).Order("created_at desc").Find(&messages)
	c.JSON(http.StatusOK, messages)
}

func createPromoMessage(c *gin.Context) {
	if err := c.Request.ParseMultipartForm(10 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form data"})
		return
	}

	accountIDStr := c.PostForm("account_id")
	accountID, _ := strconv.ParseUint(accountIDStr, 10, 32)
	msg := c.PostForm("message")
	msgType := c.PostForm("msg_type")
	pollOptions := c.PostForm("poll_options")

	var mediaPath string
	var mimeType string

	file, header, err := c.Request.FormFile("media")
	if err == nil {
		defer file.Close()
		fileBytes, _ := io.ReadAll(file)
		mimeType = header.Header.Get("Content-Type")

		os.MkdirAll("media/promo", 0755)
		filename := fmt.Sprintf("%d_%d_%s", accountID, time.Now().UnixMilli(), header.Filename)
		mediaPath = filepath.Join("media", "promo", filename)
		os.WriteFile(mediaPath, fileBytes, 0644)
	}

	pm := models.PromoMessage{
		AccountID:   uint(accountID),
		Message:     msg,
		MsgType:     msgType,
		PollOptions: pollOptions,
		MediaPath:   mediaPath,
		MimeType:    mimeType,
		IsActive:    true,
	}
	if err := db.DB.Create(&pm).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, pm)
}

func deletePromoMessage(c *gin.Context) {
	idStr := c.Param("id")
	id, _ := strconv.ParseUint(idStr, 10, 32)

	var pm models.PromoMessage
	if err := db.DB.First(&pm, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Message not found"})
		return
	}

	if pm.MediaPath != "" {
		os.Remove(pm.MediaPath)
	}

	db.DB.Delete(&pm)
	c.JSON(http.StatusOK, gin.H{"message": "Promo message deleted"})
}

func togglePromoMessage(c *gin.Context) {
	idStr := c.Param("id")
	id, _ := strconv.ParseUint(idStr, 10, 32)

	var pm models.PromoMessage
	if err := db.DB.First(&pm, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Message not found"})
		return
	}

	pm.IsActive = !pm.IsActive
	db.DB.Save(&pm)
	c.JSON(http.StatusOK, gin.H{"message": "Toggled", "is_active": pm.IsActive})
}

func updatePromoMessage(c *gin.Context) {
	idStr := c.Param("id")
	id, _ := strconv.ParseUint(idStr, 10, 32)

	var pm models.PromoMessage
	if err := db.DB.First(&pm, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Message not found"})
		return
	}

	if err := c.Request.ParseMultipartForm(10 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form data"})
		return
	}

	pm.Message = c.PostForm("message")
	pm.MsgType = c.PostForm("msg_type")
	pm.PollOptions = c.PostForm("poll_options")

	file, header, err := c.Request.FormFile("media")
	if err == nil {
		defer file.Close()
		fileBytes, _ := io.ReadAll(file)
		mimeType := header.Header.Get("Content-Type")

		if pm.MediaPath != "" {
			os.Remove(pm.MediaPath)
		}

		os.MkdirAll("media/promo", 0755)
		filename := fmt.Sprintf("%d_%d_%s", pm.AccountID, time.Now().UnixMilli(), header.Filename)
		pm.MediaPath = filepath.Join("media", "promo", filename)
		pm.MimeType = mimeType
		os.WriteFile(pm.MediaPath, fileBytes, 0644)
	}

	removeMedia := c.PostForm("remove_media")
	if removeMedia == "true" && pm.MediaPath != "" {
		os.Remove(pm.MediaPath)
		pm.MediaPath = ""
		pm.MimeType = ""
	}

	db.DB.Save(&pm)
	c.JSON(http.StatusOK, pm)
}

func getPromoSettings(c *gin.Context) {
	accountIDStr := c.Query("account_id")
	accountID, _ := strconv.ParseUint(accountIDStr, 10, 32)

	enabled := false
	sendsPerDay := 5

	var confEnabled models.AppConfig
	if err := db.DB.First(&confEnabled, "account_id = ? AND key = ?", uint(accountID), "promo_enabled").Error; err == nil {
		enabled = confEnabled.Value == "true"
	}

	var confSends models.AppConfig
	if err := db.DB.First(&confSends, "account_id = ? AND key = ?", uint(accountID), "promo_sends_per_day").Error; err == nil {
		if n, e := strconv.Atoi(confSends.Value); e == nil {
			sendsPerDay = n
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"enabled":        enabled,
		"sends_per_day":  sendsPerDay,
		"is_running":     promo.IsSchedulerRunning(uint(accountID)),
		"is_sending_now": promo.IsSending(uint(accountID)),
	})
}

type PromoSettingsReq struct {
	AccountID   uint `json:"account_id"`
	Enabled     bool `json:"enabled"`
	SendsPerDay int  `json:"sends_per_day"`
}

func setPromoSettings(c *gin.Context) {
	var req PromoSettingsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.SendsPerDay < 1 {
		req.SendsPerDay = 5
	}

	upsertAppConfig(req.AccountID, "promo_enabled", strconv.FormatBool(req.Enabled))
	upsertAppConfig(req.AccountID, "promo_sends_per_day", strconv.Itoa(req.SendsPerDay))

	promo.RestartAccountScheduler(req.AccountID)

	c.JSON(http.StatusOK, gin.H{"message": "Promo settings updated"})
}

type TestPromoReq struct {
	AccountID uint `json:"account_id"`
	MessageID uint `json:"message_id"`
}

func testPromoSend(c *gin.Context) {
	var req TestPromoReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var pm models.PromoMessage
	if err := db.DB.First(&pm, req.MessageID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Promo message not found"})
		return
	}

	if pm.AccountID != req.AccountID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Message does not belong to this account"})
		return
	}

	go promo.SendPromoToGroups(req.AccountID, pm)

	c.JSON(http.StatusOK, gin.H{"message": "Test promo send started. Check server logs."})
}

func upsertAppConfig(accountID uint, key, value string) {
	var conf models.AppConfig
	result := db.DB.First(&conf, "account_id = ? AND key = ?", accountID, key)
	if result.Error != nil {
		conf = models.AppConfig{AccountID: accountID, Key: key, Value: value}
		db.DB.Create(&conf)
	} else {
		conf.Value = value
		db.DB.Save(&conf)
	}
}
