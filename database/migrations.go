package database

import (
	"context"
)

// EnsureSyncSchema applies lightweight idempotency-related schema changes.
// It is safe to run on every startup.
func EnsureSyncSchema(ctx context.Context) error {
	db := GetDB()
	if db == nil {
		return nil
	}

	statements := []string{
		`ALTER TABLE sales ADD COLUMN IF NOT EXISTS client_sale_id TEXT`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_sales_client_sale_id ON sales (client_sale_id)`,
		`CREATE TABLE IF NOT EXISTS inventory_operations (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			client_operation_id TEXT UNIQUE NOT NULL,
			operation_type VARCHAR(80) NOT NULL,
			actor_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
			shop_id UUID REFERENCES shops(id) ON DELETE SET NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, statement := range statements {
		if _, err := db.Exec(ctx, statement); err != nil {
			return err
		}
	}

	return nil
}
