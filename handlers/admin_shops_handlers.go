package handlers

import (
	"app/database"
	"app/models"
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// HandleDeleteShop permanently deletes a shop.
// DELETE /api/v1/admin/shops/:shopId
func HandleDeleteShop(c *fiber.Ctx) error {
	shopID := c.Params("shopId")
	db := database.GetDB()
	ctx := context.Background()

	query := "DELETE FROM shops WHERE id = $1"
	result, err := db.Exec(ctx, query, shopID)
	if err != nil {
		log.Printf("Error deleting shop %s: %v", shopID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to delete shop"})
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Shop not found"})
	}

	return c.JSON(fiber.Map{"status": "success", "message": fmt.Sprintf("Shop %s deleted successfully", shopID)})
}

// HandleSetShopActiveStatus sets the active status of a shop.
// PUT /api/v1/admin/shops/:shopId/status
func HandleSetShopActiveStatus(c *fiber.Ctx) error {
	shopID := c.Params("shopId")
	db := database.GetDB()
	ctx := context.Background()

	var body struct {
		IsActive bool `json:"is_active"`
	}

	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid JSON"})
	}

	query := "UPDATE shops SET is_active = $1, updated_at = NOW() WHERE id = $2"
	result, err := db.Exec(ctx, query, body.IsActive, shopID)
	if err != nil {
		log.Printf("Error updating shop active status for shop %s: %v", shopID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update shop status"})
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Shop not found"})
	}

	return c.JSON(fiber.Map{"status": "success", "message": fmt.Sprintf("Shop %s status updated successfully to %v", shopID, body.IsActive)})
}

// HandleUpdateShop updates a shop's details.
// PUT /api/v1/admin/shops/:shopId
func HandleUpdateShop(c *fiber.Ctx) error {
	shopID := c.Params("shopId")
	db := database.GetDB()
	ctx := context.Background()

	var updates map[string]interface{}
	if err := c.BodyParser(&updates); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid JSON"})
	}

	var setParts []string
	var args []interface{}
	argId := 1

	for key, value := range updates {
		switch key {
		case "name", "merchant_id", "address", "phone", "is_active", "is_primary":
			setParts = append(setParts, fmt.Sprintf("%s = $%d", key, argId))
			args = append(args, value)
			argId++
		default:
			log.Printf("Attempted to update non-whitelisted shop field: %s", key)
		}
	}

	if len(setParts) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "No updatable fields provided"})
	}

	query := fmt.Sprintf("UPDATE shops SET %s, updated_at = NOW() WHERE id = $%d RETURNING id, name, merchant_id, address, phone, is_active, is_primary, created_at, updated_at", strings.Join(setParts, ", "), argId)
	args = append(args, shopID)

	var updatedShop models.Shop
	var address, phone sql.NullString

	err := db.QueryRow(ctx, query, args...).Scan(
		&updatedShop.ID, &updatedShop.Name, &updatedShop.MerchantID, &address, &phone, &updatedShop.IsActive, &updatedShop.IsPrimary, &updatedShop.CreatedAt, &updatedShop.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Shop not found"})
		}
		log.Printf("Error updating shop %s: %v", shopID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update shop"})
	}

	if address.Valid {
		updatedShop.Address = &address.String
	}
	if phone.Valid {
		updatedShop.Phone = &phone.String
	}

	return c.JSON(fiber.Map{"status": "success", "data": updatedShop})
}

// HandleCreateShop creates a new shop.
// POST /api/v1/admin/shops
func HandleCreateShop(c *fiber.Ctx) error {
	// Log raw body for debugging
	log.Printf("Raw request body: %s", string(c.Body()))

	var req models.CreateShopRequest
	if err := c.BodyParser(&req); err != nil {
		log.Printf("Error parsing body: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Cannot parse JSON"})
	}

	log.Printf("Create shop request: name=%s, merchantId=%s, address=%v, phone=%v, isActive=%v, isPrimary=%v",
		req.Name, req.MerchantID, req.Address, req.Phone, req.IsActive, req.IsPrimary)

	// Convert empty strings to nil for optional fields
	var addressVal interface{}
	if req.Address != nil && *req.Address != "" {
		addressVal = *req.Address
	} else {
		addressVal = nil
	}

	var phoneVal interface{}
	if req.Phone != nil && *req.Phone != "" {
		phoneVal = *req.Phone
	} else {
		phoneVal = nil
	}

	db := database.GetDB()
	ctx := context.Background()
	query := `
        INSERT INTO shops (name, merchant_id, address, phone, is_active, is_primary)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id, name, merchant_id, address, phone, is_active, is_primary, created_at, updated_at
    `

	var newShop models.Shop
	var address, phone sql.NullString

	err := db.QueryRow(ctx, query, req.Name, req.MerchantID, addressVal, phoneVal, req.IsActive, req.IsPrimary).Scan(
		&newShop.ID, &newShop.Name, &newShop.MerchantID, &address, &phone, &newShop.IsActive, &newShop.IsPrimary, &newShop.CreatedAt, &newShop.UpdatedAt,
	)

	if err != nil {
		log.Printf("Error creating shop: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create shop"})
	}

	if address.Valid {
		newShop.Address = &address.String
	}
	if phone.Valid {
		newShop.Phone = &phone.String
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "data": newShop})
}

// HandleGetShopByID fetches a single shop by its ID.
// GET /api/v1/admin/shops/:shopId
func HandleGetShopByID(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()
	shopID := c.Params("shopId")

	query := "SELECT id, name, merchant_id, address, phone, is_active, is_primary, created_at, updated_at FROM shops WHERE id = $1"
	row := db.QueryRow(ctx, query, shopID)

	var s models.Shop
	var address, phone sql.NullString
	if err := row.Scan(&s.ID, &s.Name, &s.MerchantID, &address, &phone, &s.IsActive, &s.IsPrimary, &s.CreatedAt, &s.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Shop not found"})
		}
		log.Printf("Error scanning shop row: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}

	if address.Valid {
		s.Address = &address.String
	}
	if phone.Valid {
		s.Phone = &phone.String
	}

	return c.JSON(fiber.Map{"status": "success", "data": s})
}

// HandleListShops fetches a paginated and filtered list of shops for the admin.
// GET /api/v1/admin/shops
func HandleListShops(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	// --- Pagination Parameters ---
	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize", "10"))
	offset := (page - 1) * pageSize

	// --- Filtering Parameters ---
	nameFilter := c.Query("name")
	merchantIDFilter := c.Query("merchant_id")
	isActiveFilter := c.Query("is_active")

	// --- Build Query ---
	var args []interface{}
	baseQuery := "FROM shops"
	conditions := []string{}
	argId := 1

	if nameFilter != "" {
		conditions = append(conditions, fmt.Sprintf("name ILIKE $%d", argId))
		args = append(args, "%"+nameFilter+"%")
		argId++
	}
	if merchantIDFilter != "" {
		conditions = append(conditions, fmt.Sprintf("merchant_id = $%d", argId))
		args = append(args, merchantIDFilter)
		argId++
	}
	if isActiveFilter != "" {
		conditions = append(conditions, fmt.Sprintf("is_active = $%d", argId))
		args = append(args, isActiveFilter)
		argId++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	// --- Get Total Count ---
	var totalItems int
	countQuery := "SELECT COUNT(*) " + baseQuery + whereClause
	if err := db.QueryRow(ctx, countQuery, args...).Scan(&totalItems); err != nil {
		log.Printf("Error counting shops: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to count shops"})
	}

	// --- Get Paginated Data ---
	query := "SELECT id, name, merchant_id, address, phone, is_active, is_primary, created_at, updated_at " + baseQuery + whereClause + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argId, argId+1)
	args = append(args, pageSize, offset)

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		log.Printf("Error querying for shops: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve shops"})
	}
	defer rows.Close()

	shops := []models.Shop{}
	for rows.Next() {
		var s models.Shop
		var address, phone sql.NullString
		if err := rows.Scan(&s.ID, &s.Name, &s.MerchantID, &address, &phone, &s.IsActive, &s.IsPrimary, &s.CreatedAt, &s.UpdatedAt); err != nil {
			log.Printf("Error scanning shop row: %v", err)
			continue
		}
		if address.Valid {
			s.Address = &address.String
		}
		if phone.Valid {
			s.Phone = &phone.String
		}
		shops = append(shops, s)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Error after iterating over shop rows: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Error processing shop data"})
	}

	// --- Construct Response ---
	pagination := models.Pagination{
		TotalItems:  totalItems,
		TotalPages:  int(math.Ceil(float64(totalItems) / float64(pageSize))),
		CurrentPage: page,
		PageSize:    pageSize,
	}

	response := models.PaginatedAdminShopsResponse{
		Data:       shops,
		Pagination: pagination,
	}

	return c.JSON(fiber.Map{"status": "success", "data": response})
}
