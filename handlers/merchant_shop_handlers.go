package handlers

import (
	"app/database"
	"app/middleware"
	"app/models"
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func HandleListMerchantShops(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	merchantID := claims.UserID

	page, _ := strconv.Atoi(c.Query("page", "1"))
	size, _ := strconv.Atoi(c.Query("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	where := " WHERE s.merchant_id=$1"
	args := []interface{}{merchantID}
	if v := c.Query("search"); v != "" {
		where += " AND (s.name ILIKE $2 OR s.address ILIKE $2 OR s.phone ILIKE $2)"
		args = append(args, "%"+v+"%")
	}
	if v := c.Query("isActive"); v != "" {
		where += fmt.Sprintf(" AND s.is_active=$%d", len(args)+1)
		args = append(args, v == "true")
	}
	var total int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM shops s"+where, args...).Scan(&total); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to count shops"})
	}
	query := `
		SELECT s.id, s.name, s.address, s.phone, s.tax_rate, s.is_active, s.is_primary,
		       COALESCE(ps.delivery_charge, 0), s.created_at, s.updated_at
		FROM shops s
		LEFT JOIN payment_settings ps ON ps.shop_id = s.id
		` + where + fmt.Sprintf(" ORDER BY s.created_at DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, size, (page-1)*size)

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve shops"})
	}
	defer rows.Close()

	shops := make([]models.Shop, 0)
	for rows.Next() {
		var shop models.Shop
		if err := rows.Scan(&shop.ID, &shop.Name, &shop.Address, &shop.Phone, &shop.TaxRate, &shop.IsActive, &shop.IsPrimary, &shop.DeliveryCharge, &shop.CreatedAt, &shop.UpdatedAt); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to scan shop data"})
		}
		shop.MerchantID = merchantID
		shops = append(shops, shop)
	}

	return c.JSON(fiber.Map{"status": "success", "data": shops, "pagination": fiber.Map{"totalItems": total, "totalPages": (total + size - 1) / size, "currentPage": page, "pageSize": size}})
}

// HandleUpdateMerchantShop updates a shop belonging to the authenticated merchant.
func HandleUpdateMerchantShop(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	merchantID := claims.UserID

	shopID := c.Params("shopId")
	clientOperationID := c.Get("X-Client-Operation-Id")
	if clientOperationID == "" {
		clientOperationID = c.Query("clientOperationId")
	}
	if clientOperationID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "clientOperationId is required"})
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to start transaction"})
	}
	defer tx.Rollback(ctx)

	claimed, err := claimInventoryOperation(ctx, tx, clientOperationID, "merchant_update_shop", merchantID, &shopID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to claim operation"})
	}
	if !claimed {
		return c.Status(fiber.StatusNoContent).JSON(fiber.Map{"status": "success"})
	}

	var req models.Shop
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}

	query := `
		UPDATE shops
		SET name = $1, address = $2, phone = $3, tax_rate = $4
		WHERE id = $5 AND merchant_id = $6
		RETURNING id, name, address, phone, tax_rate, is_active, is_primary, created_at, updated_at
	`

	var shop models.Shop
	err = tx.QueryRow(ctx, query, req.Name, req.Address, req.Phone, req.TaxRate, shopID, merchantID).Scan(
		&shop.ID, &shop.Name, &shop.Address, &shop.Phone, &shop.TaxRate, &shop.IsActive, &shop.IsPrimary, &shop.CreatedAt, &shop.UpdatedAt,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update shop"})
	}
	shop.MerchantID = merchantID

	if err := tx.Commit(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to commit transaction"})
	}

	return c.JSON(fiber.Map{"status": "success", "data": shop})
}

// HandleDeleteMerchantShop deletes a shop belonging to the authenticated merchant.
func HandleDeleteMerchantShop(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	merchantID := claims.UserID

	shopID := c.Params("shopId")

	clientOperationID := c.Get("X-Client-Operation-Id")
	if clientOperationID == "" {
		clientOperationID = c.Query("clientOperationId")
	}
	if clientOperationID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "clientOperationId is required"})
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to start transaction"})
	}
	defer tx.Rollback(ctx)

	claimed, err := claimInventoryOperation(ctx, tx, clientOperationID, "merchant_delete_shop", merchantID, &shopID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to claim operation"})
	}
	if !claimed {
		return c.JSON(fiber.Map{"status": "success", "data": fiber.Map{"id": shopID}})
	}

	query := "DELETE FROM shops WHERE id = $1 AND merchant_id = $2"
	if _, err := tx.Exec(ctx, query, shopID, merchantID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to delete shop"})
	}

	if err := tx.Commit(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to commit transaction"})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// HandleListProductsForShop fetches all inventory items associated with a specific shop.
func HandleListProductsForShop(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	merchantID := claims.UserID

	shopID := c.Params("shopId")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	size, _ := strconv.Atoi(c.Query("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	// Verify the shop belongs to the merchant
	var count int
	checkQuery := "SELECT COUNT(*) FROM shops WHERE id = $1 AND merchant_id = $2"
	if err := db.QueryRow(ctx, checkQuery, shopID, merchantID).Scan(&count); err != nil || count == 0 {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Shop not found or access denied"})
	}
	where := " WHERE ii.shop_id = $1"
	args := []interface{}{shopID}
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		where += " AND (si.name ILIKE $2 OR si.sku ILIKE $2)"
		args = append(args, "%"+search+"%")
	}
	if categoryID := strings.TrimSpace(c.Query("categoryId")); categoryID != "" {
		where += " AND EXISTS (SELECT 1 FROM product_categories pc WHERE pc.product_id=si.product_id AND pc.category_id=$" + strconv.Itoa(len(args)+1) + ")"
		args = append(args, categoryID)
	}
	if brandID := strings.TrimSpace(c.Query("brandId")); brandID != "" {
		where += " AND p.brand_id=$" + strconv.Itoa(len(args)+1)
		args = append(args, brandID)
	}
	if c.Query("lowStock") == "true" {
		where += " AND ii.low_stock_threshold IS NOT NULL AND ii.quantity_on_hand <= ii.low_stock_threshold"
	}

	query := `
		SELECT ii.stock_item_id, ii.merchant_id, si.name, p.description, si.sku,
			COALESCE(pp.selling_price,0), COALESCE(pp.cost_price,0), ii.low_stock_threshold,
			NULL, NULL, NOT p.is_active, si.created_at, si.updated_at, ii.quantity_on_hand
		FROM inventory_items ii
		JOIN stock_items si ON si.id=ii.stock_item_id
		JOIN products p ON p.id=si.product_id
		LEFT JOIN LATERAL (SELECT selling_price,cost_price FROM product_prices WHERE product_id=si.product_id AND shop_id IS NULL AND price_type='RETAIL' ORDER BY created_at DESC LIMIT 1) pp ON TRUE
		` + where + `
	`
	query += " ORDER BY si.name"
	var total int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM inventory_items ii JOIN stock_items si ON si.id=ii.stock_item_id JOIN products p ON p.id=si.product_id"+where, args...).Scan(&total); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to count products"})
	}
	query += " LIMIT $" + strconv.Itoa(len(args)+1) + " OFFSET $" + strconv.Itoa(len(args)+2)
	args = append(args, size, (page-1)*size)
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		log.Printf("Error fetching products for shop: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve products"})
	}
	defer rows.Close()

	products := make([]models.InventoryItemWithQuantity, 0)
	for rows.Next() {
		var p models.InventoryItemWithQuantity
		var description, sku, category, supplierID *string
		var originalPrice *float64
		var lowStockThreshold *int

		err := rows.Scan(
			&p.ID,
			&p.MerchantID,
			&p.Name,
			&description,
			&sku,
			&p.SellingPrice,
			&originalPrice,
			&lowStockThreshold,
			&category,
			&supplierID,
			&p.IsArchived,
			&p.CreatedAt,
			&p.UpdatedAt,
			&p.Quantity,
		)
		if err != nil {
			log.Printf("Error scanning product data: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to process product data"})
		}

		// Assign nullable fields
		p.Description = description
		p.SKU = sku
		p.OriginalPrice = originalPrice
		p.LowStockThreshold = lowStockThreshold
		p.Category = category
		p.SupplierID = supplierID

		products = append(products, p)
	}

	return c.JSON(fiber.Map{"status": "success", "data": products, "pagination": fiber.Map{"totalItems": total, "totalPages": (total + size - 1) / size, "currentPage": page, "pageSize": size}})
}

// HandleSetPrimaryShop sets a shop as the primary shop for the merchant.
func HandleSetPrimaryShop(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	merchantID := claims.UserID

	shopID := c.Params("shopId")

	clientOperationID := c.Get("X-Client-Operation-Id")
	if clientOperationID == "" {
		clientOperationID = c.Query("clientOperationId")
	}
	if clientOperationID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "clientOperationId is required"})
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to start transaction"})
	}
	defer tx.Rollback(ctx)
	claimed, err := claimInventoryOperation(ctx, tx, clientOperationID, "merchant_set_primary_shop", merchantID, &shopID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to claim operation"})
	}
	if !claimed {
		return c.Status(fiber.StatusNoContent).JSON(fiber.Map{"status": "success"})
	}

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
		RETURNING id, name, address, phone, tax_rate, is_active, is_primary, created_at, updated_at
    `
	var shop models.Shop
	err = tx.QueryRow(ctx, setQuery, shopID, merchantID).Scan(
		&shop.ID, &shop.Name, &shop.Address, &shop.Phone, &shop.TaxRate, &shop.IsActive, &shop.IsPrimary, &shop.CreatedAt, &shop.UpdatedAt,
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

// HandleCheckDeleteMerchantShop performs a preflight check to see if a shop can be safely deleted.
// Returns { deletable: bool, blockers: {resourceName: count, ...} }
func HandleCheckDeleteMerchantShop(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	merchantID := claims.UserID

	shopID := c.Params("shopId")

	// Verify shop belongs to merchant
	var exists int
	checkQuery := "SELECT COUNT(*) FROM shops WHERE id = $1 AND merchant_id = $2"
	if err := db.QueryRow(ctx, checkQuery, shopID, merchantID).Scan(&exists); err != nil || exists == 0 {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Shop not found or access denied"})
	}

	blockers := map[string]int{}

	// Count sales referencing this shop
	var cnt int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM sales WHERE shop_id = $1", shopID).Scan(&cnt); err == nil && cnt > 0 {
		blockers["sales"] = cnt
	}

	// Count shop_stock rows
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM inventory_items WHERE shop_id = $1", shopID).Scan(&cnt); err == nil && cnt > 0 {
		blockers["inventory_items"] = cnt
	}

	// Count stock_movements rows
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM inventory_movements WHERE shop_id = $1", shopID).Scan(&cnt); err == nil && cnt > 0 {
		blockers["inventory_movements"] = cnt
	}

	// Count shop_customers
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM shop_customers WHERE shop_id = $1", shopID).Scan(&cnt); err == nil && cnt > 0 {
		blockers["shop_customers"] = cnt
	}

	// Count promotions referencing this shop
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM promotions WHERE shop_id = $1", shopID).Scan(&cnt); err == nil && cnt > 0 {
		blockers["promotions"] = cnt
	}

	// Count users assigned to this shop
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE assigned_shop_id = $1", shopID).Scan(&cnt); err == nil && cnt > 0 {
		blockers["assigned_users"] = cnt
	}

	deletable := len(blockers) == 0

	return c.JSON(fiber.Map{"status": "success", "data": fiber.Map{"deletable": deletable, "blockers": blockers}})
}
