package handlers

import (
	"app/database"
	"app/middleware"
	"app/models"
	"context"
	"database/sql"
	"github.com/gofiber/fiber/v2"
	"strconv"
)

// HandleGetCombinedStocks returns every canonical stock balance across the
// merchant's shops with stable pagination and optional name/SKU/shop filters.
func HandleGetCombinedStocks(c *fiber.Ctx) error {
	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}
	page, _ := strconv.Atoi(c.Query("page", "1"))
	size, _ := strconv.Atoi(c.Query("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	offset := (page - 1) * size
	search := c.Query("searchTerm")
	shopID := c.Query("shopId")
	db, ctx := database.GetDB(), context.Background()
	where := ` WHERE ii.merchant_id=$1`
	args := []interface{}{claims.UserID}
	if search != "" {
		where += " AND (si.name ILIKE $2 OR si.sku ILIKE $2)"
		args = append(args, "%"+search+"%")
	}
	if shopID != "" {
		where += ` AND ii.shop_id=$` + strconv.Itoa(len(args)+1)
		args = append(args, shopID)
	}
	var total int
	if err = db.QueryRow(ctx, `SELECT COUNT(*) FROM inventory_items ii JOIN stock_items si ON si.id=ii.stock_item_id`+where, args...).Scan(&total); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}
	query := `SELECT si.id,si.name,si.sku,ii.quantity_on_hand,COALESCE(pp.selling_price,0),COALESCE(pp.cost_price,0),s.name,s.id,p.brand_id FROM inventory_items ii JOIN stock_items si ON si.id=ii.stock_item_id JOIN products p ON p.id=si.product_id JOIN shops s ON s.id=ii.shop_id LEFT JOIN LATERAL(SELECT selling_price,cost_price FROM product_prices WHERE product_id=si.product_id AND shop_id IS NULL AND price_type='RETAIL' ORDER BY created_at DESC LIMIT 1)pp ON TRUE` + where + ` ORDER BY si.name,s.name LIMIT $` + strconv.Itoa(len(args)+1) + ` OFFSET $` + strconv.Itoa(len(args)+2)
	args = append(args, size, offset)
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}
	defer rows.Close()
	items := make([]models.CombinedStockItem, 0)
	for rows.Next() {
		var i models.CombinedStockItem
		var sku, brand sql.NullString
		var q float64
		var cost float64
		if err = rows.Scan(&i.ID, &i.Name, &sku, &q, &i.SellingPrice, &cost, &i.ShopName, &i.ShopID, &brand); err != nil {
			continue
		}
		if sku.Valid {
			i.SKU = &sku.String
		}
		if brand.Valid {
			i.BrandID = &brand.String
		}
		i.Quantity = int(q)
		i.OriginalPrice = &cost
		items = append(items, i)
	}
	return c.JSON(fiber.Map{"status": "success", "data": items, "pagination": models.Pagination{TotalItems: total, TotalPages: (total + size - 1) / size, CurrentPage: page, PageSize: size}})
}
