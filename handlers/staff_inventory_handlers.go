package handlers

import (
	"app/database"
	"app/models"
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

// HandleGetShopInventory godoc
// @Summary Get shop inventory
// @Description Fetches a list of all inventory items for the staff's assigned shop.
// @Tags Shop
// @Accept  json
// @Produce  json
// @Security ApiKeyAuth
// @Success 200 {array} models.ShopInventoryItem
// @Failure 401 {object} fiber.Map{message=string}
// @Failure 404 {object} fiber.Map{message=string}
// @Failure 500 {object} fiber.Map{message=string}
// @Router /api/v1/shop/inventory [get]
func HandleGetShopInventory(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	userID := claims["userId"].(string)

	var assignedShopID string
	userQuery := `SELECT assigned_shop_id FROM users WHERE id = $1`
	if err := db.QueryRow(ctx, userQuery, userID).Scan(&assignedShopID); err != nil {
		log.Printf("Error fetching assigned shop ID for user %s: %v", userID, err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Assigned shop not found for this user"})
	}

	query := `
        SELECT
            ss.id, 
            ii.id as product_id,
            ii.name,
            COALESCE(ii.sku, ''),
            ss.quantity,
            ii.selling_price
        FROM 
            shop_stock ss
        JOIN 
            inventory_items ii ON ss.inventory_item_id = ii.id
        WHERE 
            ss.shop_id = $1
    `
	rows, err := db.Query(ctx, query, assignedShopID)
	if err != nil {
		log.Printf("Error querying shop inventory: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve shop inventory"})
	}
	defer rows.Close()

	var items []models.ShopInventoryItem
	for rows.Next() {
		var item models.ShopInventoryItem
		if err := rows.Scan(&item.ID, &item.ProductID, &item.Name, &item.SKU, &item.Quantity, &item.SellingPrice); err != nil {
			log.Printf("Error scanning shop inventory item: %v", err)
			continue // Or handle more gracefully
		}
		items = append(items, item)
	}

	return c.JSON(fiber.Map{"status": "success", "data": items})
}

// HandleStockIn godoc
// @Summary Stock in items
// @Description Updates the quantities of items in the shop's inventory (Stock In).
// @Tags Shop
// @Accept  json
// @Produce  json
// @Security ApiKeyAuth
// @Param   items  body  models.StockInRequest  true  "Stock In Request"
// @Success 200 {object} fiber.Map{message=string}
// @Failure 400 {object} fiber.Map{message=string}
// @Failure 401 {object} fiber.Map{message=string}
// @Failure 500 {object} fiber.Map{message=string}
// @Router /api/v1/shop/inventory/stock-in [post]
func HandleStockIn(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	userID := claims["userId"].(string)

	var req models.StockInRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}

	var assignedShopID string
	userQuery := `SELECT assigned_shop_id FROM users WHERE id = $1`
	if err := db.QueryRow(ctx, userQuery, userID).Scan(&assignedShopID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Assigned shop not found for this user"})
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to start transaction"})
	}
	defer tx.Rollback(ctx)

	for _, item := range req.Items {
		// Get current quantity
		var currentQuantity int
		stockQuery := `SELECT quantity FROM shop_stock WHERE shop_id = $1 AND inventory_item_id = $2`
		err := tx.QueryRow(ctx, stockQuery, assignedShopID, item.ProductID).Scan(&currentQuantity)
		if err != nil {
			log.Printf("Error getting current stock for product %s: %v", item.ProductID, err)
			continue
		}

		newQuantity := currentQuantity + item.Quantity

		// Update shop_stock
		updateQuery := `UPDATE shop_stock SET quantity = $1, last_stocked_in_at = NOW() WHERE shop_id = $2 AND inventory_item_id = $3`
		_, err = tx.Exec(ctx, updateQuery, newQuantity, assignedShopID, item.ProductID)
		if err != nil {
			log.Printf("Error updating stock for product %s: %v", item.ProductID, err)
			continue
		}

		// Insert into stock_movements
		movementQuery := `
            INSERT INTO stock_movements (inventory_item_id, shop_id, user_id, movement_type, quantity_changed, new_quantity, reason)
            VALUES ($1, $2, $3, 'stock_in', $4, $5, 'Stock received')
        `
		_, err = tx.Exec(ctx, movementQuery, item.ProductID, assignedShopID, userID, item.Quantity, newQuantity)
		if err != nil {
			log.Printf("Error inserting stock movement for product %s: %v", item.ProductID, err)
			continue
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to commit transaction"})
	}

	return c.JSON(fiber.Map{"status": "success", "message": "Stock updated successfully"})
}
