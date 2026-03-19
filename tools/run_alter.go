package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	_ "github.com/lib/pq"
)

func main() {
	// Prefer DATABASE_URL if provided (common in .env / cloud settings)
	dbURL := os.Getenv("DATABASE_URL")
	var dsn string
	if dbURL != "" {
		// Ensure sslmode is present for cloud providers like Aiven
		if !strings.Contains(dbURL, "sslmode=") {
			// append sslmode=require preserving existing query params
			if strings.Contains(dbURL, "?") {
				dbURL = dbURL + "&sslmode=require"
			} else {
				dbURL = dbURL + "?sslmode=require"
			}
		}
		// Validate URL
		if _, err := url.Parse(dbURL); err != nil {
			log.Fatalf("invalid DATABASE_URL: %v", err)
		}
		dsn = dbURL
	} else {
		dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			envOr("PGHOST", "localhost"),
			envOr("PGPORT", "5432"),
			envOr("PGUSER", "postgres"),
			envOr("PGPASSWORD", ""),
			envOr("PGDATABASE", "postgres"),
			envOr("PGSSLMODE", "disable"),
		)
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	sqlStmt := `ALTER TABLE brands ADD COLUMN IF NOT EXISTS image_url TEXT;`
	if _, err := db.Exec(sqlStmt); err != nil {
		log.Fatalf("ALTER TABLE failed: %v", err)
	}

	fmt.Println("ALTER TABLE executed successfully.")
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
