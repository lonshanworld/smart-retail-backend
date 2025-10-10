package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/debug"

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
	dbpool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer dbpool.Close()

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
		err := dbpool.Ping(context.Background())
		if err != nil {
			return c.Status(500).SendString("Database ping failed: " + err.Error())
		}
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
