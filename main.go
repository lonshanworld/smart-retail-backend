package main

import (
	"app/config"
	"app/database"
	"app/routes"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file, using environment variables")
	}

	// Load configuration
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET is not set")
	}

	// Set up the application configuration
	config.AppConfig.JWTSecret = jwtSecret

	// Initialize database
	database.InitDB(databaseURL)
	defer database.CloseDB()

	app := fiber.New()

	// Add CORS middleware
	app.Use(cors.New())

	// Setup routes
	routes.SetupRoutes(app)

	// Start server
	log.Fatal(app.Listen(":3000"))
}
