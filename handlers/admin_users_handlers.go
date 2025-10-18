package handlers

import (
	"context"
	"app/database"
	"app/models"
	"database/sql"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

// HandleGetUsers fetches a paginated and filtered list of users.
// GET /api/v1/admin/users
func HandleGetUsers(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	// --- Query Parameters ---
	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize", "10"))
	role := c.Query("role")
	isActiveStr := c.Query("is_active")
	searchTerm := c.Query("q")

	// --- Building the Query ---
	var whereClauses []string
	var args []interface{}
	argCount := 1

	if role != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("role = $%d", argCount))
		args = append(args, role)
		argCount++
	}
	if isActiveStr != "" {
		isActive, err := strconv.ParseBool(isActiveStr)
		if err == nil {
			whereClauses = append(whereClauses, fmt.Sprintf("is_active = $%d", argCount))
			args = append(args, isActive)
			argCount++
		}
	}
	if searchTerm != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("(name ILIKE $%d OR email ILIKE $%d)", argCount, argCount))
		args = append(args, "%"+searchTerm+"%")
		argCount++
	}

	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// --- Count Query ---
	countQuery := "SELECT COUNT(*) FROM users " + whereClause
	var totalItems int
	if err := db.QueryRow(ctx, countQuery, args...).Scan(&totalItems); err != nil {
		log.Printf("Error counting users: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to count users"})
	}

	// --- Main Query ---
	offset := (page - 1) * pageSize
	query := fmt.Sprintf("SELECT id, name, email, role, is_active, created_at, updated_at, merchant_id FROM users %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d", whereClause, argCount, argCount+1)
	args = append(args, pageSize, offset)

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		log.Printf("Error fetching users: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to fetch users"})
	}
	defer rows.Close()

	users := make([]models.User, 0)
	for rows.Next() {
		var user models.User
		var merchantID sql.NullString
		if err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt, &merchantID); err != nil {
			log.Printf("Error scanning user row: %v", err)
			continue
		}
		if merchantID.Valid {
			user.MerchantID = &merchantID.String
		}
		users = append(users, user)
	}

	// --- Response ---
	totalPages := int(math.Ceil(float64(totalItems) / float64(pageSize)))
	paginatedResponse := models.PaginatedAdminsResponse{
		Data: users,
		Pagination: models.Pagination{
			TotalItems:  totalItems,
			TotalPages:  totalPages,
			CurrentPage: page,
			PageSize:    pageSize,
		},
	}

	return c.JSON(fiber.Map{"status": "success", "data": paginatedResponse})
}

// HandleGetUserByID fetches a single user by their ID.
// GET /api/v1/admin/users/:userId
func HandleGetUserByID(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()
	userId := c.Params("userId")

	query := "SELECT id, name, email, role, is_active, phone, assigned_shop_id, merchant_id, created_at, updated_at FROM users WHERE id = $1"

	var user models.User
	var phone, assignedShopID, merchantID sql.NullString

	err := db.QueryRow(ctx, query, userId).Scan(
		&user.ID, &user.Name, &user.Email, &user.Role, &user.IsActive,
		&phone, &assignedShopID, &merchantID,
		&user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "User not found"})
		}
		log.Printf("Error fetching user by ID %s: %v", userId, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}

	if phone.Valid {
		user.Phone = &phone.String
	}
	if assignedShopID.Valid {
		user.AssignedShopID = &assignedShopID.String
	}
	if merchantID.Valid {
		user.MerchantID = &merchantID.String
	}

	return c.JSON(fiber.Map{"status": "success", "data": user})
}

// HandleAdminUpdateUser updates a user's details.
// PUT /api/v1/admin/users/:userId
func HandleAdminUpdateUser(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()
	userId := c.Params("userId")

	var updates map[string]interface{}
	if err := c.BodyParser(&updates); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid JSON format"})
	}

	var setParts []string
	var args []interface{}
	argCount := 1

	for key, value := range updates {
		if key == "password" && value.(string) != "" {
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(value.(string)), bcrypt.DefaultCost)
			if err != nil {
				log.Printf("Error hashing new password for user %s: %v", userId, err)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Error processing password"})
			}
			setParts = append(setParts, fmt.Sprintf("password_hash = $%d", argCount))
			args = append(args, string(hashedPassword))
			argCount++
		} else if key != "id" && key != "email" && key != "role" { // Prevent changing some critical fields
			setParts = append(setParts, fmt.Sprintf("%s = $%d", key, argCount))
			args = append(args, value)
			argCount++
		}
	}

	if len(setParts) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "No update fields provided"})
	}

	query := fmt.Sprintf("UPDATE users SET %s, updated_at = NOW() WHERE id = $%d RETURNING id", strings.Join(setParts, ", "), argCount)
	args = append(args, userId)

	var updatedId string
	if err := db.QueryRow(ctx, query, args...).Scan(&updatedId); err != nil {
		log.Printf("Error updating user %s: %v", userId, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update user"})
	}

	// Fetch the updated user to return it
	return HandleGetUserByID(c)
}

// HandleSetUserStatus activates or deactivates a user.
// PUT /api/v1/admin/users/:userId/status
func HandleSetUserStatus(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()
	userId := c.Params("userId")

	var body struct {
		IsActive bool `json:"is_active"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}

	query := "UPDATE users SET is_active = $1, updated_at = NOW() WHERE id = $2"
	_, err := db.Exec(ctx, query, body.IsActive, userId)
	if err != nil {
		log.Printf("Error updating status for user %s: %v", userId, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update user status"})
	}

	statusMsg := "deactivated"
	if body.IsActive {
		statusMsg = "activated"
	}

	return c.JSON(fiber.Map{"status": "success", "message": fmt.Sprintf("User %s successfully", statusMsg)})
}

// HandleDeleteUserMerchant deactivates a user, effectively a soft delete.
// DELETE /api/v1/admin/users/:userId
func HandleDeleteUserMerchant(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()
	userId := c.Params("userId")

	// Set user to inactive
	_, err := db.Exec(ctx, "UPDATE users SET is_active = false, updated_at = NOW() WHERE id = $1", userId)
	if err != nil {
		log.Printf("Error deactivating user %s: %v", userId, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to deactivate user"})
	}

	return c.JSON(fiber.Map{"status": "success", "message": "User deactivated successfully"})
}

// HandlePermanentDeleteUser permanently deletes a user.
// DELETE /api/v1/admin/users/:userId/permanent-delete
func HandlePermanentDeleteUser(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()
	userId := c.Params("userId")

	_, err := db.Exec(ctx, "DELETE FROM users WHERE id = $1", userId)
	if err != nil {
		log.Printf("Error permanently deleting user %s: %v", userId, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to permanently delete user"})
	}

	return c.JSON(fiber.Map{"status": "success", "message": "User permanently deleted"})
}

// HandleGetMerchantsForSelection fetches a list of merchants for dropdowns.
// GET /api/v1/admin/users/merchants-for-selection
func HandleGetMerchantsForSelection(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()
	query := "SELECT id, name FROM users WHERE role = 'merchant' AND is_active = true ORDER BY name"

	rows, err := db.Query(ctx, query)
	if err != nil {
		log.Printf("Error fetching merchants for selection: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}
	defer rows.Close()

	var selectionItems []models.User
	for rows.Next() {
		var item models.User
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			log.Printf("Error scanning merchant selection item: %v", err)
			continue
		}
		selectionItems = append(selectionItems, item)
	}

	return c.JSON(fiber.Map{"status": "success", "data": selectionItems})
}

// HandleCreateUser handles creation of a new user by an admin.
func HandleCreateUser(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	var req models.CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request format"})
	}

	if req.Email == "" || req.Password == "" || req.Role == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Missing required fields: email, password, and role"})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to secure password"})
	}

	query := `
        INSERT INTO users (name, email, password_hash, role, is_active, merchant_id)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id, name, email, role, is_active, created_at, updated_at
    `
	var createdUser models.User
	err = db.QueryRow(ctx, query,
		req.Name, req.Email, string(hashedPassword), req.Role, true, req.MerchantID,
	).Scan(
		&createdUser.ID, &createdUser.Name, &createdUser.Email, &createdUser.Role, &createdUser.IsActive,
		&createdUser.CreatedAt, &createdUser.UpdatedAt,
	)

	if err != nil {
		log.Printf("Error creating user in database: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Could not create user"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "data": createdUser})
}
