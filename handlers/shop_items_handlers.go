package handlers

import (
	"app/database"
	"app/middleware"
	"app/models"
	"context"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v4/pgxpool"
	"strconv"
)

func getShopIDFromStaffID(ctx context.Context, db *pgxpool.Pool, staffID string) (string, error) {
	var id string
	err := db.QueryRow(ctx, `SELECT assigned_shop_id FROM users WHERE id=$1 AND role='staff'`, staffID).Scan(&id)
	return id, err
}

func HandleGetShopItems(c *fiber.Ctx) error {
	shopID := c.Query("shopId")
	if shopID == "" {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "shopId query parameter is required"})
	}
	if err := authorizeShopAccess(c, shopID); err != nil {
		return err
	}
	return listCanonicalShopItems(c, shopID)
}

func authorizeShopAccess(c *fiber.Ctx, shopID string) error {
	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	db, ctx := database.GetDB(), context.Background()
	if claims.Role == "merchant" {
		var owner string
		if err = db.QueryRow(ctx, `SELECT merchant_id FROM shops WHERE id=$1`, shopID).Scan(&owner); err != nil {
			return fiber.NewError(404, "Shop not found")
		}
		if owner != claims.UserID {
			return fiber.NewError(403, "Shop access denied")
		}
		return nil
	}
	if claims.Role == "staff" {
		assigned, e := getShopIDFromStaffID(ctx, db, claims.UserID)
		if e != nil || assigned != shopID {
			return fiber.NewError(403, "Shop access denied")
		}
		return nil
	}
	return fiber.NewError(403, "Shop access denied")
}
func HandleGetStaffItems(c *fiber.Ctx) error {
	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	db, ctx := database.GetDB(), context.Background()
	shopID, err := getShopIDFromStaffID(ctx, db, claims.UserID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"status": "error", "message": "Assigned shop not found"})
	}
	return listCanonicalShopItems(c, shopID)
}

func listCanonicalShopItems(c *fiber.Ctx, shopID string) error {
	db, ctx := database.GetDB(), context.Background()
	page, _ := strconv.Atoi(c.Query("page", "1"))
	size, _ := strconv.Atoi(c.Query("pageSize", "50"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 50
	}
	off := (page - 1) * size
	search := c.Query("searchTerm")
	args := []interface{}{shopID}
	where := ""
	if search != "" {
		where = " AND (si.name ILIKE $2 OR si.sku ILIKE $2)"
		args = append(args, "%"+search+"%")
	}
	if v := c.Query("categoryId"); v != "" {
		where += fmt.Sprintf(" AND EXISTS(SELECT 1 FROM product_categories pc WHERE pc.product_id=si.product_id AND pc.category_id=$%d)", len(args)+1)
		args = append(args, v)
	}
	if v := c.Query("brandId"); v != "" {
		where += fmt.Sprintf(" AND p.brand_id=$%d", len(args)+1)
		args = append(args, v)
	}
	var total int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM inventory_items ii JOIN stock_items si ON si.id=ii.stock_item_id JOIN products p ON p.id=si.product_id WHERE ii.shop_id=$1`+where, args...).Scan(&total); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to count shop inventory"})
	}
	query := `SELECT si.id,ii.merchant_id,si.name,si.sku,COALESCE(pp.selling_price,0),COALESCE(pp.cost_price,0),ii.quantity_on_hand,ii.shop_id,si.created_at,si.updated_at FROM inventory_items ii JOIN stock_items si ON si.id=ii.stock_item_id JOIN products p ON p.id=si.product_id LEFT JOIN LATERAL(SELECT selling_price,cost_price FROM product_prices WHERE product_id=si.product_id AND shop_id IS NULL AND price_type='RETAIL' ORDER BY created_at DESC LIMIT 1)pp ON TRUE WHERE ii.shop_id=$1` + where + ` ORDER BY si.name LIMIT $` + strconv.Itoa(len(args)+1) + ` OFFSET $` + strconv.Itoa(len(args)+2)
	args = append(args, size, off)
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve shop inventory"})
	}
	defer rows.Close()
	items := make([]models.InventoryItem, 0)
	for rows.Next() {
		var item models.InventoryItem
		var sku *string
		var cost float64
		var qty float64
		var stock models.ShopStock
		if err := rows.Scan(&item.ID, &item.MerchantID, &item.Name, &sku, &item.SellingPrice, &cost, &qty, &stock.ShopID, &item.CreatedAt, &item.UpdatedAt); err != nil {
			continue
		}
		item.SKU = sku
		item.OriginalPrice = &cost
		stock.InventoryItemID = item.ID
		stock.Quantity = int(qty)
		stock.LastStockedInAt = item.UpdatedAt
		item.Stock = &stock
		items = append(items, item)
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": items, "pagination": models.Pagination{TotalItems: total, TotalPages: (total + size - 1) / size, CurrentPage: page, PageSize: size}})
}

type UpdateStockRequest struct {
	Quantity int `json:"quantity"`
}

func HandleUpdateShopItemStock(c *fiber.Ctx) error {
	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	var req UpdateStockRequest
	if err = c.BodyParser(&req); err != nil || req.Quantity < 0 {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "A non-negative quantity is required"})
	}
	db, ctx := database.GetDB(), context.Background()
	shopID := c.Query("shopId")
	if claims.Role == "staff" {
		shopID, err = getShopIDFromStaffID(ctx, db, claims.UserID)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"status": "error", "message": "Assigned shop not found"})
		}
	} else if claims.Role != "merchant" {
		return c.Status(403).JSON(fiber.Map{"status": "error", "message": "Shop access denied"})
	}
	if shopID == "" {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "shopId is required"})
	}
	if err = authorizeShopAccess(c, shopID); err != nil {
		return err
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to start stock update"})
	}
	defer tx.Rollback(ctx)
	opID := c.Get("X-Client-Operation-Id")
	if opID == "" {
		opID = c.Query("clientOperationId")
	}
	if opID == "" {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "clientOperationId is required"})
	}
	claimed, err := claimInventoryOperation(ctx, tx, opID, "shop_update_stock", claims.UserID, &shopID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to start stock update"})
	}
	if !claimed {
		return c.JSON(fiber.Map{"status": "success", "success": true, "message": "Operation already processed"})
	}
	var invID string
	var old float64
	if err = tx.QueryRow(ctx, `SELECT id,quantity_on_hand FROM inventory_items WHERE shop_id=$1 AND stock_item_id=$2 FOR UPDATE`, shopID, c.Params("itemId")).Scan(&invID, &old); err != nil {
		return c.Status(404).JSON(fiber.Map{"status": "error", "message": "Item not found in shop inventory"})
	}
	delta := float64(req.Quantity) - old
	var merchantID, productID string
	if err = tx.QueryRow(ctx, `SELECT merchant_id,product_id FROM inventory_items WHERE id=$1`, invID).Scan(&merchantID, &productID); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to resolve item"})
	}
	if _, err = tx.Exec(ctx, `UPDATE inventory_items SET quantity_on_hand=$1,updated_at=NOW() WHERE id=$2`, req.Quantity, invID); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to update quantity"})
	}
	if delta != 0 {
		typ := "ADJUSTMENT"
		qty := delta
		if qty < 0 {
			typ = "OUT"
			qty = -qty
		} else {
			typ = "IN"
		}
		_, err = tx.Exec(ctx, `INSERT INTO inventory_movements(merchant_id,shop_id,inventory_item_id,product_id,stock_item_id,movement_type,quantity,base_quantity,reference_type,notes) VALUES($1,$2,$3,$4,$5,$6,$7,$7,'STAFF_ADJUSTMENT',$8)`, merchantID, shopID, invID, productID, c.Params("itemId"), typ, qty, "Staff stock quantity update")
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to record stock update"})
	}
	if err = tx.Commit(ctx); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to commit stock update"})
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "quantity": req.Quantity})
}
