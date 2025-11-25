package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// GenerateInvoiceNumber generates a unique invoice number in the format INV-YYYY-NNNN
// where YYYY is the current year and NNNN is a sequential number.
func GenerateInvoiceNumber(ctx context.Context, db interface{}) (string, error) {
	year := time.Now().Year()
	prefix := fmt.Sprintf("INV-%d-", year)

	// Query to find the latest invoice number for this year
	query := `
		SELECT invoice_number 
		FROM invoices 
		WHERE invoice_number LIKE $1 
		ORDER BY invoice_number DESC 
		LIMIT 1
	`
	pattern := fmt.Sprintf("INV-%d-%%", year)

	var lastInvoiceNumber string
	var err error

	// Handle both pgxpool.Pool and pgx.Tx types
	switch v := db.(type) {
	case *pgxpool.Pool:
		err = v.QueryRow(ctx, query, pattern).Scan(&lastInvoiceNumber)
	case pgx.Tx:
		err = v.QueryRow(ctx, query, pattern).Scan(&lastInvoiceNumber)
	default:
		return "", fmt.Errorf("unsupported database type")
	}

	// If no invoice exists for this year, start at 0001
	if err != nil {
		if err.Error() == "no rows in result set" {
			return fmt.Sprintf("%s%04d", prefix, 1), nil
		}
		return "", fmt.Errorf("failed to query last invoice number: %w", err)
	}

	// Extract the sequential number from the last invoice
	var lastSeq int
	_, err = fmt.Sscanf(lastInvoiceNumber, prefix+"%d", &lastSeq)
	if err != nil {
		// If parsing fails, start fresh
		return fmt.Sprintf("%s%04d", prefix, 1), nil
	}

	// Increment and return
	newSeq := lastSeq + 1
	return fmt.Sprintf("%s%04d", prefix, newSeq), nil
}
