package config

import (
	"os"
	"strings"
)

// Config struct holds application configuration
// This is a simple way to make config accessible globally.
// A more advanced approach might use dependency injection.
type Config struct {
	JWTSecret        string
	LocalStorageOnly bool
}

// AppConfig holds the application-wide configuration
var AppConfig Config

func LoadBoolEnv(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "true", "1", "yes", "y":
		return true
	default:
		return false
	}
}
