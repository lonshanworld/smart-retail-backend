package handlers

import (
	"app/database"
	"app/models"
	"app/utils"
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

// HandleCreateUser creates a new user (admin, merchant, or staff).
func HandleCreateUser(c *fiber.Ctx) error {
	var req models.CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Cannot parse JSON"})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Error processing password"})
	}

	db := database.GetDB()
	ctx := context.Background()
	query := `
        INSERT INTO users (name, email, password_hash, role, merchant_id)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id, name, email, role, is_active, phone, assigned_shop_id, merchant_id, created_at, updated_at
    `

	var newUser models.User
	var phone, assignedShopID, merchantID sql.NullString

	err = db.QueryRow(ctx, query, req.Name, req.Email, string(hashedPassword), req.Role, req.MerchantID).Scan(
		&newUser.ID, &newUser.Name, &newUser.Email, &newUser.Role, &newUser.IsActive,
		&phone, &assignedShopID, &merchantID,
		&newUser.CreatedAt, &newUser.UpdatedAt,
	)

	if err != nil {
		log.Printf("Error creating user: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create user"})
	}

	newUser.Phone = utils.NullStringToStringPtr(phone)
	newUser.AssignedShopID = utils.NullStringToStringPtr(assignedShopID)
	newUser.MerchantID = utils.NullStringToStringPtr(merchantID)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "data": newUser})
}

// HandleListUsers retrieves a paginated and filtered list of all users.
func HandleListUsers(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize", "10"))
	offset := (page - 1) * pageSize

	var users []models.User
	query := "SELECT id, name, email, role, is_active, created_at, updated_at FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2"
	rows, err := db.Query(ctx, query, pageSize, offset)
	if err != nil {
		log.Printf("Error fetching users: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to fetch users"})
	}
	defer rows.Close()

	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt); err != nil {
			log.Printf("Error scanning user: %v", err)
			continue
		}
		users = append(users, user)
	}

	var totalItems int
	countErr := db.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&totalItems)
	if countErr != nil {
		log.Printf("Error counting users: %v", countErr)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to count users"})
	}

	pagination := models.Pagination{
		TotalItems:  totalItems,
		TotalPages:  int(math.Ceil(float64(totalItems) / float64(pageSize))),
		CurrentPage: page,
		PageSize:    pageSize,
	}

	return c.JSON(models.PaginatedUsersResponse{
		Data:       users,
		Pagination: pagination,
	})
}

// HandleGetUserByID retrieves a single user by their ID.
func HandleGetUserByID(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()
	userID := c.Params("userId")

	query := "SELECT id, name, email, role, is_active, phone, assigned_shop_id, merchant_id, created_at, updated_at FROM users WHERE id = $1"
	var user models.User
	var phone, assignedShopID, merchantID sql.NullString

	err := db.QueryRow(ctx, query, userID).Scan(
		&user.ID, &user.Name, &user.Email, &user.Role, &user.IsActive,
		&phone, &assignedShopID, &merchantID,
		&user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "User not found"})
		}
		log.Printf("Error fetching user by ID %s: %v", userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}

	user.Phone = utils.NullStringToStringPtr(phone)
	user.AssignedShopID = utils.NullStringToStringPtr(assignedShopID)
	user.MerchantID = utils.NullStringToStringPtr(merchantID)

	return c.JSON(fiber.Map{"status": "success", "data": user})
}

// HandleUpdateUser updates a user's details.
func HandleUpdateUser(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()
	userID := c.Params("userId")

	var updates map[string]interface{}
	if err := c.BodyParser(&updates); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid JSON"})
	}

	var setParts []string
	var args []interface{}
	argId := 1

	for key, value := range updates {
		switch key {
		case "name", "email", "role", "is_active", "phone", "assigned_shop_id", "merchant_id":
			setParts = append(setParts, fmt.Sprintf("%s = $%d", key, argId))
			args = append(args, value)
			argId++
		default:
			log.Printf("Attempted to update non-whitelisted user field: %s", key)
		}
	}

	if len(setParts) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "No updatable fields provided"})
	}

	query := fmt.Sprintf("UPDATE users SET %s, updated_at = NOW() WHERE id = $%d RETURNING id, name, email, role, is_active, phone, assigned_shop_id, merchant_id, created_at, updated_at", strings.Join(setParts, ", "), argId)
	args = append(args, userID)

	var updatedUser models.User
	var phone, assignedShopID, merchantID sql.NullString

	err := db.QueryRow(ctx, query, args...).Scan(
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

	updatedUser.Phone = utils.NullStringToStringPtr(phone)
	updatedUser.AssignedShopID = utils.NullStringToStringPtr(assignedShopID)
	updatedUser.MerchantID = utils.NullStringToStringPtr(merchantID)

	return c.JSON(fiber.Map{"status": "success", "data": updatedUser})
}

// HandleSetUserStatus sets the active status of a user.
func HandleSetUserStatus(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()
	userID := c.Params("userId")

	var body struct {
		IsActive bool `json:"is_active"`
	}

	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid JSON"})
	}

	query := "UPDATE users SET is_active = $1, updated_at = NOW() WHERE id = $2"
	result, err := db.Exec(ctx, query, body.IsActive, userID)
	if err != nil {
		log.Printf("Error updating user active status for user %s: %v", userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update user status"})
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "User not found"})
	}

	return c.JSON(fiber.Map{"status": "success", "message": fmt.Sprintf("User %s status updated successfully to %v", userID, body.IsActive)})
}

// HandleHardDeleteUser permanently deletes a user.
func HandleHardDeleteUser(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()
	userID := c.Params("userId")

	query := "DELETE FROM users WHERE id = $1"
	result, err := db.Exec(ctx, query, userID)
	if err != nil {
		log.Printf("Error deleting user %s: %v", userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to delete user"})
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "User not found"})
	}

	return c.JSON(fiber.Map{"status": "success", "message": fmt.Sprintf("User %s deleted successfully", userID)})
}

// HandleGetMerchantsForSelection fetches a list of merchants for dropdowns.
func HandleGetMerchantsForSelection(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	query := "SELECT id, name FROM users WHERE role = 'merchant' ORDER BY name"

	rows, err := db.Query(ctx, query)
	if err != nil {
		log.Printf("Error fetching merchants for selection: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}
	defer rows.Close()

	var selectionItems []models.UserSelectionItem
	for rows.Next() {
		var item models.UserSelectionItem
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			log.Printf("Error scanning merchant selection item: %v", err)
			continue
		}
		selectionItems = append(selectionItems, item)
	}

	return c.JSON(fiber.Map{"data": selectionItems})
}
