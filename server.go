package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func usage() {
	fmt.Println("usage: helloserver [options]")
	flag.PrintDefaults()
}

var (
	greeting = flag.String("g", "Hello", "Greet with `greeting`")
	addr     = flag.String("addr", "localhost:8080", "address to serve")
)

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file, using environment variables directly")
	}

	// Parse flags.
	flag.Usage = usage
	flag.Parse()

	// Create a new Fiber app.
	app := fiber.New()

	// Connect to the database.
	log.Println("Attempting to connect to the database...")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatalf("Unable to parse database URL: %v\n", err)
	}
	config.ConnConfig.ConnectTimeout = 10 * time.Second // Add a 10-second timeout

	dbpool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		log.Fatalf("Unable to create new database pool: %v\n", err)
	}
	defer dbpool.Close()

	log.Println("Database connection pool created successfully.")

	// Register handlers.
	app.Get("/version", func(c *fiber.Ctx) error {
		info, ok := debug.ReadBuildInfo()
		if !ok {
			return c.Status(500).SendString("no build information available")
		}
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTML)
		return c.SendString("<pre>\n" + info.String() + "</pre>\n")
	})

	app.Get("/db", func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // Add a 5-second timeout for the ping
		defer cancel()

		log.Println("Pinging the database...")
		err := dbpool.Ping(ctx)
		if err != nil {
			log.Printf("Database ping failed: %v\n", err)
			return c.Status(500).SendString("Database ping failed: " + err.Error())
		}
		log.Println("Database ping successful.")
		return c.SendString("Database ping successful!")
	})

	app.Get("/:name?", func(c *fiber.Ctx) error {
		name := c.Params("name")
		if name == "" {
			name = "Gopher"
		}
		return c.SendString(fmt.Sprintf("%s, %s!", *greeting, name))
	})

	// Start the server.
	log.Printf("serving http://%s\n", *addr)
	log.Fatal(app.Listen(*addr))
}
