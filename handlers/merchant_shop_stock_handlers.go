package handlers

import (
	"app/database"
	"app/models"
	"context"
	"fmt"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v4"
)

// ShopStockInRequest defines the expected body for a shop stock-in request.
type ShopStockInRequest struct {
	Items []models.StockInItem `json:"items"`
}

// HandleShopStockIn handles the bulk stock-in of items for a specific shop by a merchant.
func HandleShopStockIn(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	// Extract merchant ID from locals (set by middleware)
	merchantID, ok := c.Locals("userID").(string)
	if !ok || merchantID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "Unauthorized"})
	}

	// Get shopId from URL
	shopID := c.Params("shopId")
	if shopID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Shop ID is required"})
	}

	// Parse request body
	var req ShopStockInRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}

	if len(req.Items) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "At least one item is required for stock-in"})
	}

	// --- Transaction Begins ---
	tx, err := db.Begin(ctx)
	if err != nil {
		log.Printf("Failed to begin transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to start process"})
	}
	defer tx.Rollback(ctx)

	// 1. Verify the shop belongs to the merchant
	var dbMerchantID string
	shopQuery := "SELECT merchant_id FROM shops WHERE id = $1"
	if err := tx.QueryRow(ctx, shopQuery, shopID).Scan(&dbMerchantID); err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Shop not found"})
		}
		log.Printf("Error verifying shop ownership: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Could not verify shop details"})
	}
	if dbMerchantID != merchantID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "You do not have permission to stock in items for this shop"})
	}

	// 2. Process each item
	for _, item := range req.Items {
		if item.Quantity <= 0 {
			continue // Skip items with non-positive quantity
		}

		// Upsert shop_stock: Insert or update the quantity
		var newQuantity int
		stockUpsertQuery := `
            INSERT INTO shop_stock (shop_id, inventory_item_id, quantity)
            VALUES ($1, $2, $3)
            ON CONFLICT (shop_id, inventory_item_id)
            DO UPDATE SET quantity = shop_stock.quantity + EXCLUDED.quantity
            RETURNING quantity;
        `
		err := tx.QueryRow(ctx, stockUpsertQuery, shopID, item.ProductID, item.Quantity).Scan(&newQuantity)
		if err != nil {
			log.Printf("Error upserting stock for item %s: %v", item.ProductID, err)
			if err.Error() == `ERROR: insert or update on table "shop_stock" violates foreign key constraint "shop_stock_inventory_item_id_fkey" (SQLSTATE 23503)` {
				msg := fmt.Sprintf("Product with ID %s does not exist in the master inventory.", item.ProductID)
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": msg})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update stock quantity"})
		}

		// Log the stock movement
		movementQuery := `
            INSERT INTO stock_movements (inventory_item_id, shop_id, user_id, movement_type, quantity_changed, new_quantity, reason)
            VALUES ($1, $2, $3, 'stock_in', $4, $5, 'Merchant Stock-In');
        `
		_, err = tx.Exec(ctx, movementQuery, item.ProductID, shopID, merchantID, item.Quantity, newQuantity)
		if err != nil {
			log.Printf("Error logging stock movement for item %s: %v", item.ProductID, err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to log stock change"})
		}
	}

	// --- Commit Transaction ---
	if err := tx.Commit(ctx); err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to finalize stock-in process"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "message": "Stock-in successful"})
}
