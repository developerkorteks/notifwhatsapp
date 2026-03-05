package api

import (
	"log"

	"github.com/gin-gonic/gin"
)

func StartServer() {
	r := gin.Default()
	RegisterHandlers(r)
	
	log.Println("Starting Server on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
