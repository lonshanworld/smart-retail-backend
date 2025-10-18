package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config holds the application configuration.
// We can add more fields here as needed.
type Config struct {
	DatabaseURL string
	JWTSecret   string
}

// LoadConfig loads the configuration from a .env file and environment variables.
func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, reading from environment variables")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET is not set")
	}

	return &Config{
		DatabaseURL: databaseURL,
		JWTSecret:   jwtSecret,
	}
}
