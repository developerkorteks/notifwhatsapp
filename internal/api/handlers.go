package api

import (
	"net/http"

	"juraganxl-notif/internal/whatsapp"

	"github.com/gin-gonic/gin"
)

func RegisterHandlers(r *gin.Engine) {
	api := r.Group("/api")
	{
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

func getStatus(c *gin.Context) {
	if whatsapp.WAClient != nil && whatsapp.WAClient.IsConnected() && whatsapp.WAClient.IsLoggedIn() {
		c.JSON(http.StatusOK, gin.H{"status": "connected"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "disconnected"})
}

func generateQR(c *gin.Context) {
	qrChan, err := whatsapp.GenerateQR()
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
	whatsapp.Logout()
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

func getGroups(c *gin.Context) {
	groups := whatsapp.GetDBGroups()
	c.JSON(http.StatusOK, groups)
}

func syncGroups(c *gin.Context) {
	if err := whatsapp.SyncGroups(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Groups synced"})
}

type GroupSettingReq struct {
	JID            string `json:"jid"`
	IsStockActive  bool   `json:"is_stock_active"`
	IsCustomActive bool   `json:"is_custom_active"`
}

func updateGroupSettings(c *gin.Context) {
	var req GroupSettingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := whatsapp.UpdateGroupSettings(req.JID, req.IsStockActive, req.IsCustomActive); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Settings updated"})
}

func getChannels(c *gin.Context) {
	channels := whatsapp.GetDBChannels()
	c.JSON(http.StatusOK, channels)
}

func syncChannels(c *gin.Context) {
	if err := whatsapp.SyncChannels(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Channels synced"})
}

type ActiveChannelReq struct {
	JID string `json:"jid"`
}

func setActiveChannel(c *gin.Context) {
	var req ActiveChannelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := whatsapp.SetActiveChannel(req.JID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Active channel updated"})
}

type CustomBroadcastReq struct {
	Message string `json:"message"`
}

func sendCustomBroadcast(c *gin.Context) {
	var req CustomBroadcastReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Message cannot be empty"})
		return
	}

	// Will implement broadcost logic in whatsapp package soon
	err := whatsapp.BroadcastCustomMessage(req.Message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Broadcast sent"})
}
