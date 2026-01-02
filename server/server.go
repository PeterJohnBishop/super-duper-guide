package server

import (
	"log"

	"github.com/gin-gonic/gin"
)

func StartServer() {
	r := gin.Default()

	hubManager := NewHubManager()

	AddWebSocketRoutes(r, hubManager)

	log.Println("Your localhost:8080 is serving Gin!")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
