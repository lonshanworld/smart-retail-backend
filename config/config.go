package config

// Config struct holds application configuration
// This is a simple way to make config accessible globally.
// A more advanced approach might use dependency injection.
type Config struct {
	JWTSecret string
}

// AppConfig holds the application-wide configuration
var AppConfig Config
