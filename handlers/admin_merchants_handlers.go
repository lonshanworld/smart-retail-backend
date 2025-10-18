package handlers

import (
	"app/database"
	"app/models"
	"database/sql"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// HandleDeleteUserMerchant permanently deletes a user from the database.
// DELETE /api/v1/admin/users/:userId
func HandleDeleteUserMerchant(c *fiber.Ctx) error {
	userID := c.Params("userId")
	db := database.GetDB()

	query := "DELETE FROM users WHERE id = $1"
	result, err := db.Exec(query, userID)
	if err != nil {
		log.Printf("Error deleting user %s: %v", userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to delete user"})
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Error getting rows affected for user deletion %s: %v", userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to confirm deletion"})
	}

	if rowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "User not found"})
	}

	return c.JSON(fiber.Map{"status": "success", "message": fmt.Sprintf("User %s deleted successfully", userID)})
}

// HandleSetMerchantUserActiveStatus sets the active status of a user.
// PUT /api/v1/admin/users/:userId/status
func HandleSetMerchantUserActiveStatus(c *fiber.Ctx) error {
	userID := c.Params("userId")
	db := database.GetDB()

	var body struct {
		IsActive bool `json:"is_active"`
	}

	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid JSON"})
	}

	query := "UPDATE users SET is_active = $1, updated_at = NOW() WHERE id = $2"
	result, err := db.Exec(query, body.IsActive, userID)
	if err != nil {
		log.Printf("Error updating user active status for user %s: %v", userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update user status"})
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Error getting rows affected for user status update %s: %v", userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to confirm update"})
	}

	if rowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "User not found"})
	}

	return c.JSON(fiber.Map{"status": "success", "message": fmt.Sprintf("User %s status updated successfully to %v", userID, body.IsActive)})
}

// HandleUpdateUserMerchantDetails allows an admin to update another user's details.
// PUT /api/v1/admin/users/:userId
func HandleUpdateUserMerchantDetails(c *fiber.Ctx) error {
	userID := c.Params("userId")
	db := database.GetDB()

	var updates map[string]interface{}
	if err := c.BodyParser(&updates); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid JSON"})
	}

	// Dynamically build the SET part of the query
	var setParts []string
	var args []interface{}
	argId := 1

	for key, value := range updates {
		// Whitelist columns that can be updated
		switch key {
		case "name", "email", "shop_name", "is_active":
			setParts = append(setParts, fmt.Sprintf("%s = $%d", key, argId))
			args = append(args, value)
			argId++
		default:
			// Ignore other fields to prevent updating restricted columns
			log.Printf("Attempted to update non-whitelisted field: %s", key)
		}
	}

	if len(setParts) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "No updatable fields provided"})
	}

	query := fmt.Sprintf("UPDATE users SET %s, updated_at = NOW() WHERE id = $%d RETURNING id, name, email, role, is_active, phone, assigned_shop_id, merchant_id, created_at, updated_at", strings.Join(setParts, ", "), argId)
	args = append(args, userID)

	var updatedUser models.User
	var phone, assignedShopID, merchantID sql.NullString

	err := db.QueryRow(query, args...).Scan(
		&updatedUser.ID, &updatedUser.Name, &updatedUser.Email, &updatedUser.Role, &updatedUser.IsActive,
		&phone, &assignedShopID, &merchantID,
		&updatedUser.CreatedAt, &updatedUser.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "User not found"})
		}
		log.Printf("Error updating user %s: %v", userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update user"})
	}

	if phone.Valid {
		updatedUser.Phone = &phone.String
	}
	if assignedShopID.Valid {
		updatedUser.AssignedShopID = &assignedShopID.String
	}
	if merchantID.Valid {
		updatedUser.MerchantID = &merchantID.String
	}

	return c.JSON(fiber.Map{"status": "success", "data": updatedUser})
}


// HandleGetMerchantByID fetches a single merchant by their ID.
// GET /api/v1/admin/merchants/:merchantIdOrUserId
func HandleGetMerchantByID(c *fiber.Ctx) error {
	db := database.GetDB()
	merchantID := c.Params("merchantIdOrUserId")

	query := "SELECT id, name, email, is_active, created_at, updated_at FROM users WHERE id = $1 AND role = 'merchant'"
	row := db.QueryRow(query, merchantID)

	var m models.Merchant
	if err := row.Scan(&m.ID, &m.Name, &m.Email, &m.IsActive, &m.CreatedAt, &m.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Merchant not found"})
		}
		log.Printf("Error scanning merchant row: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}

	return c.JSON(fiber.Map{"status": "success", "data": m})
}

// HandleListMerchants fetches a paginated and filtered list of merchants.
// GET /api/v1/admin/merchants
func HandleListMerchants(c *fiber.Ctx) error {
	db := database.GetDB()

	// --- Pagination Parameters ---
	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize", "10"))
	offset := (page - 1) * pageSize

	// --- Filtering Parameters ---
	nameFilter := c.Query("name")
	emailFilter := c.Query("email")
	isActiveFilter := c.Query("isActive")
	userIdFilter := c.Query("userId")

	// --- Build Query ---
	var args []interface{}
	baseQuery := "FROM users WHERE role = 'merchant'"
	conditions := []string{}
	argId := 1

	if nameFilter != "" {
		conditions = append(conditions, fmt.Sprintf("name ILIKE $%d", argId))
		args = append(args, "%"+nameFilter+"%")
		argId++
	}
	if emailFilter != "" {
		conditions = append(conditions, fmt.Sprintf("email ILIKE $%d", argId))
		args = append(args, "%"+emailFilter+"%")
		argId++
	}
	if isActiveFilter != "" {
		conditions = append(conditions, fmt.Sprintf("is_active = $%d", argId))
		args = append(args, isActiveFilter)
		argId++
	}
	if userIdFilter != "" {
		conditions = append(conditions, fmt.Sprintf("id = $%d", argId))
		args = append(args, userIdFilter)
		argId++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " AND " + strings.Join(conditions, " AND ")
	}

	// --- Get Total Count ---
	var totalItems int
	countQuery := "SELECT COUNT(*) " + baseQuery + whereClause
	if err := db.QueryRow(countQuery, args...).Scan(&totalItems); err != nil {
		log.Printf("Error counting merchants: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to count merchants"})
	}

	// --- Get Paginated Data ---
	query := "SELECT id, name, email, is_active, created_at, updated_at " + baseQuery + whereClause + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argId, argId+1)
	args = append(args, pageSize, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("Error querying for merchants: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve merchants"})
	}
	defer rows.Close()

	merchants := []models.Merchant{}
	for rows.Next() {
		var m models.Merchant
		if err := rows.Scan(&m.ID, &m.Name, &m.Email, &m.IsActive, &m.CreatedAt, &m.UpdatedAt); err != nil {
			log.Printf("Error scanning merchant row: %v", err)
			continue
		}
		merchants = append(merchants, m)
	}

	if err := rows.Err(); err != nil {
        log.Printf("Error after iterating over merchant rows: %v", err)
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Error processing merchant data"})
    }

	// --- Construct Response ---
	pagination := models.PaginationInfo{
		TotalItems:  totalItems,
		TotalPages:  int(math.Ceil(float64(totalItems) / float64(pageSize))),
		CurrentPage: page,
		PageSize:    pageSize,
	}

	response := models.PaginatedAdminMerchantsResponse{
		Merchants:  merchants,
		Pagination: pagination,
	}

	return c.JSON(fiber.Map{"status": "success", "data": response})
}
