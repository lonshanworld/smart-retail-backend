package main

import (
	"flag"
	"log"

	"app/config"
	"app/database"
	"app/handlers"
	"app/middleware"
	"app/routes"

	"github.com/gofiber/fiber/v2"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Set JWT secret for middleware and handlers
	middleware.JWTSecret = []byte(cfg.JWTSecret)
	handlers.JWTSecret = []byte(cfg.JWTSecret)

	// Connect to the database
	database.Connect(cfg.DatabaseURL)
	defer database.Close()

	// Create a new Fiber app
	app := fiber.New()

	// Setup routes
	routes.SetupRoutes(app)

	// Start the server
	addr := flag.String("addr", ":8080", "address to serve")
	flag.Parse()

	log.Printf("Server is listening on port %s", *addr)
	log.Fatal(app.Listen(*addr))
}
