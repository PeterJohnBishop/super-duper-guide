package main

import (
	"log"
	"super-duper-guide/server"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Print("No .env file found")
	}
	server.StartServer()
}
