package handlers

import (
	"app/database"
	"app/middleware"
	"app/models"
	"context"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"strings"
)

func HandleGetShopInventory(c *fiber.Ctx) error {
	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	db, ctx := database.GetDB(), context.Background()
	shopID := c.Query("shopId")
	if claims.Role == "staff" {
		shopID, err = getShopIDFromStaffID(ctx, db, claims.UserID)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"status": "error", "message": "Assigned shop not found"})
		}
	} else if claims.Role == "merchant" {
		if shopID == "" {
			return c.Status(400).JSON(fiber.Map{"status": "error", "message": "shopId is required"})
		}
		if err = authorizeShopAccess(c, shopID); err != nil {
			return err
		}
	} else {
		return c.Status(403).JSON(fiber.Map{"status": "error", "message": "Shop access denied"})
	}
	page := c.QueryInt("page", 1)
	size := c.QueryInt("pageSize", 20)
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	where := " WHERE ii.shop_id=$1"
	args := []interface{}{shopID}
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		where += fmt.Sprintf(" AND (si.name ILIKE $%d OR COALESCE(si.sku,'') ILIKE $%d)", len(args)+1, len(args)+1)
		args = append(args, "%"+search+"%")
	}
	if categoryID := strings.TrimSpace(c.Query("categoryId")); categoryID != "" {
		where += fmt.Sprintf(" AND EXISTS(SELECT 1 FROM product_categories pc WHERE pc.product_id=si.product_id AND pc.category_id=$%d)", len(args)+1)
		args = append(args, categoryID)
	}
	var total int
	if err = db.QueryRow(ctx, `SELECT COUNT(*) FROM inventory_items ii JOIN stock_items si ON si.id=ii.stock_item_id`+where, args...).Scan(&total); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to count shop inventory"})
	}
	query := `SELECT ii.id,si.id,si.name,COALESCE(si.sku,''),ii.quantity_on_hand,COALESCE(pp.selling_price,0) FROM inventory_items ii JOIN stock_items si ON si.id=ii.stock_item_id LEFT JOIN LATERAL(SELECT selling_price FROM product_prices WHERE product_id=si.product_id AND shop_id IS NULL AND price_type='RETAIL' ORDER BY created_at DESC LIMIT 1)pp ON TRUE` + where + fmt.Sprintf(" ORDER BY si.name, si.id LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, size, (page-1)*size)
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve shop inventory"})
	}
	defer rows.Close()
	items := make([]models.ShopInventoryItem, 0)
	for rows.Next() {
		var i models.ShopInventoryItem
		var q float64
		if err = rows.Scan(&i.ID, &i.ProductID, &i.Name, &i.SKU, &q, &i.SellingPrice); err == nil {
			i.Quantity = int(q)
			items = append(items, i)
		}
	}
	return c.JSON(fiber.Map{"status": "success", "data": items, "pagination": fiber.Map{"totalItems": total, "totalPages": (total + size - 1) / size, "currentPage": page, "pageSize": size}})
}

func HandleStockIn(c *fiber.Ctx) error {
	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	db, ctx := database.GetDB(), context.Background()
	shopID := c.Query("shopId")
	operationType := "merchant_stock_in"
	merchantID := claims.UserID
	if claims.Role == "staff" {
		shopID, err = getShopIDFromStaffID(ctx, db, claims.UserID)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"status": "error", "message": "Assigned shop not found"})
		}
		operationType = "staff_stock_in"
		if err = db.QueryRow(ctx, `SELECT merchant_id FROM shops WHERE id=$1`, shopID).Scan(&merchantID); err != nil {
			return c.Status(404).JSON(fiber.Map{"status": "error", "message": "Shop not found"})
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
	return handleBulkStockIn(c, shopID, merchantID, operationType)
}
