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

// HandleListMerchants retrieves a paginated and filtered list of merchants (users with the 'merchant' role).
func HandleListMerchants(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	// --- Query Parameters ---
	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize", "10"))
	nameFilter := c.Query("name")
	emailFilter := c.Query("email")
	isActiveFilter := c.Query("isActive")
	userIdFilter := c.Query("userId")

	// --- Building the Query ---
	var whereClauses []string
	var args []interface{}
	argCount := 1

	// Always filter by role = 'merchant'
	whereClauses = append(whereClauses, fmt.Sprintf("u.role = $%d", argCount))
	args = append(args, "merchant")
	argCount++

	if nameFilter != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("u.name ILIKE $%d", argCount))
		args = append(args, "%"+nameFilter+"%")
		argCount++
	}
	if emailFilter != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("u.email ILIKE $%d", argCount))
		args = append(args, "%"+emailFilter+"%")
		argCount++
	}
	if isActiveFilter != "" {
		if isActive, err := strconv.ParseBool(isActiveFilter); err == nil {
			whereClauses = append(whereClauses, fmt.Sprintf("u.is_active = $%d", argCount))
			args = append(args, isActive)
			argCount++
		}
	}
	if userIdFilter != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("u.id = $%d", argCount))
		args = append(args, userIdFilter)
		argCount++
	}

	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// --- Count Query ---
	countQuery := "SELECT COUNT(DISTINCT u.id) FROM users u " + whereClause
	var totalItems int
	if err := db.QueryRow(ctx, countQuery, args...).Scan(&totalItems); err != nil {
		log.Printf("Error counting merchants: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to count merchants"})
	}

	// --- Main Query ---
	offset := (page - 1) * pageSize
	query := fmt.Sprintf(`
        SELECT u.id, u.name, u.email, u.is_active, s.name as shop_name, u.created_at, u.updated_at
        FROM users u
        LEFT JOIN shops s ON u.id = s.merchant_id AND s.is_primary = true
        %s
        ORDER BY u.created_at DESC
        LIMIT $%d OFFSET $%d
    `, whereClause, argCount, argCount+1)

	finalArgs := append(args, pageSize, offset)
	rows, err := db.Query(ctx, query, finalArgs...)
	if err != nil {
		log.Printf("Error fetching merchants: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to fetch merchants"})
	}
	defer rows.Close()

	merchants := make([]models.Merchant, 0)
	for rows.Next() {
		var merchant models.Merchant
		var shopName sql.NullString
		if err := rows.Scan(&merchant.ID, &merchant.Name, &merchant.Email, &merchant.IsActive, &shopName, &merchant.CreatedAt, &merchant.UpdatedAt); err != nil {
			log.Printf("Error scanning merchant row: %v", err)
			continue // Or handle error more gracefully
		}
		if shopName.Valid {
			merchant.ShopName = &shopName.String
		}
		merchants = append(merchants, merchant)
	}

	// --- Response ---
	pagination := models.Pagination{
		TotalItems:  totalItems,
		TotalPages:  int(math.Ceil(float64(totalItems) / float64(pageSize))),
		CurrentPage: page,
		PageSize:    pageSize,
	}

	return c.JSON(models.PaginatedAdminMerchantsResponse{
		Data:       merchants,
		Pagination: pagination,
	})
}

// HandleGetMerchantByID retrieves details for a single merchant, including all their associated shops.
func HandleGetMerchantByID(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()
	merchantIdOrUserId := c.Params("merchantIdOrUserId")

	// First, get the merchant user details
	userQuery := `
        SELECT id, name, email, phone, is_active, created_at, updated_at
        FROM users
        WHERE id = $1 AND role = 'merchant'
    `

	var merchant models.Merchant
	var phone sql.NullString

	err := db.QueryRow(ctx, userQuery, merchantIdOrUserId).Scan(
		&merchant.ID, &merchant.Name, &merchant.Email, &phone,
		&merchant.IsActive, &merchant.CreatedAt, &merchant.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Merchant not found"})
		}
		log.Printf("Error fetching merchant by ID %s: %v", merchantIdOrUserId, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}

	if phone.Valid {
		merchant.Phone = &phone.String
	}

	// Now fetch all shops for this merchant
	shopsQuery := `
        SELECT id, merchant_id, name, address, phone, is_active, is_primary, created_at, updated_at
        FROM shops
        WHERE merchant_id = $1
        ORDER BY is_primary DESC, created_at ASC
    `

	rows, err := db.Query(ctx, shopsQuery, merchantIdOrUserId)
	if err != nil {
		log.Printf("Error fetching shops for merchant %s: %v", merchantIdOrUserId, err)
		// Return merchant without shops but with proper structure
		return c.JSON(fiber.Map{
			"status": "success",
			"data": fiber.Map{
				"id":        merchant.ID,
				"name":      merchant.Name,
				"email":     merchant.Email,
				"phone":     merchant.Phone,
				"isActive":  merchant.IsActive,
				"shopName":  merchant.ShopName,
				"createdAt": merchant.CreatedAt,
				"updatedAt": merchant.UpdatedAt,
				"shops":     []models.Shop{}, // Empty array instead of nil
			},
		})
	}
	defer rows.Close()

	var shops []models.Shop
	for rows.Next() {
		var shop models.Shop
		var address sql.NullString
		var phoneShop sql.NullString

		err := rows.Scan(
			&shop.ID, &shop.MerchantID, &shop.Name, &address, &phoneShop,
			&shop.IsActive, &shop.IsPrimary, &shop.CreatedAt, &shop.UpdatedAt,
		)
		if err != nil {
			log.Printf("Error scanning shop row: %v", err)
			continue
		}

		if address.Valid {
			shop.Address = &address.String
		}
		if phoneShop.Valid {
			shop.Phone = &phoneShop.String
		}

		shops = append(shops, shop)
	}

	// Ensure shops is not nil
	if shops == nil {
		shops = []models.Shop{}
	}

	// For backward compatibility, set shopName to primary shop name
	if len(shops) > 0 {
		for _, shop := range shops {
			if shop.IsPrimary {
				merchant.ShopName = &shop.Name
				break
			}
		}
		// If no primary shop, use first shop
		if merchant.ShopName == nil {
			merchant.ShopName = &shops[0].Name
		}
	}

	log.Printf("Returning merchant %s with %d shops", merchantIdOrUserId, len(shops))

	// Construct response
	responseData := fiber.Map{
		"id":        merchant.ID,
		"name":      merchant.Name,
		"email":     merchant.Email,
		"phone":     merchant.Phone,
		"isActive":  merchant.IsActive,
		"shopName":  merchant.ShopName,
		"createdAt": merchant.CreatedAt,
		"updatedAt": merchant.UpdatedAt,
		"shops":     shops,
	}

	log.Printf("Response data keys: %v", getMapKeys(responseData))
	log.Printf("Shops in response: %+v", shops)

	// Return merchant with shops array
	return c.JSON(fiber.Map{
		"status": "success",
		"data":   responseData,
	})
}

// Helper function to get map keys for debugging
func getMapKeys(m fiber.Map) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
