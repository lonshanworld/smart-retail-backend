package handlers

import (
	"app/database"
	"app/models"
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

// HandleListMerchantShops lists all shops belonging to the authenticated merchant.
func HandleListMerchantShops(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantID := claims["userId"].(string)

	query := `SELECT id, name, address, phone, is_active, is_primary, created_at, updated_at FROM shops WHERE merchant_id = $1`
	rows, err := db.Query(ctx, query, merchantID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve shops"})
	}
	defer rows.Close()

	shops := make([]models.Shop, 0)
	for rows.Next() {
		var shop models.Shop
		if err := rows.Scan(&shop.ID, &shop.Name, &shop.Address, &shop.Phone, &shop.IsActive, &shop.IsPrimary, &shop.CreatedAt, &shop.UpdatedAt); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to scan shop data"})
		}
		shop.MerchantID = merchantID // Set the merchant ID from the token
		shops = append(shops, shop)
	}

	return c.JSON(fiber.Map{"status": "success", "data": shops})
}

// HandleUpdateMerchantShop updates a shop belonging to the authenticated merchant.
func HandleUpdateMerchantShop(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantID := claims["userId"].(string)

	shopID := c.Params("shopId")
	var req models.Shop
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}

	query := `
        UPDATE shops 
        SET name = $1, address = $2, phone = $3 
        WHERE id = $4 AND merchant_id = $5
        RETURNING id, name, address, phone, is_active, is_primary, created_at, updated_at
    `
	var shop models.Shop
	err := db.QueryRow(ctx, query, req.Name, req.Address, req.Phone, shopID, merchantID).Scan(
		&shop.ID, &shop.Name, &shop.Address, &shop.Phone, &shop.IsActive, &shop.IsPrimary, &shop.CreatedAt, &shop.UpdatedAt,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update shop"})
	}
	shop.MerchantID = merchantID

	return c.JSON(fiber.Map{"status": "success", "data": shop})
}

// HandleDeleteMerchantShop deletes a shop belonging to the authenticated merchant.
func HandleDeleteMerchantShop(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantID := claims["userId"].(string)

	shopID := c.Params("shopId")

	query := "DELETE FROM shops WHERE id = $1 AND merchant_id = $2"
	_, err := db.Exec(ctx, query, shopID, merchantID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to delete shop"})
	}

	return c.Status(204).JSON(fiber.Map{"status": "success"})
}

// HandleListProductsForShop fetches all inventory items associated with a specific shop.
func HandleListProductsForShop(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantID := claims["userId"].(string)

	shopID := c.Params("shopId")

	// Verify the shop belongs to the merchant
	var count int
	checkQuery := "SELECT COUNT(*) FROM shops WHERE id = $1 AND merchant_id = $2"
	if err := db.QueryRow(ctx, checkQuery, shopID, merchantID).Scan(&count); err != nil || count == 0 {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Shop not found or access denied"})
	}

	query := `
        SELECT ii.id, ii.merchant_id, ii.name, ii.description, ii.sku, 
               ii.selling_price, ii.original_price, ii.low_stock_threshold, 
               ii.category, ii.supplier_id, ii.is_archived, ii.created_at, ii.updated_at, ss.quantity
        FROM inventory_items ii
        JOIN shop_stock ss ON ii.id = ss.inventory_item_id
        WHERE ss.shop_id = $1
    `
	rows, err := db.Query(ctx, query, shopID)
	if err != nil {
		log.Printf("Error fetching products for shop: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve products"})
	}
	defer rows.Close()

	products := make([]models.InventoryItemWithQuantity, 0)
	for rows.Next() {
		var p models.InventoryItemWithQuantity
		if err := rows.Scan(&p.ID, &p.MerchantID, &p.Name, &p.Description, &p.SKU, &p.SellingPrice, &p.OriginalPrice, &p.LowStockThreshold, &p.Category, &p.SupplierID, &p.IsArchived, &p.CreatedAt, &p.UpdatedAt, &p.Quantity); err != nil {
			log.Printf("Error scanning product data: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to process product data"})
		}
		products = append(products, p)
	}

	return c.JSON(fiber.Map{"status": "success", "data": products})
}

// HandleSetPrimaryShop sets a shop as the primary shop for the merchant.
func HandleSetPrimaryShop(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantID := claims["userId"].(string)

	shopID := c.Params("shopId")

	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to start transaction"})
	}
	defer tx.Rollback(ctx)

	// Reset all other shops for this merchant to not be primary
	resetQuery := "UPDATE shops SET is_primary = FALSE WHERE merchant_id = $1"
	if _, err := tx.Exec(ctx, resetQuery, merchantID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to reset primary shops"})
	}

	// Set the specified shop as primary
	setQuery := `
        UPDATE shops 
        SET is_primary = TRUE 
        WHERE id = $1 AND merchant_id = $2
        RETURNING id, name, address, phone, is_active, is_primary, created_at, updated_at
    `
	var shop models.Shop
	err = tx.QueryRow(ctx, setQuery, shopID, merchantID).Scan(
		&shop.ID, &shop.Name, &shop.Address, &shop.Phone, &shop.IsActive, &shop.IsPrimary, &shop.CreatedAt, &shop.UpdatedAt,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to set primary shop"})
	}
	shop.MerchantID = merchantID

	if err := tx.Commit(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to commit transaction"})
	}

	return c.JSON(fiber.Map{"status": "success", "data": shop})
}
