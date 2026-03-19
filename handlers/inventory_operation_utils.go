package handlers

import (
	"context"

	"github.com/jackc/pgx/v4"
)

type inventoryOperationQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

// claimInventoryOperation records a client-generated inventory operation ID inside the current transaction.
// It returns false when the same operation was already processed and should be treated as an idempotent retry.
func claimInventoryOperation(ctx context.Context, tx inventoryOperationQuerier, clientOperationID, operationType, actorID string, shopID *string) (bool, error) {
	var insertedID string
	if shopID != nil {
		if err := tx.QueryRow(ctx, `
			INSERT INTO inventory_operations (client_operation_id, operation_type, actor_id, shop_id)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (client_operation_id) DO NOTHING
			RETURNING id
		`, clientOperationID, operationType, actorID, *shopID).Scan(&insertedID); err != nil {
			if err == pgx.ErrNoRows {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}

	if err := tx.QueryRow(ctx, `
		INSERT INTO inventory_operations (client_operation_id, operation_type, actor_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (client_operation_id) DO NOTHING
		RETURNING id
	`, clientOperationID, operationType, actorID).Scan(&insertedID); err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
