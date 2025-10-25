package handlers

import (
	"app/database"
	"app/models"
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// getShopIDFromStaffID retrieves the assigned shop ID for a given staff ID.
func getShopIDFromStaffID(ctx context.Context, db *pgxpool.Pool, staffID string) (string, error) {
	var shopID string
	query := "SELECT assigned_shop_id FROM users WHERE id = $1 AND role = 'staff'"
	err := db.QueryRow(ctx, query, staffID).Scan(&shopID)
	if err != nil {
		return "", err
	}
	return shopID, nil
}

// HandleGetShopItems retrieves all inventory items for the logged-in staff's assigned shop.
func HandleGetShopItems(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	staffID := claims["userId"].(string)

	shopID, err := getShopIDFromStaffID(ctx, db, staffID)
	if err != nil {
		log.Printf("Error finding shop for staff ID %s: %v", staffID, err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Could not find an assigned shop for this staff member."})
	}

	query := `
        SELECT
            ss.inventory_item_id,
            ii.merchant_id,
            ii.name,
            ii.sku,
            ii.selling_price,
            ii.original_price,
            ss.quantity,
            ss.shop_id,
            ii.created_at,
            ii.updated_at
        FROM shop_stock ss
        JOIN inventory_items ii ON ss.inventory_item_id = ii.id
        WHERE ss.shop_id = $1
        ORDER BY ii.name ASC
    `

	rows, err := db.Query(ctx, query, shopID)
	if err != nil {
		log.Printf("Error querying shop items for shop %s: %v", shopID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve shop inventory."})
	}
	defer rows.Close()

	var items []models.InventoryItem
	for rows.Next() {
		var item models.InventoryItem
		var stock models.ShopStock
		err := rows.Scan(
			&item.ID,
			&item.MerchantID,
			&item.Name,
			&item.SKU,
			&item.SellingPrice,
			&item.OriginalPrice,
			&stock.Quantity,
			&stock.ShopID,
			&item.CreatedAt,
			&item.UpdatedAt,
		)
		if err != nil {
			log.Printf("Error scanning shop item row: %v", err)
			continue // Skip problematic rows
		}
		item.Stock = &stock
		items = append(items, item)
	}

	return c.JSON(items)
}

type UpdateStockRequest struct {
	Quantity int `json:"quantity"`
}

// HandleUpdateShopItemStock updates the stock quantity for a specific item in the staff's shop.
func HandleUpdateShopItemStock(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	staffID := claims["userId"].(string)

	shopID, err := getShopIDFromStaffID(ctx, db, staffID)
	if err != nil {
		log.Printf("Error finding shop for staff ID %s: %v", staffID, err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Could not find an assigned shop for this staff member."})
	}

	itemID := c.Params("itemId")
	var req UpdateStockRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body."})
	}

	if req.Quantity < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Quantity cannot be negative."})
	}

	// For now, we are doing a direct update.
	// A more robust solution would be to use the stock adjustment logic that logs the movement.
	query := "UPDATE shop_stock SET quantity = $1, updated_at = NOW() WHERE inventory_item_id = $2 AND shop_id = $3"

	cmdTag, err := db.Exec(ctx, query, req.Quantity, itemID, shopID)
	if err != nil {
		log.Printf("Error updating stock for item %s in shop %s: %v", itemID, shopID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update stock quantity."})
	}

	if cmdTag.RowsAffected() == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Item not found in this shop's inventory."})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "message": "Stock quantity updated successfully."})
}
