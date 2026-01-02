package server

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
)

func StartServer() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Fallback for local dev
	}

	r := gin.Default()

	hubManager := NewHubManager()

	AddWebSocketRoutes(r, hubManager)

	log.Printf("Serving on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
