package handlers

import (
	"app/database"
	"app/middleware"
	"app/models"
	"app/utils"
	"context"
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v4"
)

func hydrateNestedCatalogFields(
	item *models.InventoryItem,
	categoryID sql.NullString,
	categoryName sql.NullString,
	categoryDescription sql.NullString,
	subcategoryID sql.NullString,
	subcategoryName sql.NullString,
	subcategoryDescription sql.NullString,
	brandID sql.NullString,
	brandName sql.NullString,
	brandDescription sql.NullString,
) {
	if categoryID.Valid {
		item.CategoryObj = &models.Category{
			ID:          categoryID.String,
			MerchantID:  item.MerchantID,
			Name:        categoryName.String,
			Description: utils.NullStringToStringPtr(categoryDescription),
		}
	}

	if subcategoryID.Valid {
		item.SubcategoryObj = &models.Subcategory{
			ID:          subcategoryID.String,
			CategoryID:  categoryID.String,
			Name:        subcategoryName.String,
			Description: utils.NullStringToStringPtr(subcategoryDescription),
		}
	}

	if brandID.Valid {
		item.BrandObj = &models.Brand{
			ID:          brandID.String,
			MerchantID:  item.MerchantID,
			Name:        brandName.String,
			Description: utils.NullStringToStringPtr(brandDescription),
		}
	}
}

func fetchInventoryItemWithNested(ctx context.Context, db interface {
	QueryRow(context.Context, string, ...interface{}) interface{ Scan(...interface{}) error }
}, query string, args ...interface{}) (models.InventoryItem, error) {
	var item models.InventoryItem
	var categoryID, categoryName, categoryDescription sql.NullString
	var subcategoryID, subcategoryName, subcategoryDescription sql.NullString
	var brandID, brandName, brandDescription sql.NullString

	err := db.QueryRow(ctx, query, args...).Scan(
		&item.ID, &item.MerchantID, &item.Name, &item.Description, &item.SKU, &item.SellingPrice,
		&item.OriginalPrice, &item.LowStockThreshold, &item.Category, &item.CategoryID,
		&item.SubcategoryID, &item.BrandID, &item.SupplierID, &item.IsArchived,
		&item.CreatedAt, &item.UpdatedAt,
		&categoryID, &categoryName, &categoryDescription,
		&subcategoryID, &subcategoryName, &subcategoryDescription,
		&brandID, &brandName, &brandDescription,
	)
	if err != nil {
		return item, err
	}

	hydrateNestedCatalogFields(
		&item,
		categoryID,
		categoryName,
		categoryDescription,
		subcategoryID,
		subcategoryName,
		subcategoryDescription,
		brandID,
		brandName,
		brandDescription,
	)

	return item, nil
}

func adjustShopStock(ctx context.Context, tx pgx.Tx, shopID, itemID, userID, clientOperationID, movementType, reason string, quantityChange int) (int, bool, error) {
	normalizedMovementType := movementType
	switch movementType {
	case "stock_in", "sale", "return", "adjustment":
		// already canonical
	case "return_to_supplier":
		normalizedMovementType = "return"
	case "inventory_correction", "damaged_goods", "expired_goods", "theft_loss", "correction_add", "correction_remove", "found_item", "spoilage", "damage", "theft", "other_add", "other_remove":
		normalizedMovementType = "adjustment"
	default:
		if quantityChange < 0 {
			normalizedMovementType = "adjustment"
		} else if normalizedMovementType == "" {
			normalizedMovementType = "stock_in"
		}
	}

	var currentQuantity int
	if err := tx.QueryRow(ctx, `SELECT quantity FROM shop_stock WHERE shop_id = $1 AND inventory_item_id = $2 FOR UPDATE`, shopID, itemID).Scan(&currentQuantity); err != nil {
		if err == pgx.ErrNoRows {
			return 0, false, sql.ErrNoRows
		}
		return 0, false, err
	}

	newQuantity := currentQuantity + quantityChange
	if newQuantity < 0 {
		return 0, false, fmt.Errorf("insufficient stock")
	}

	claimed, err := claimInventoryOperation(ctx, tx, clientOperationID, "merchant_stock_adjustment", userID, &shopID)
	if err != nil {
		return 0, false, err
	}
	if !claimed {
		return 0, false, nil
	}

	if _, err := tx.Exec(ctx, `
		UPDATE shop_stock
		SET quantity = $1, last_stocked_in_at = CURRENT_TIMESTAMP
		WHERE shop_id = $2 AND inventory_item_id = $3
	`, newQuantity, shopID, itemID); err != nil {
		return 0, false, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO stock_movements (shop_id, inventory_item_id, user_id, movement_type, quantity_changed, new_quantity, reason, movement_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, shopID, itemID, userID, normalizedMovementType, quantityChange, newQuantity, reason, time.Now()); err != nil {
		return 0, false, err
	}

	return newQuantity, true, nil
}

// HandleAdjustStock adjusts the stock for a given item in a shop.
func HandleAdjustStock(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	userID := claims.UserID

	shopID := c.Params("shopId")
	itemID := c.Params("itemId")

	var req struct {
		ClientOperationID string `json:"clientOperationId"`
		Quantity          int    `json:"quantity"`
		Reason            string `json:"reason"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}
	// Validation check
	var exists bool
	checkQuery := "SELECT EXISTS(SELECT 1 FROM shop_stock WHERE shop_id = $1 AND inventory_item_id = $2)"
	err = db.QueryRow(ctx, checkQuery, shopID, itemID).Scan(&exists)
	if err != nil {
		log.Printf("Error checking shop_stock link: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error during validation"})
	}
	if !exists {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Item not found in this shop's stock"})
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to start transaction"})
	}
	defer tx.Rollback(ctx)

	if req.ClientOperationID == "" {
		req.ClientOperationID = fmt.Sprintf("legacy-adjust-%s-%s-%d-%d", shopID, itemID, req.Quantity, time.Now().UnixNano())
	}

	newQuantity, processed, err := adjustShopStock(ctx, tx, shopID, itemID, userID, req.ClientOperationID, "adjustment", req.Reason, req.Quantity)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Item not found in this shop for stock adjustment"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to adjust stock"})
	}
	if !processed {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "message": "Stock adjustment already processed"})
	}

	if err := tx.Commit(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to commit transaction"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "data": fiber.Map{"newQuantity": newQuantity}})
}

// HandleGetStockMovementHistory retrieves the stock movement history for an item.
func HandleGetStockMovementHistory(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	shopID := c.Params("shopId")
	itemID := c.Params("itemId")

	query := `
		SELECT id, shop_id, inventory_item_id, user_id, movement_type, quantity_changed, new_quantity, reason, notes, movement_date
		FROM stock_movements 
		WHERE shop_id = $1 AND inventory_item_id = $2
		ORDER BY movement_date DESC
	`
	rows, err := db.Query(ctx, query, shopID, itemID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve stock movement history"})
	}
	defer rows.Close()

	history := make([]models.StockMovement, 0)
	for rows.Next() {
		var h models.StockMovement
		var reason sql.NullString
		var notes sql.NullString
		if err := rows.Scan(&h.ID, &h.ShopID, &h.InventoryItemID, &h.UserID, &h.MovementType, &h.QuantityChanged, &h.NewQuantity, &reason, &notes, &h.MovementDate); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to scan history data"})
		}
		if reason.Valid {
			h.Reason = &reason.String
		}
		if notes.Valid {
			h.Notes = &notes.String
		}
		history = append(history, h)
	}

	return c.JSON(fiber.Map{"status": "success", "success": true, "data": history})
}

func HandleListInventoryItems(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	merchantID := claims.UserID

	// Optional filter query parameters
	categoryId := c.Query("categoryId")
	subcategoryId := c.Query("subcategoryId")
	brandId := c.Query("brandId")

	baseQuery := `
		SELECT
			i.id, i.merchant_id, i.name, i.description, i.sku, i.selling_price, i.original_price,
			i.low_stock_threshold, i.category, i.category_id, i.subcategory_id, i.brand_id,
			i.supplier_id, i.is_archived, i.created_at, i.updated_at,
			c.id, c.name, c.description,
			sc.id, sc.name, sc.description,
			b.id, b.name, b.description
		FROM inventory_items i
		LEFT JOIN categories c ON i.category_id = c.id
		LEFT JOIN subcategories sc ON i.subcategory_id = sc.id
		LEFT JOIN brands b ON i.brand_id = b.id
		WHERE i.merchant_id = $1
	`

	whereClauses := ""
	args := []interface{}{merchantID}
	argPos := 2
	if categoryId != "" {
		whereClauses += fmt.Sprintf(" AND i.category_id = $%d", argPos)
		args = append(args, categoryId)
		argPos++
	}
	if subcategoryId != "" {
		whereClauses += fmt.Sprintf(" AND i.subcategory_id = $%d", argPos)
		args = append(args, subcategoryId)
		argPos++
	}
	if brandId != "" {
		whereClauses += fmt.Sprintf(" AND i.brand_id = $%d", argPos)
		args = append(args, brandId)
		argPos++
	}

	finalQuery := baseQuery + whereClauses
	rows, err := db.Query(ctx, finalQuery, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve inventory items"})
	}
	defer rows.Close()

	items := make([]models.InventoryItem, 0)
	for rows.Next() {
		var item models.InventoryItem
		var categoryID, categoryName, categoryDescription sql.NullString
		var subcategoryID, subcategoryName, subcategoryDescription sql.NullString
		var brandID, brandName, brandDescription sql.NullString

		if err := rows.Scan(
			&item.ID, &item.MerchantID, &item.Name, &item.Description, &item.SKU, &item.SellingPrice,
			&item.OriginalPrice, &item.LowStockThreshold, &item.Category, &item.CategoryID,
			&item.SubcategoryID, &item.BrandID, &item.SupplierID, &item.IsArchived,
			&item.CreatedAt, &item.UpdatedAt,
			&categoryID, &categoryName, &categoryDescription,
			&subcategoryID, &subcategoryName, &subcategoryDescription,
			&brandID, &brandName, &brandDescription,
		); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to scan inventory item data"})
		}

		hydrateNestedCatalogFields(
			&item,
			categoryID,
			categoryName,
			categoryDescription,
			subcategoryID,
			subcategoryName,
			subcategoryDescription,
			brandID,
			brandName,
			brandDescription,
		)
		items = append(items, item)
	}

	return c.JSON(fiber.Map{"status": "success", "success": true, "data": items})
}

func HandleCreateInventoryItem(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	merchantID := claims.UserID

	var req models.InventoryItem
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}
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
	claimed, err := claimInventoryOperation(ctx, tx, clientOperationID, "merchant_create_inventory_item", merchantID, nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to start operation"})
	}
	if !claimed {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "message": "Operation already processed"})
	}

	// Convert empty strings to nil for optional fields to avoid unique constraint issues
	if req.Description != nil && *req.Description == "" {
		req.Description = nil
	}
	if req.SKU != nil && *req.SKU == "" {
		req.SKU = nil
	}
	if req.Category != nil && *req.Category == "" {
		req.Category = nil
	}

	if req.CategoryID != nil && *req.CategoryID == "" {
		req.CategoryID = nil
	}
	if req.SubcategoryID != nil && *req.SubcategoryID == "" {
		req.SubcategoryID = nil
	}
	if req.BrandID != nil && *req.BrandID == "" {
		req.BrandID = nil
	}

	query := `
		INSERT INTO inventory_items (merchant_id, name, description, sku, selling_price, original_price, low_stock_threshold, category, category_id, subcategory_id, brand_id, supplier_id, is_archived)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, created_at, updated_at
	`
	err = tx.QueryRow(ctx, query, merchantID, req.Name, req.Description, req.SKU, req.SellingPrice, req.OriginalPrice, req.LowStockThreshold, req.Category, req.CategoryID, req.SubcategoryID, req.BrandID, req.SupplierID, req.IsArchived).Scan(&req.ID, &req.CreatedAt, &req.UpdatedAt)
	if err != nil {
		log.Printf("Error creating inventory item: %v", err)
		log.Printf("Values: merchantID=%s, name=%s, description=%v, sku=%v, sellingPrice=%v, originalPrice=%v, lowStockThreshold=%v, category=%v, supplierID=%v, isArchived=%v",
			merchantID, req.Name, req.Description, req.SKU, req.SellingPrice, req.OriginalPrice, req.LowStockThreshold, req.Category, req.SupplierID, req.IsArchived)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create inventory item"})
	}

	if req.CategoryID != nil {
		var cat models.Category
		var catDesc sql.NullString
		if err := tx.QueryRow(ctx, `SELECT id, merchant_id, name, description, created_at, updated_at FROM categories WHERE id = $1`, *req.CategoryID).Scan(&cat.ID, &cat.MerchantID, &cat.Name, &catDesc, &cat.CreatedAt, &cat.UpdatedAt); err == nil {
			if catDesc.Valid {
				cat.Description = &catDesc.String
			}
			req.CategoryObj = &cat
		}
	}

	if req.SubcategoryID != nil {
		var sub models.Subcategory
		var subDesc sql.NullString
		if err := tx.QueryRow(ctx, `SELECT id, category_id, name, description, created_at, updated_at FROM subcategories WHERE id = $1`, *req.SubcategoryID).Scan(&sub.ID, &sub.CategoryID, &sub.Name, &subDesc, &sub.CreatedAt, &sub.UpdatedAt); err == nil {
			if subDesc.Valid {
				sub.Description = &subDesc.String
			}
			req.SubcategoryObj = &sub
		}
	}

	if req.BrandID != nil {
		var b models.Brand
		var bDesc sql.NullString
		if err := tx.QueryRow(ctx, `SELECT id, merchant_id, name, description, created_at, updated_at FROM brands WHERE id = $1`, *req.BrandID).Scan(&b.ID, &b.MerchantID, &b.Name, &bDesc, &b.CreatedAt, &b.UpdatedAt); err == nil {
			if bDesc.Valid {
				b.Description = &bDesc.String
			}
			req.BrandObj = &b
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to commit transaction"})
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": req})
}

func HandleGetInventoryItemByID(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	itemID := c.Params("itemId")

	query := `
		SELECT
			i.id, i.merchant_id, i.name, i.description, i.sku, i.selling_price, i.original_price,
			i.low_stock_threshold, i.category, i.category_id, i.subcategory_id, i.brand_id,
			i.supplier_id, i.is_archived, i.created_at, i.updated_at,
			c.id, c.name, c.description,
			sc.id, sc.name, sc.description,
			b.id, b.name, b.description
		FROM inventory_items i
		LEFT JOIN categories c ON i.category_id = c.id
		LEFT JOIN subcategories sc ON i.subcategory_id = sc.id
		LEFT JOIN brands b ON i.brand_id = b.id
		WHERE i.id = $1
	`
	var item models.InventoryItem
	var categoryID, categoryName, categoryDescription sql.NullString
	var subcategoryID, subcategoryName, subcategoryDescription sql.NullString
	var brandID, brandName, brandDescription sql.NullString
	err := db.QueryRow(ctx, query, itemID).Scan(
		&item.ID, &item.MerchantID, &item.Name, &item.Description, &item.SKU, &item.SellingPrice,
		&item.OriginalPrice, &item.LowStockThreshold, &item.Category, &item.CategoryID,
		&item.SubcategoryID, &item.BrandID, &item.SupplierID, &item.IsArchived,
		&item.CreatedAt, &item.UpdatedAt,
		&categoryID, &categoryName, &categoryDescription,
		&subcategoryID, &subcategoryName, &subcategoryDescription,
		&brandID, &brandName, &brandDescription,
	)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Inventory item not found"})
	}

	hydrateNestedCatalogFields(
		&item,
		categoryID,
		categoryName,
		categoryDescription,
		subcategoryID,
		subcategoryName,
		subcategoryDescription,
		brandID,
		brandName,
		brandDescription,
	)

	return c.JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleUpdateInventoryItem(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	merchantID := claims.UserID

	itemID := c.Params("itemId")
	var req models.InventoryItem
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}
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
	claimed, err := claimInventoryOperation(ctx, tx, clientOperationID, "merchant_update_inventory_item", merchantID, nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to start operation"})
	}
	if !claimed {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "message": "Operation already processed"})
	}

	// Convert empty strings to nil for optional fields to avoid unique constraint issues
	if req.Description != nil && *req.Description == "" {
		req.Description = nil
	}
	if req.SKU != nil && *req.SKU == "" {
		req.SKU = nil
	}
	if req.Category != nil && *req.Category == "" {
		req.Category = nil
	}
	if req.CategoryID != nil && *req.CategoryID == "" {
		req.CategoryID = nil
	}
	if req.SubcategoryID != nil && *req.SubcategoryID == "" {
		req.SubcategoryID = nil
	}
	if req.BrandID != nil && *req.BrandID == "" {
		req.BrandID = nil
	}

	query := `
		UPDATE inventory_items
		SET name = $1, description = $2, sku = $3, selling_price = $4, original_price = $5, low_stock_threshold = $6, category = $7, category_id = $8, subcategory_id = $9, brand_id = $10, supplier_id = $11, is_archived = $12
		WHERE id = $13
		RETURNING updated_at
	`
	err = tx.QueryRow(ctx, query, req.Name, req.Description, req.SKU, req.SellingPrice, req.OriginalPrice, req.LowStockThreshold, req.Category, req.CategoryID, req.SubcategoryID, req.BrandID, req.SupplierID, req.IsArchived, itemID).Scan(&req.UpdatedAt)
	if err != nil {
		log.Printf("Error updating inventory item %s: %v", itemID, err)
		log.Printf("Values: name=%s, description=%v, sku=%v, sellingPrice=%v, originalPrice=%v, lowStockThreshold=%v, category=%v, supplierID=%v, isArchived=%v",
			req.Name, req.Description, req.SKU, req.SellingPrice, req.OriginalPrice, req.LowStockThreshold, req.Category, req.SupplierID, req.IsArchived)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update inventory item"})
	}

	// Return canonical payload with nested category/subcategory/brand objects.
	if err := tx.Commit(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to commit transaction"})
	}
	return HandleGetInventoryItemByID(c)
}

func HandleDeleteInventoryItem(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	itemID := c.Params("itemId")
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
	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	claimed, err := claimInventoryOperation(ctx, tx, clientOperationID, "merchant_delete_inventory_item", claims.UserID, nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to start operation"})
	}
	if !claimed {
		return c.SendStatus(fiber.StatusNoContent)
	}

	query := "DELETE FROM inventory_items WHERE id = $1"
	_, err = tx.Exec(ctx, query, itemID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to delete inventory item"})
	}
	if err := tx.Commit(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to commit transaction"})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// HandleCheckDeleteInventoryItem performs a preflight check to see if an inventory item can be safely deleted.
// Returns { deletable: bool, blockers: {resourceName: count, ...} }
func HandleCheckDeleteInventoryItem(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "Unauthorized"})
	}
	merchantID := claims.UserID

	itemID := c.Params("itemId")

	// Verify item belongs to merchant
	var exists int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM inventory_items WHERE id = $1 AND merchant_id = $2", itemID, merchantID).Scan(&exists); err != nil || exists == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Inventory item not found or access denied"})
	}

	blockers := map[string]int{}
	var cnt int

	// shop_stock
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM shop_stock WHERE inventory_item_id = $1", itemID).Scan(&cnt); err == nil && cnt > 0 {
		blockers["shop_stock"] = cnt
	}

	// stock_movements
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM stock_movements WHERE inventory_item_id = $1", itemID).Scan(&cnt); err == nil && cnt > 0 {
		blockers["stock_movements"] = cnt
	}

	// sale_items
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM sale_items WHERE inventory_item_id = $1", itemID).Scan(&cnt); err == nil && cnt > 0 {
		blockers["sale_items"] = cnt
	}

	// promotion_products
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM promotion_products WHERE inventory_item_id = $1", itemID).Scan(&cnt); err == nil && cnt > 0 {
		blockers["promotion_products"] = cnt
	}

	deletable := len(blockers) == 0
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": fiber.Map{"deletable": deletable, "blockers": blockers}})
}

func HandleArchiveInventoryItem(c *fiber.Ctx) error {
	return handleArchiveStatus(c, true)
}

func HandleUnarchiveInventoryItem(c *fiber.Ctx) error {
	return handleArchiveStatus(c, false)
}

func handleArchiveStatus(c *fiber.Ctx, isArchived bool) error {
	db := database.GetDB()
	ctx := context.Background()

	itemID := c.Params("itemId")
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
	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	operationName := "merchant_archive_inventory_item"
	if !isArchived {
		operationName = "merchant_unarchive_inventory_item"
	}
	claimed, err := claimInventoryOperation(ctx, tx, clientOperationID, operationName, claims.UserID, nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to start operation"})
	}
	if !claimed {
		return c.SendStatus(fiber.StatusNoContent)
	}

	query := "UPDATE inventory_items SET is_archived = $1 WHERE id = $2"
	_, err = tx.Exec(ctx, query, isArchived, itemID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update archive status"})
	}
	if err := tx.Commit(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to commit transaction"})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func HandleListInventoryForShop(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()
	shopID := c.Params("shopId")

	// Parse query parameters for pagination
	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize", "10"))
	offset := (page - 1) * pageSize

	// Query to get paginated inventory items
	query := `
        SELECT 
            i.id, i.name, i.sku, i.selling_price, 
            s.quantity, s.last_stocked_in_at
        FROM inventory_items i
        JOIN shop_stock s ON i.id = s.inventory_item_id
        WHERE s.shop_id = $1
        LIMIT $2 OFFSET $3
    `

	rows, err := db.Query(ctx, query, shopID, pageSize, offset)
	if err != nil {
		log.Printf("Error querying shop inventory: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to retrieve shop inventory",
		})
	}
	defer rows.Close()

	var items []models.ShopStockItem
	for rows.Next() {
		var item models.ShopStockItem
		if err := rows.Scan(&item.ID, &item.ItemName, &item.ItemSku, &item.ItemUnitPrice, &item.Quantity, &item.LastStockedInAt); err != nil {
			log.Printf("Error scanning shop inventory item: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status":  "error",
				"message": "Error processing shop inventory data",
			})
		}
		items = append(items, item)
	}

	// Query to get the total count of items for pagination metadata
	countQuery := "SELECT COUNT(*) FROM shop_stock WHERE shop_id = $1"
	var totalItems int
	if err := db.QueryRow(ctx, countQuery, shopID).Scan(&totalItems); err != nil {
		log.Printf("Error counting shop inventory: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to count shop inventory",
		})
	}

	totalPages := (totalItems + pageSize - 1) / pageSize

	return c.JSON(models.PaginatedShopStockResponse{
		Items:       items,
		TotalItems:  totalItems,
		CurrentPage: page,
		PageSize:    pageSize,
		TotalPages:  totalPages,
	})
}

func HandleAdjustStockItem(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	userID := claims.UserID

	shopID := c.Params("shopId")
	itemID := c.Params("inventoryItemId")
	if shopID == "" || itemID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "shopId and inventoryItemId are required"})
	}

	var req struct {
		ClientOperationID string `json:"clientOperationId"`
		AdjustmentType    string `json:"adjustmentType"`
		QuantityChange    int    `json:"quantityChange"`
		Reason            string `json:"reason"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}
	if req.ClientOperationID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "clientOperationId is required"})
	}
	if req.QuantityChange == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "quantityChange must not be zero"})
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to start transaction"})
	}
	defer tx.Rollback(ctx)

	var dbMerchantID string
	if err := tx.QueryRow(ctx, "SELECT merchant_id FROM shops WHERE id = $1", shopID).Scan(&dbMerchantID); err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Shop not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Could not verify shop details"})
	}
	if dbMerchantID != userID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "You do not have permission to adjust stock for this shop"})
	}

	movementType := req.AdjustmentType
	if movementType == "" {
		movementType = "adjustment"
	}
	reason := req.Reason
	if reason == "" {
		reason = movementType
	}

	newQuantity, processed, err := adjustShopStock(ctx, tx, shopID, itemID, userID, req.ClientOperationID, movementType, reason, req.QuantityChange)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Item not found in this shop for stock adjustment"})
		}
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"status": "error", "message": "Failed to adjust stock"})
	}
	if !processed {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "message": "Stock adjustment already processed"})
	}

	if err := tx.Commit(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to commit transaction"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "data": fiber.Map{"newQuantity": newQuantity}})
}

func HandleStockInItem(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	merchantID := claims.UserID

	shopID := c.Params("shopId")
	inventoryItemID := c.Params("inventoryItemId")
	if shopID == "" || inventoryItemID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "shopId and inventoryItemId are required"})
	}

	var req struct {
		ClientOperationID string `json:"clientOperationId"`
		QuantityAdded     int    `json:"quantityAdded"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}
	if req.ClientOperationID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "clientOperationId is required"})
	}
	if req.QuantityAdded <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "quantityAdded must be greater than 0"})
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to start transaction"})
	}
	defer tx.Rollback(ctx)

	var dbMerchantID string
	if err := tx.QueryRow(ctx, "SELECT merchant_id FROM shops WHERE id = $1", shopID).Scan(&dbMerchantID); err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Shop not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Could not verify shop details"})
	}
	if dbMerchantID != merchantID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "You do not have permission to stock in items for this shop"})
	}

	var itemMerchantID string
	if err := tx.QueryRow(ctx, "SELECT merchant_id FROM inventory_items WHERE id = $1", inventoryItemID).Scan(&itemMerchantID); err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Inventory item not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Could not verify inventory item"})
	}
	if itemMerchantID != merchantID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "You do not have permission to stock in this item"})
	}

	claimed, err := claimInventoryOperation(ctx, tx, req.ClientOperationID, "merchant_single_stock_in", merchantID, &shopID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to start inventory operation"})
	}
	if !claimed {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "message": "Stock-in already processed"})
	}

	var newQuantity int
	stockUpsertQuery := `
		INSERT INTO shop_stock (shop_id, inventory_item_id, quantity)
		VALUES ($1, $2, $3)
		ON CONFLICT (shop_id, inventory_item_id)
		DO UPDATE SET quantity = shop_stock.quantity + EXCLUDED.quantity, last_stocked_in_at = CURRENT_TIMESTAMP
		RETURNING quantity
	`
	if err := tx.QueryRow(ctx, stockUpsertQuery, shopID, inventoryItemID, req.QuantityAdded).Scan(&newQuantity); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update stock quantity"})
	}

	movementQuery := `
		INSERT INTO stock_movements (inventory_item_id, shop_id, user_id, movement_type, quantity_changed, new_quantity, reason)
		VALUES ($1, $2, $3, 'stock_in', $4, $5, 'Merchant single-item stock-in')
	`
	if _, err := tx.Exec(ctx, movementQuery, inventoryItemID, shopID, merchantID, req.QuantityAdded, newQuantity); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to log stock change"})
	}

	if err := tx.Commit(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to finalize stock-in process"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":  "success",
		"message": "Stock-in successful",
		"data": fiber.Map{
			"shopId":            shopID,
			"inventoryItemId":   inventoryItemID,
			"quantity":          newQuantity,
			"clientOperationId": req.ClientOperationID,
		},
	})
}
