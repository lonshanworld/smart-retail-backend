package handlers

import (
	"app/database"
	"app/middleware"
	"context"
	"fmt"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v4"
)

// MoveStockRequest defines the request body for moving stock between shops
type MoveStockRequest struct {
	ItemID     string `json:"itemId"`
	FromShopID string `json:"fromShopId"`
	ToShopID   string `json:"toShopId"`
	Quantity   int    `json:"quantity"`
}

// HandleMoveStock moves stock of an item from one shop to another
func HandleMoveStock(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	merchantID := claims.UserID
	userID := claims.UserID

	var req MoveStockRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid request body",
		})
	}

	// Validate request
	if req.ItemID == "" || req.FromShopID == "" || req.ToShopID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "itemId, fromShopId, and toShopId are required",
		})
	}

	if req.Quantity <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Quantity must be greater than 0",
		})
	}

	if req.FromShopID == req.ToShopID {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Source and destination shops must be different",
		})
	}

	// Start transaction
	tx, err := db.Begin(ctx)
	if err != nil {
		log.Printf("Failed to begin transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to start transaction",
		})
	}
	defer tx.Rollback(ctx)

	// Verify both shops belong to the merchant
	var fromShopMerchantID, toShopMerchantID string
	err = tx.QueryRow(ctx, "SELECT merchant_id FROM shops WHERE id = $1", req.FromShopID).Scan(&fromShopMerchantID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"status":  "error",
				"message": "Source shop not found",
			})
		}
		log.Printf("Error checking source shop: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Database error",
		})
	}

	err = tx.QueryRow(ctx, "SELECT merchant_id FROM shops WHERE id = $1", req.ToShopID).Scan(&toShopMerchantID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"status":  "error",
				"message": "Destination shop not found",
			})
		}
		log.Printf("Error checking destination shop: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Database error",
		})
	}

	// Verify ownership
	if fromShopMerchantID != merchantID || toShopMerchantID != merchantID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  "error",
			"message": "You don't have permission to move stock between these shops",
		})
	}

	// Verify the item belongs to the merchant
	var itemMerchantID string
	err = tx.QueryRow(ctx, "SELECT merchant_id FROM inventory_items WHERE id = $1", req.ItemID).Scan(&itemMerchantID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"status":  "error",
				"message": "Inventory item not found",
			})
		}
		log.Printf("Error checking inventory item: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Database error",
		})
	}

	if itemMerchantID != merchantID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  "error",
			"message": "You don't have permission to move this item",
		})
	}

	// Decrement stock from source shop
	var newFromQuantity int
	err = tx.QueryRow(ctx,
		`UPDATE shop_stock 
		 SET quantity = quantity - $1 
		 WHERE shop_id = $2 AND inventory_item_id = $3 AND quantity >= $1
		 RETURNING quantity`,
		req.Quantity, req.FromShopID, req.ItemID,
	).Scan(&newFromQuantity)

	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"status":  "error",
				"message": "Insufficient stock in source shop",
			})
		}
		log.Printf("Error decrementing stock from source shop: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to update source shop stock",
		})
	}

	// Record stock movement for source shop (outgoing)
	_, err = tx.Exec(ctx,
		`INSERT INTO stock_movements (inventory_item_id, shop_id, user_id, movement_type, quantity_changed, new_quantity, reason)
		 VALUES ($1, $2, $3, 'transfer_out', $4, $5, $6)`,
		req.ItemID, req.FromShopID, userID, -req.Quantity, newFromQuantity,
		fmt.Sprintf("Transferred %d units to another shop", req.Quantity),
	)
	if err != nil {
		log.Printf("Error recording source shop movement: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to record stock movement",
		})
	}

	// Increment stock in destination shop (or create if doesn't exist)
	var newToQuantity int
	err = tx.QueryRow(ctx,
		`INSERT INTO shop_stock (shop_id, inventory_item_id, quantity, last_stocked_in_at)
		 VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
		 ON CONFLICT (shop_id, inventory_item_id) 
		 DO UPDATE SET quantity = shop_stock.quantity + $3, last_stocked_in_at = CURRENT_TIMESTAMP
		 RETURNING quantity`,
		req.ToShopID, req.ItemID, req.Quantity,
	).Scan(&newToQuantity)

	if err != nil {
		log.Printf("Error incrementing stock in destination shop: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to update destination shop stock",
		})
	}

	// Record stock movement for destination shop (incoming)
	_, err = tx.Exec(ctx,
		`INSERT INTO stock_movements (inventory_item_id, shop_id, user_id, movement_type, quantity_changed, new_quantity, reason)
		 VALUES ($1, $2, $3, 'transfer_in', $4, $5, $6)`,
		req.ItemID, req.ToShopID, userID, req.Quantity, newToQuantity,
		fmt.Sprintf("Received %d units from another shop", req.Quantity),
	)
	if err != nil {
		log.Printf("Error recording destination shop movement: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to record stock movement",
		})
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to complete stock transfer",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":  "success",
		"message": "Stock moved successfully",
		"data": fiber.Map{
			"fromShopNewQuantity": newFromQuantity,
			"toShopNewQuantity":   newToQuantity,
		},
	})
}
