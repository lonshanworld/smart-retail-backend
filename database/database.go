package database

import (
	"context"
	"log"

	"github.com/jackc/pgx/v4/pgxpool"
)

var dbPool *pgxpool.Pool

// InitDB initializes the database connection pool.
func InitDB(databaseURL string) {
	var err error
	if databaseURL == "" {
		log.Fatal("databaseURL is not provided")
	}

	dbPool, err = pgxpool.Connect(context.Background(), databaseURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}

	log.Println("Database connection established")
}

// GetDB returns the database connection pool.
func GetDB() *pgxpool.Pool {
	return dbPool
}

// CloseDB closes the database connection pool.
func CloseDB() {
	if dbPool != nil {
		dbPool.Close()
		log.Println("Database connection closed")
	}
}
