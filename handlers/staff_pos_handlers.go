package handlers

import (
	"app/database"
	"app/models"
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

// HandleSearchProductsForStaff godoc
// @Summary Search for products in the staff member's assigned shop
// @Description Searches for products available for sale in the staff member's assigned shop.
// @Tags Staff POS
// @Accept  json
// @Produce  json
// @Security ApiKeyAuth
// @Param searchTerm query string false "Search term"
// @Success 200 {array} models.InventoryItem
// @Failure 401 {object} fiber.Map{message=string}
// @Failure 500 {object} fiber.Map{message=string}
// @Router /api/v1/staff/pos/products [get]
func HandleSearchProductsForStaff(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	userID := claims["userId"].(string)

	var assignedShopID string
	userQuery := `SELECT assigned_shop_id FROM users WHERE id = $1`
	if err := db.QueryRow(ctx, userQuery, userID).Scan(&assignedShopID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Assigned shop not found for this user"})
	}

	searchTerm := c.Query("searchTerm")

	query := `
        SELECT ii.id, ii.merchant_id, ii.name, ii.sku, ii.selling_price, ii.original_price, ii.created_at, ii.updated_at
        FROM inventory_items ii
        INNER JOIN shop_stock ss ON ii.id = ss.inventory_item_id
        WHERE ss.shop_id = $1
        AND (ii.name ILIKE $2 OR ii.sku ILIKE $2)
    `

	rows, err := db.Query(ctx, query, assignedShopID, "%"+searchTerm+"%")
	if err != nil {
		log.Printf("Error searching products: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to search products"})
	}
	defer rows.Close()

	var items []models.InventoryItem
	for rows.Next() {
		var item models.InventoryItem
		if err := rows.Scan(&item.ID, &item.MerchantID, &item.Name, &item.SKU, &item.SellingPrice, &item.OriginalPrice, &item.CreatedAt, &item.UpdatedAt); err != nil {
			log.Printf("Error scanning product item: %v", err)
			continue
		}
		items = append(items, item)
	}

	return c.JSON(fiber.Map{"status": "success", "data": items})
}

// HandleStaffCheckout godoc
// @Summary Process a new sale for the staff member's assigned shop
// @Description Processes a new sale (checkout) for the staff member's assigned shop.
// @Tags Staff POS
// @Accept  json
// @Produce  json
// @Security ApiKeyAuth
// @Param sale body models.StaffCheckoutRequest true "Sale data"
// @Success 201 {object} models.Sale
// @Failure 400 {object} fiber.Map{message=string}
// @Failure 401 {object} fiber.Map{message=string}
// @Failure 500 {object} fiber.Map{message=string}
// @Router /api/v1/staff/pos/checkout [post]
func HandleStaffCheckout(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	userID := claims["userId"].(string)

	var assignedShopID, merchantID string
	userQuery := `SELECT assigned_shop_id, merchant_id FROM users WHERE id = $1`
	if err := db.QueryRow(ctx, userQuery, userID).Scan(&assignedShopID, &merchantID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Assigned shop not found for this user"})
	}

	var req models.StaffCheckoutRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to start transaction"})
	}
	defer tx.Rollback(ctx)

	saleQuery := `
        INSERT INTO sales (shop_id, merchant_id, staff_id, total_amount, payment_type, payment_status)
        VALUES ($1, $2, $3, $4, $5, 'succeeded')
        RETURNING id, sale_date, created_at, updated_at
    `
	var sale models.Sale
	sale.ShopID = assignedShopID
	sale.MerchantID = merchantID
	sale.StaffID = &userID
	sale.TotalAmount = req.TotalAmount
	sale.PaymentType = req.PaymentType
	sale.PaymentStatus = "succeeded"

	err = tx.QueryRow(ctx, saleQuery, sale.ShopID, sale.MerchantID, sale.StaffID, sale.TotalAmount, sale.PaymentType).Scan(&sale.ID, &sale.SaleDate, &sale.CreatedAt, &sale.UpdatedAt)
	if err != nil {
		log.Printf("Error creating sale: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create sale"})
	}

	for _, item := range req.Items {
		saleItemQuery := `
            INSERT INTO sale_items (sale_id, inventory_item_id, quantity_sold, selling_price_at_sale)
            VALUES ($1, $2, $3, $4)
        `
		_, err := tx.Exec(ctx, saleItemQuery, sale.ID, item.ProductID, item.Quantity, item.SellingPriceAtSale)
		if err != nil {
			log.Printf("Error creating sale item: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create sale item"})
		}

		updateStockQuery := `
            UPDATE shop_stock
            SET quantity = quantity - $1
            WHERE shop_id = $2 AND inventory_item_id = $3
        `
		_, err = tx.Exec(ctx, updateStockQuery, item.Quantity, assignedShopID, item.ProductID)
		if err != nil {
			log.Printf("Error updating stock: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update stock"})
		}

		var currentQuantity int
		stockQuery := `SELECT quantity FROM shop_stock WHERE shop_id = $1 AND inventory_item_id = $2`
		err = tx.QueryRow(ctx, stockQuery, assignedShopID, item.ProductID).Scan(&currentQuantity)
		if err != nil {
			log.Printf("Error getting current stock quantity: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to get current stock quantity"})
		}

		stockMovementQuery := `
            INSERT INTO stock_movements (inventory_item_id, shop_id, user_id, movement_type, quantity_changed, new_quantity, reason)
            VALUES ($1, $2, $3, 'sale', $4, $5, 'Sale')
        `
		_, err = tx.Exec(ctx, stockMovementQuery, item.ProductID, assignedShopID, userID, -item.Quantity, currentQuantity)
		if err != nil {
			log.Printf("Error creating stock movement: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create stock movement"})
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to commit transaction"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "data": sale})
}
