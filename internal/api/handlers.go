package api

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"juraganxl-notif/internal/db"
	"juraganxl-notif/internal/models"
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

	err = whatsapp.BroadcastCustomMessage(uint(accountID), msg, msgType, pollOptions, fileBytes, mimeType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Broadcast sent"})
}
