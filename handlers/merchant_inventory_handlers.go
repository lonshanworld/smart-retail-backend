package handlers

import (
	"app/database"
	"app/middleware"
	"app/models"
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v4"
)

// The merchant inventory API exposes stock_items as the merchant catalog item
// and inventory_items as its shop-specific balance. This keeps catalog data
// reusable across shops while keeping quantities isolated per shop.
const inventoryMasterSelect = `
	SELECT si.id, si.merchant_id, si.name, p.description, si.sku,
		COALESCE(pp.selling_price, 0), COALESCE(pp.cost_price, 0),
		NULL, NULL, NULL, p.brand_id, NULL, NOT p.is_active,
		si.created_at, si.updated_at
	FROM stock_items si
	JOIN products p ON p.id = si.product_id
	LEFT JOIN LATERAL (
		SELECT selling_price, cost_price FROM product_prices
		WHERE merchant_id = si.merchant_id AND product_id = si.product_id
			AND variant_id IS NULL AND (shop_id IS NULL)
			AND price_type = 'RETAIL'
		ORDER BY starts_at DESC NULLS LAST, created_at DESC LIMIT 1
	) pp ON TRUE
`

func inventoryItemFromRow(scan func(...interface{}) error) (models.InventoryItem, error) {
	var item models.InventoryItem
	var description, sku, categoryID, subcategoryID, brandID, supplierID sql.NullString
	var selling, original float64
	var threshold sql.NullFloat64
	err := scan(&item.ID, &item.MerchantID, &item.Name, &description, &sku, &selling, &original,
		&threshold, &categoryID, &subcategoryID, &brandID, &supplierID, &item.IsArchived, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return item, err
	}
	if description.Valid {
		item.Description = &description.String
	}
	if sku.Valid {
		item.SKU = &sku.String
	}
	item.SellingPrice = selling
	item.OriginalPrice = &original
	if threshold.Valid {
		v := int(threshold.Float64)
		item.LowStockThreshold = &v
	}
	if categoryID.Valid {
		item.CategoryID = &categoryID.String
	}
	if brandID.Valid {
		item.BrandID = &brandID.String
	}
	return item, nil
}

func merchantIDFromClaims(c *fiber.Ctx) (string, error) {
	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return "", err
	}
	return claims.UserID, nil
}

func normalizeSlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.Join(strings.Fields(s), "-")
	return fmt.Sprintf("%s-%d", s, time.Now().UnixNano())
}

func HandleListInventoryItems(c *fiber.Ctx) error {
	db, ctx := database.GetDB(), context.Background()
	merchantID, err := merchantIDFromClaims(c)
	if err != nil {
		return err
	}
	query := inventoryMasterSelect + " WHERE si.merchant_id = $1"
	args := []interface{}{merchantID}
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		query += fmt.Sprintf(" AND (si.name ILIKE $%d OR COALESCE(si.sku,'') ILIKE $%d)", len(args)+1, len(args)+1)
		args = append(args, "%"+search+"%")
	}
	if v := c.Query("categoryId"); v != "" {
		query += " AND EXISTS (SELECT 1 FROM product_categories pc WHERE pc.product_id = si.product_id AND pc.category_id = $2)"
		args = append(args, v)
	}
	if v := c.Query("brandId"); v != "" {
		query += fmt.Sprintf(" AND p.brand_id = $%d", len(args)+1)
		args = append(args, v)
	}
	if archived := strings.TrimSpace(c.Query("isArchived")); archived != "" {
		if value, parseErr := strconv.ParseBool(archived); parseErr == nil {
			query += fmt.Sprintf(" AND p.is_active = $%d", len(args)+1)
			args = append(args, !value)
		}
	}
	page, _ := strconv.Atoi(c.Query("page", "1"))
	size, _ := strconv.Atoi(c.Query("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	countQuery := "SELECT COUNT(*) FROM stock_items si JOIN products p ON p.id=si.product_id" + strings.Replace(query[len(inventoryMasterSelect):], " ORDER BY si.name", "", 1)
	var total int
	if err := db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to count inventory items"})
	}
	query += fmt.Sprintf(" ORDER BY si.name LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, size, (page-1)*size)
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve inventory items"})
	}
	defer rows.Close()
	items := make([]models.InventoryItem, 0)
	for rows.Next() {
		item, scanErr := inventoryItemFromRow(rows.Scan)
		if scanErr != nil {
			return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to scan inventory item"})
		}
		items = append(items, item)
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": items, "pagination": fiber.Map{"totalItems": total, "totalPages": (total + size - 1) / size, "currentPage": page, "pageSize": size}})
}

func HandleCreateInventoryItem(c *fiber.Ctx) error {
	db, ctx := database.GetDB(), context.Background()
	merchantID, err := merchantIDFromClaims(c)
	if err != nil {
		return err
	}
	var req models.InventoryItem
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "A valid item name is required"})
	}
	opID := c.Get("X-Client-Operation-Id")
	if opID == "" {
		opID = c.Query("clientOperationId")
	}
	if opID == "" {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "clientOperationId is required"})
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to start transaction"})
	}
	defer tx.Rollback(ctx)
	claimed, err := claimInventoryOperation(ctx, tx, opID, "merchant_create_inventory_item", merchantID, nil)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to start operation"})
	}
	if !claimed {
		return c.JSON(fiber.Map{"status": "success", "message": "Operation already processed"})
	}
	var productID, stockItemID string
	if err = tx.QueryRow(ctx, `INSERT INTO products (merchant_id, brand_id, name, slug, description, is_active) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`, merchantID, req.BrandID, req.Name, normalizeSlug(req.Name), req.Description, !req.IsArchived).Scan(&productID); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to create product"})
	}
	if err = tx.QueryRow(ctx, `INSERT INTO stock_items (merchant_id, product_id, name, sku) VALUES ($1,$2,$3,$4) RETURNING id`, merchantID, productID, req.Name, req.SKU).Scan(&stockItemID); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to create stock item"})
	}
	if _, err = tx.Exec(ctx, `INSERT INTO product_prices (merchant_id, product_id, price_type, cost_price, selling_price) VALUES ($1,$2,'RETAIL',$3,$4)`, merchantID, productID, valueOrZero(req.OriginalPrice), req.SellingPrice); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to create item price"})
	}
	if req.CategoryID != nil && *req.CategoryID != "" {
		var categoryOwned bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM categories WHERE id=$1 AND merchant_id=$2)`, *req.CategoryID, merchantID).Scan(&categoryOwned); err != nil {
			return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to validate category"})
		}
		if !categoryOwned {
			return c.Status(400).JSON(fiber.Map{"status": "error", "message": "Invalid category"})
		}
		if _, err = tx.Exec(ctx, `INSERT INTO product_categories (merchant_id, product_id, category_id) VALUES ($1,$2,$3)`, merchantID, productID, *req.CategoryID); err != nil {
			return c.Status(400).JSON(fiber.Map{"status": "error", "message": "Invalid category"})
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to commit item"})
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": fiber.Map{"id": stockItemID, "merchantId": merchantID, "name": req.Name, "sku": req.SKU, "sellingPrice": req.SellingPrice, "originalPrice": req.OriginalPrice, "isArchived": req.IsArchived}})
}

func valueOrZero(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func HandleGetInventoryItemByID(c *fiber.Ctx) error {
	db, ctx := database.GetDB(), context.Background()
	merchantID, err := merchantIDFromClaims(c)
	if err != nil {
		return err
	}
	id := c.Params("itemId")
	row := db.QueryRow(ctx, inventoryMasterSelect+" WHERE si.id = $1 AND si.merchant_id = $2", id, merchantID)
	item, err := inventoryItemFromRow(row.Scan)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"status": "error", "message": "Inventory item not found"})
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleUpdateInventoryItem(c *fiber.Ctx) error {
	db, ctx := database.GetDB(), context.Background()
	merchantID, err := merchantIDFromClaims(c)
	if err != nil {
		return err
	}
	id := c.Params("itemId")
	var req models.InventoryItem
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}
	opID := c.Get("X-Client-Operation-Id")
	if opID == "" {
		opID = c.Query("clientOperationId")
	}
	if opID == "" {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "clientOperationId is required"})
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to start transaction"})
	}
	defer tx.Rollback(ctx)
	claimed, err := claimInventoryOperation(ctx, tx, opID, "merchant_update_inventory_item", merchantID, nil)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to start operation"})
	}
	if !claimed {
		return c.JSON(fiber.Map{"status": "success", "message": "Operation already processed"})
	}
	var productID string
	if err = tx.QueryRow(ctx, `SELECT product_id FROM stock_items WHERE id=$1 AND merchant_id=$2`, id, merchantID).Scan(&productID); err != nil {
		return c.Status(404).JSON(fiber.Map{"status": "error", "message": "Inventory item not found"})
	}
	if _, err = tx.Exec(ctx, `UPDATE products SET name=COALESCE(NULLIF($1,''),name), description=$2, brand_id=$3, is_active=$4, updated_at=NOW() WHERE id=$5 AND merchant_id=$6`, req.Name, req.Description, req.BrandID, !req.IsArchived, productID, merchantID); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to update product"})
	}
	if _, err = tx.Exec(ctx, `UPDATE stock_items SET name=COALESCE(NULLIF($1,''),name), sku=$2, updated_at=NOW() WHERE id=$3 AND merchant_id=$4`, req.Name, req.SKU, id, merchantID); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to update stock item"})
	}
	if _, err = tx.Exec(ctx, `UPDATE product_prices SET selling_price=$1,cost_price=$2,updated_at=NOW() WHERE product_id=$3 AND merchant_id=$4 AND price_type='RETAIL'`, req.SellingPrice, valueOrZero(req.OriginalPrice), productID, merchantID); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to update price"})
	}
	if err = tx.Commit(ctx); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to commit update"})
	}
	return HandleGetInventoryItemByID(c)
}

func HandleDeleteInventoryItem(c *fiber.Ctx) error    { return setInventoryActive(c, false, true) }
func HandleArchiveInventoryItem(c *fiber.Ctx) error   { return setInventoryActive(c, false, false) }
func HandleUnarchiveInventoryItem(c *fiber.Ctx) error { return setInventoryActive(c, true, false) }

func setInventoryActive(c *fiber.Ctx, active bool, hardDelete bool) error {
	db, ctx := database.GetDB(), context.Background()
	merchantID, err := merchantIDFromClaims(c)
	if err != nil {
		return err
	}
	id := c.Params("itemId")
	if hardDelete {
		_, err = db.Exec(ctx, `DELETE FROM stock_items WHERE id=$1 AND merchant_id=$2`, id, merchantID)
	} else {
		_, err = db.Exec(ctx, `UPDATE products SET is_active=$1,updated_at=NOW() WHERE id=(SELECT product_id FROM stock_items WHERE id=$2 AND merchant_id=$3)`, active, id, merchantID)
	}
	if err != nil {
		log.Printf("inventory status change: %v", err)
		return c.Status(409).JSON(fiber.Map{"status": "error", "message": "Inventory item cannot be changed because it is in use"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func HandleCheckDeleteInventoryItem(c *fiber.Ctx) error {
	db, ctx := database.GetDB(), context.Background()
	merchantID, err := merchantIDFromClaims(c)
	if err != nil {
		return err
	}
	id := c.Params("itemId")
	var count int
	if err = db.QueryRow(ctx, `SELECT COUNT(*) FROM stock_items WHERE id=$1 AND merchant_id=$2`, id, merchantID).Scan(&count); err != nil || count == 0 {
		return c.Status(404).JSON(fiber.Map{"status": "error", "message": "Inventory item not found"})
	}
	blockers := map[string]int{}
	queries := map[string]string{"inventory_items": `SELECT COUNT(*) FROM inventory_items WHERE stock_item_id=$1`, "sale_items": `SELECT COUNT(*) FROM sale_items WHERE stock_item_id=$1`}
	for name, q := range queries {
		if db.QueryRow(ctx, q, id).Scan(&count) == nil && count > 0 {
			blockers[name] = count
		}
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": fiber.Map{"deletable": len(blockers) == 0, "blockers": blockers}})
}

func adjustCanonicalStock(ctx context.Context, tx pgx.Tx, shopID, stockItemID, merchantID, userID, opID string, delta float64, movementType, reason string) (float64, bool, error) {
	claimed, err := claimInventoryOperation(ctx, tx, opID, "stock_adjustment", userID, &shopID)
	if err != nil || !claimed {
		return 0, claimed, err
	}
	var inventoryID, productID string
	var current float64
	err = tx.QueryRow(ctx, `SELECT id,product_id,quantity_on_hand FROM inventory_items WHERE shop_id=$1 AND stock_item_id=$2 AND merchant_id=$3 FOR UPDATE`, shopID, stockItemID, merchantID).Scan(&inventoryID, &productID, &current)
	if err != nil {
		return 0, false, err
	}
	newQty := current + delta
	if newQty < 0 {
		return 0, false, fmt.Errorf("insufficient stock")
	}
	abs := math.Abs(delta)
	dbType := "ADJUSTMENT"
	if movementType == "stock_in" || delta > 0 {
		dbType = "IN"
	}
	if movementType == "sale" || delta < 0 {
		dbType = "OUT"
	}
	if _, err = tx.Exec(ctx, `UPDATE inventory_items SET quantity_on_hand=$1,updated_at=NOW() WHERE id=$2`, newQty, inventoryID); err != nil {
		return 0, false, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO inventory_movements (merchant_id,shop_id,inventory_item_id,product_id,stock_item_id,movement_type,quantity,base_quantity,reference_type,reference_id,event_key,notes) VALUES ($1,$2,$3,$4,$5,$6,$7,$7,NULL,NULL,$8,$9)`, merchantID, shopID, inventoryID, productID, stockItemID, dbType, abs, opID, reason)
	return newQty, true, err
}

func HandleAdjustStock(c *fiber.Ctx) error {
	return handleStockAdjustment(c, c.Params("itemId"), c.Params("shopId"), "adjustment")
}
func HandleAdjustStockItem(c *fiber.Ctx) error {
	return handleStockAdjustment(c, c.Params("inventoryItemId"), c.Params("shopId"), "adjustment")
}

func handleStockAdjustment(c *fiber.Ctx, stockItemID, shopID, mode string) error {
	merchantID, err := merchantIDFromClaims(c)
	if err != nil {
		return err
	}
	var req struct {
		ClientOperationID string  `json:"clientOperationId"`
		Quantity          float64 `json:"quantity"`
		QuantityChange    float64 `json:"quantityChange"`
		QuantityAdded     float64 `json:"quantityAdded"`
		Reason            string  `json:"reason"`
		AdjustmentType    string  `json:"adjustmentType"`
	}
	if err = c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}
	delta := req.Quantity
	if delta == 0 {
		delta = req.QuantityChange
	}
	if delta == 0 {
		delta = req.QuantityAdded
	}
	if req.ClientOperationID == "" {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "clientOperationId is required"})
	}
	if mode == "stock_in" {
		delta = math.Abs(delta)
	}
	db, ctx := database.GetDB(), context.Background()
	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to start transaction"})
	}
	defer tx.Rollback(ctx)
	if req.AdjustmentType == "sale" {
		delta = -math.Abs(delta)
	}
	newQty, processed, err := adjustCanonicalStock(ctx, tx, shopID, stockItemID, merchantID, merchantID, req.ClientOperationID, delta, req.AdjustmentType, req.Reason)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{"status": "error", "message": "Item is not stocked in this shop"})
		}
		return c.Status(409).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}
	if !processed {
		return c.JSON(fiber.Map{"status": "success", "message": "Stock adjustment already processed"})
	}
	if err = tx.Commit(ctx); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to commit stock adjustment"})
	}
	return c.JSON(fiber.Map{"status": "success", "data": fiber.Map{"newQuantity": newQty}})
}

func HandleStockInItem(c *fiber.Ctx) error {
	return handleStockAdjustment(c, c.Params("inventoryItemId"), c.Params("shopId"), "stock_in")
}

func HandleGetStockMovementHistory(c *fiber.Ctx) error {
	db, ctx := database.GetDB(), context.Background()
	if err := authorizeShopAccess(c, c.Params("shopId")); err != nil {
		return err
	}
	page, _ := strconv.Atoi(c.Query("page", "1"))
	size, _ := strconv.Atoi(c.Query("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	var total int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM inventory_movements WHERE shop_id=$1 AND stock_item_id=$2`, c.Params("shopId"), c.Params("itemId")).Scan(&total); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to count stock movement history"})
	}
	rows, err := db.Query(ctx, `SELECT id,shop_id,stock_item_id,movement_type,quantity,movement_date,notes FROM inventory_movements WHERE shop_id=$1 AND stock_item_id=$2 ORDER BY movement_date DESC LIMIT $3 OFFSET $4`, c.Params("shopId"), c.Params("itemId"), size, (page-1)*size)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve stock movement history"})
	}
	defer rows.Close()
	history := make([]models.StockMovement, 0)
	for rows.Next() {
		var h models.StockMovement
		var qty float64
		var notes sql.NullString
		if err := rows.Scan(&h.ID, &h.ShopID, &h.InventoryItemID, &h.MovementType, &qty, &h.MovementDate, &notes); err != nil {
			return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to scan history"})
		}
		h.QuantityChanged = int(qty)
		if notes.Valid {
			h.Notes = &notes.String
		}
		history = append(history, h)
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": history, "pagination": fiber.Map{"totalItems": total, "totalPages": (total + size - 1) / size, "currentPage": page, "pageSize": size}})
}

func HandleListInventoryForShop(c *fiber.Ctx) error {
	db, ctx := database.GetDB(), context.Background()
	shopID := c.Params("shopId")
	if err := authorizeShopAccess(c, shopID); err != nil {
		return err
	}
	page, _ := strconv.Atoi(c.Query("page", "1"))
	size, _ := strconv.Atoi(c.Query("pageSize", "10"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 10
	}
	off := (page - 1) * size
	where := " WHERE ii.shop_id=$1"
	args := []interface{}{shopID}
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		where += " AND (si.name ILIKE $2 OR si.sku ILIKE $2)"
		args = append(args, "%"+search+"%")
	}
	if categoryID := strings.TrimSpace(c.Query("categoryId")); categoryID != "" {
		where += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM product_categories pc WHERE pc.product_id=ii.product_id AND pc.category_id=$%d)", len(args)+1)
		args = append(args, categoryID)
	}
	if brandID := strings.TrimSpace(c.Query("brandId")); brandID != "" {
		where += fmt.Sprintf(" AND p.brand_id=$%d", len(args)+1)
		args = append(args, brandID)
	}
	if c.Query("lowStock") == "true" {
		where += " AND ii.low_stock_threshold IS NOT NULL AND ii.quantity_on_hand <= ii.low_stock_threshold"
	}
	var total int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM inventory_items ii JOIN stock_items si ON si.id=ii.stock_item_id JOIN products p ON p.id=ii.product_id"+where, args...).Scan(&total); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to count shop inventory"})
	}
	query := `SELECT ii.id,ii.shop_id,ii.stock_item_id,si.name,si.sku,COALESCE(pp.selling_price,0),ii.quantity_on_hand,ii.created_at,ii.updated_at FROM inventory_items ii JOIN stock_items si ON si.id=ii.stock_item_id JOIN products p ON p.id=ii.product_id LEFT JOIN LATERAL (SELECT selling_price FROM product_prices WHERE product_id=ii.product_id AND shop_id IS NULL AND price_type='RETAIL' ORDER BY created_at DESC LIMIT 1) pp ON TRUE` + where + fmt.Sprintf(" ORDER BY si.name LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, size, off)
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve shop inventory"})
	}
	defer rows.Close()
	items := make([]models.ShopStockItem, 0)
	for rows.Next() {
		var i models.ShopStockItem
		var q float64
		if err := rows.Scan(&i.ID, &i.ShopID, &i.InventoryItemID, &i.ItemName, &i.ItemSku, &i.ItemUnitPrice, &q, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to scan shop inventory"})
		}
		i.Quantity = int(q)
		i.LastStockedInAt = i.UpdatedAt
		items = append(items, i)
	}
	return c.JSON(models.PaginatedShopStockResponse{Items: items, TotalItems: total, CurrentPage: page, PageSize: size, TotalPages: (total + size - 1) / size})
}

// handleBulkStockIn is shared by merchant and shop inventory endpoints.
// ProductID in the API is the canonical stock_items.id; the shop balance is
// created lazily the first time stock is received at that shop.
func handleBulkStockIn(c *fiber.Ctx, shopID, merchantID, operationType string) error {
	var req models.StockInRequest
	if err := c.BodyParser(&req); err != nil || len(req.Items) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "At least one stock item is required"})
	}
	if req.ClientOperationID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "clientOperationId is required"})
	}
	db, ctx := database.GetDB(), context.Background()
	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to start stock-in"})
	}
	defer tx.Rollback(ctx)
	var owner string
	if err = tx.QueryRow(ctx, `SELECT merchant_id FROM shops WHERE id=$1`, shopID).Scan(&owner); err != nil {
		return c.Status(404).JSON(fiber.Map{"status": "error", "message": "Shop not found"})
	}
	if owner != merchantID {
		return c.Status(403).JSON(fiber.Map{"status": "error", "message": "Shop access denied"})
	}
	claimed, err := claimInventoryOperation(ctx, tx, req.ClientOperationID, operationType, merchantID, &shopID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to start inventory operation"})
	}
	if !claimed {
		return c.JSON(fiber.Map{"status": "success", "message": "Stock-in already processed"})
	}
	for _, item := range req.Items {
		if item.Quantity <= 0 {
			continue
		}
		var productID string
		if err = tx.QueryRow(ctx, `SELECT product_id FROM stock_items WHERE id=$1 AND merchant_id=$2`, item.ProductID, merchantID).Scan(&productID); err != nil {
			return c.Status(400).JSON(fiber.Map{"status": "error", "message": "Invalid stock item: " + item.ProductID})
		}
		var inventoryID string
		if err = tx.QueryRow(ctx, `SELECT id FROM inventory_items WHERE shop_id=$1 AND stock_item_id=$2 FOR UPDATE`, shopID, item.ProductID).Scan(&inventoryID); err == pgx.ErrNoRows {
			err = tx.QueryRow(ctx, `INSERT INTO inventory_items (merchant_id,shop_id,product_id,stock_item_id,quantity_on_hand) VALUES ($1,$2,$3,$4,$5) RETURNING id`, merchantID, shopID, productID, item.ProductID, item.Quantity).Scan(&inventoryID)
		} else if err == nil {
			_, err = tx.Exec(ctx, `UPDATE inventory_items SET quantity_on_hand=quantity_on_hand+$1,updated_at=NOW() WHERE id=$2`, item.Quantity, inventoryID)
		}
		if err != nil {
			return c.Status(409).JSON(fiber.Map{"status": "error", "message": "Failed to update stock quantity"})
		}
		if _, err = tx.Exec(ctx, `INSERT INTO inventory_movements (merchant_id,shop_id,inventory_item_id,product_id,stock_item_id,movement_type,quantity,base_quantity,reference_type,reference_id,event_key,notes) VALUES ($1,$2,$3,$4,$5,'IN',$6,$6,'STOCK_IN',NULL,$7,$8)`, merchantID, shopID, inventoryID, productID, item.ProductID, item.Quantity, fmt.Sprintf("%s:%s", req.ClientOperationID, item.ProductID), "Bulk stock-in"); err != nil {
			return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to log stock movement"})
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to commit stock-in"})
	}
	return c.JSON(fiber.Map{"status": "success", "message": "Stock-in successful"})
}
