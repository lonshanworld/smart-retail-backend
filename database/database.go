package database

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB is a global variable to hold the database connection pool.
var DB *pgxpool.Pool

// Connect sets up the database connection pool.
func Connect(databaseURL string) {
	var err error
	DB, err = pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}

	// Optional: Check if the connection is actually working
	err = DB.Ping(context.Background())
	if err != nil {
		log.Fatalf("Database ping failed: %v\n", err)
	}

	log.Println("Successfully connected to the database")
}

// Close closes the database connection pool.
func Close() {
	if DB != nil {
		DB.Close()
		log.Println("Database connection pool closed")
	}
}
