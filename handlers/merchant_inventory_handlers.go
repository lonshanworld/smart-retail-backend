package handlers

import (
	"app/database"
	"app/models"
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

// HandleListInventoryItems handles listing all inventory items for a merchant with pagination and filtering
func HandleListInventoryItems(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantId := claims["userId"].(string)

	// Pagination and filtering parameters
	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("pageSize", 10)
	nameFilter := c.Query("name")

	offset := (page - 1) * pageSize

	// --- Build Query --- //
	var queryBuilder, countQueryBuilder strings.Builder
	args := []interface{}{merchantId}
	paramIndex := 2

	queryBuilder.WriteString("SELECT id, merchant_id, name, description, sku, selling_price, original_price, low_stock_threshold, category, supplier_id, is_archived, created_at, updated_at FROM merchant_inventory WHERE merchant_id = $1 ")
	countQueryBuilder.WriteString("SELECT COUNT(*) FROM merchant_inventory WHERE merchant_id = $1 ")

	if nameFilter != "" {
		queryBuilder.WriteString(fmt.Sprintf("AND name ILIKE $%d ", paramIndex))
		countQueryBuilder.WriteString(fmt.Sprintf("AND name ILIKE $%d ", paramIndex))
		args = append(args, "%"+nameFilter+"%")
		paramIndex++
	}

	// --- Get Total Count --- //
	var totalItems int
	err := db.QueryRow(ctx, countQueryBuilder.String(), args...).Scan(&totalItems)
	if err != nil {
		log.Printf("Error counting inventory items: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}

	// --- Add pagination and get items --- //
	queryBuilder.WriteString(fmt.Sprintf("ORDER BY created_at DESC LIMIT $%d OFFSET $%d", paramIndex, paramIndex+1))
	args = append(args, pageSize, offset)

	rows, err := db.Query(ctx, queryBuilder.String(), args...)
	if err != nil {
		log.Printf("Error listing inventory items: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}
	defer rows.Close()

	items := make([]models.InventoryItem, 0)
	for rows.Next() {
		var item models.InventoryItem
		var description, sku, category, supplierId sql.NullString
		var lowStockThreshold sql.NullInt32

		err := rows.Scan(&item.ID, &item.MerchantID, &item.Name, &description, &sku, &item.SellingPrice, &item.OriginalPrice, &lowStockThreshold, &category, &supplierId, &item.IsArchived, &item.CreatedAt, &item.UpdatedAt)
		if err != nil {
			log.Printf("Error scanning inventory item: %v", err)
			continue
		}
		if description.Valid {
			item.Description = &description.String
		}
		if sku.Valid {
			item.SKU = &sku.String
		}
		if category.Valid {
			item.Category = &category.String
		}
		if supplierId.Valid {
			item.SupplierID = &supplierId.String
		}
		if lowStockThreshold.Valid {
			val := int(lowStockThreshold.Int32)
			item.LowStockThreshold = &val
		}

		items = append(items, item)
	}

	// --- Construct Response --- //
	totalPages := int(math.Ceil(float64(totalItems) / float64(pageSize)))

	return c.JSON(fiber.Map{
		"status": "success",
		"data": fiber.Map{
			"items":       items,
			"totalItems":  totalItems,
			"currentPage": page,
			"pageSize":    pageSize,
			"totalPages":  totalPages,
		},
	})
}



// HandleCreateInventoryItem handles creating a new inventory item
func HandleCreateInventoryItem(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantId := claims["userId"].(string)

	var item models.InventoryItem
	if err := c.BodyParser(&item); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid JSON"})
	}

	item.MerchantID = merchantId

	query := `INSERT INTO merchant_inventory (merchant_id, name, description, sku, selling_price, original_price, low_stock_threshold, category, supplier_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id, created_at, updated_at`

	err := db.QueryRow(ctx, query, item.MerchantID, item.Name, item.Description, item.SKU, item.SellingPrice, item.OriginalPrice, item.LowStockThreshold, item.Category, item.SupplierID).Scan(&item.ID, &item.CreatedAt, &item.UpdatedAt)

	if err != nil {
		log.Printf("Error creating inventory item: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "data": item})
}

// HandleGetInventoryItemByID handles getting a single inventory item by ID
func HandleGetInventoryItemByID(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantId := claims["userId"].(string)

	itemId := c.Params("itemId")

	var item models.InventoryItem
	var description, sku, category, supplierId sql.NullString
	var lowStockThreshold sql.NullInt32

	query := `SELECT id, merchant_id, name, description, sku, selling_price, original_price, low_stock_threshold, category, supplier_id, is_archived, created_at, updated_at FROM merchant_inventory WHERE id = $1 AND merchant_id = $2`

	err := db.QueryRow(ctx, query, itemId, merchantId).Scan(&item.ID, &item.MerchantID, &item.Name, &description, &sku, &item.SellingPrice, &item.OriginalPrice, &lowStockThreshold, &category, &supplierId, &item.IsArchived, &item.CreatedAt, &item.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Item not found"})
		}
		log.Printf("Error getting inventory item: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}

	if description.Valid {
		item.Description = &description.String
	}
	if sku.Valid {
		item.SKU = &sku.String
	}
	if category.Valid {
		item.Category = &category.String
	}
	if supplierId.Valid {
		item.SupplierID = &supplierId.String
	}
	if lowStockThreshold.Valid {
		val := int(lowStockThreshold.Int32)
		item.LowStockThreshold = &val
	}

	return c.JSON(fiber.Map{"status": "success", "data": item})

}

// HandleUpdateInventoryItem handles updating an inventory item
func HandleUpdateInventoryItem(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantId := claims["userId"].(string)

	itemId := c.Params("itemId")

	var item models.InventoryItem
	if err := c.BodyParser(&item); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid JSON"})
	}

	query := `UPDATE merchant_inventory SET name = $1, description = $2, sku = $3, selling_price = $4, original_price = $5, low_stock_threshold = $6, category = $7, supplier_id = $8, updated_at = NOW() WHERE id = $9 AND merchant_id = $10 RETURNING id, merchant_id, name, description, sku, selling_price, original_price, low_stock_threshold, category, supplier_id, is_archived, created_at, updated_at`

	var updatedItem models.InventoryItem
	var description, sku, category, supplierId sql.NullString
	var lowStockThreshold sql.NullInt32

	err := db.QueryRow(ctx, query, item.Name, item.Description, item.SKU, item.SellingPrice, item.OriginalPrice, item.LowStockThreshold, item.Category, item.SupplierID, itemId, merchantId).Scan(&updatedItem.ID, &updatedItem.MerchantID, &updatedItem.Name, &description, &sku, &updatedItem.SellingPrice, &updatedItem.OriginalPrice, &lowStockThreshold, &category, &supplierId, &updatedItem.IsArchived, &updatedItem.CreatedAt, &updatedItem.UpdatedAt)

	if err != nil {
		log.Printf("Error updating inventory item: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}

	if description.Valid {
		updatedItem.Description = &description.String
	}
	if sku.Valid {
		updatedItem.SKU = &sku.String
	}
	if category.Valid {
		updatedItem.Category = &category.String
	}
	if supplierId.Valid {
		updatedItem.SupplierID = &supplierId.String
	}
	if lowStockThreshold.Valid {
		val := int(lowStockThreshold.Int32)
		updatedItem.LowStockThreshold = &val
	}

	return c.JSON(fiber.Map{"status": "success", "data": updatedItem})
}

// HandleDeleteInventoryItem handles deleting an inventory item
func HandleDeleteInventoryItem(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantId := claims["userId"].(string)

	itemId := c.Params("itemId")

	query := `DELETE FROM merchant_inventory WHERE id = $1 AND merchant_id = $2`

	_, err := db.Exec(ctx, query, itemId, merchantId)
	if err != nil {
		log.Printf("Error deleting inventory item: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}

	return c.JSON(fiber.Map{"status": "success"})
}

// HandleArchiveInventoryItem handles archiving an inventory item
func HandleArchiveInventoryItem(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantId := claims["userId"].(string)

	itemId := c.Params("itemId")

	query := `UPDATE merchant_inventory SET is_archived = TRUE, updated_at = NOW() WHERE id = $1 AND merchant_id = $2 RETURNING id, merchant_id, name, description, sku, selling_price, original_price, low_stock_threshold, category, supplier_id, is_archived, created_at, updated_at`

	var item models.InventoryItem
	var description, sku, category, supplierId sql.NullString
	var lowStockThreshold sql.NullInt32

	err := db.QueryRow(ctx, query, itemId, merchantId).Scan(&item.ID, &item.MerchantID, &item.Name, &description, &sku, &item.SellingPrice, &item.OriginalPrice, &lowStockThreshold, &category, &supplierId, &item.IsArchived, &item.CreatedAt, &item.UpdatedAt)

	if err != nil {
		log.Printf("Error archiving inventory item: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}

	if description.Valid {
		item.Description = &description.String
	}
	if sku.Valid {
		item.SKU = &sku.String
	}
	if category.Valid {
		item.Category = &category.String
	}
	if supplierId.Valid {
		item.SupplierID = &supplierId.String
	}
	if lowStockThreshold.Valid {
		val := int(lowStockThreshold.Int32)
		item.LowStockThreshold = &val
	}

	return c.JSON(fiber.Map{"status": "success", "data": item})
}

// HandleUnarchiveInventoryItem handles unarchiving an inventory item
func HandleUnarchiveInventoryItem(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantId := claims["userId"].(string)

	itemId := c.Params("itemId")

	query := `UPDATE merchant_inventory SET is_archived = FALSE, updated_at = NOW() WHERE id = $1 AND merchant_id = $2 RETURNING id, merchant_id, name, description, sku, selling_price, original_price, low_stock_threshold, category, supplier_id, is_archived, created_at, updated_at`

	var item models.InventoryItem
	var description, sku, category, supplierId sql.NullString
	var lowStockThreshold sql.NullInt32

	err := db.QueryRow(ctx, query, itemId, merchantId).Scan(&item.ID, &item.MerchantID, &item.Name, &description, &sku, &item.SellingPrice, &item.OriginalPrice, &lowStockThreshold, &category, &supplierId, &item.IsArchived, &item.CreatedAt, &item.UpdatedAt)

	if err != nil {
		log.Printf("Error unarchiving inventory item: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}

	if description.Valid {
		item.Description = &description.String
	}
	if sku.Valid {
		item.SKU = &sku.String
	}
	if category.Valid {
		item.Category = &category.String
	}
	if supplierId.Valid {
		item.SupplierID = &supplierId.String
	}
	if lowStockThreshold.Valid {
		val := int(lowStockThreshold.Int32)
		item.LowStockThreshold = &val
	}

	return c.JSON(fiber.Map{"status": "success", "data": item})
}
