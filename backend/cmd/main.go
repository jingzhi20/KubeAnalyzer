package main

import (
	"log"
	"os"

	"aiops-backend/internal/agent"
	"aiops-backend/internal/api"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r := api.SetupRouter()

	// Initialize Agent Hub for remote cluster management
	agent.NewHub()
	log.Println("Agent Hub initialized for remote cluster support")

	log.Printf("AIOps backend server starting on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
