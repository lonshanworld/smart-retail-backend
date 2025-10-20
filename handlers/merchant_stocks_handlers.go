package handlers

import (
	"app/database"
	"app/models"
	"context"
	"log"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

// HandleGetCombinedStocks handles fetching a combined, paginated list of all inventory items from all shops.
func HandleGetCombinedStocks(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantId := claims["userId"].(string)

	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize", "20"))
	searchTerm := c.Query("searchTerm")
	offset := (page - 1) * pageSize

	// Base query
	query := `
		SELECT
			i.id, i.name, i.sku, ss.quantity, i.selling_price, i.original_price, s.name as shop_name, s.id as shop_id
		FROM inventory_items i
		JOIN shop_stock ss ON i.id = ss.inventory_item_id
		JOIN shops s ON ss.shop_id = s.id
		WHERE i.merchant_id = $1
	`
	countQuery := `SELECT COUNT(*) FROM inventory_items i JOIN shop_stock ss ON i.id = ss.inventory_item_id JOIN shops s ON ss.shop_id = s.id WHERE i.merchant_id = $1`

	args := []interface{}{merchantId}
	countArgs := []interface{}{merchantId}

	// Add search term if provided
	if searchTerm != "" {
		query += " AND i.name ILIKE $2"
		countQuery += " AND i.name ILIKE $2"
		args = append(args, "%"+searchTerm+"%")
		countArgs = append(countArgs, "%"+searchTerm+"%")
	}

	// Get total count
	var totalCount int
	err := db.QueryRow(ctx, countQuery, countArgs...).Scan(&totalCount)
	if err != nil {
		log.Printf("Error counting combined stocks: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}

	// Add pagination to the main query
	pagingArgs := []interface{}{pageSize, offset}
	query += " ORDER BY i.name, s.name LIMIT $" + strconv.Itoa(len(args)+1) + " OFFSET $" + strconv.Itoa(len(args)+2)
	args = append(args, pagingArgs...)

	// Execute the main query
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		log.Printf("Error fetching combined stocks: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}
	defer rows.Close()

	items := make([]models.CombinedStockItem, 0)
	for rows.Next() {
		var item models.CombinedStockItem
		if err := rows.Scan(&item.ID, &item.Name, &item.SKU, &item.Quantity, &item.SellingPrice, &item.OriginalPrice, &item.ShopName, &item.ShopID); err != nil {
			log.Printf("Error scanning combined stock item: %v", err)
			continue
		}
		items = append(items, item)
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data": fiber.Map{
			"items":      items,
			"totalItems": totalCount,
		},
	})
}
