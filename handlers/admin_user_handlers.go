package handlers

import (
	"app/database"
	"app/models"
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

// HandleListUsers lists all users, with an optional role filter.
func HandleListUsers(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	roleFilter := c.Query("role")

	query := "SELECT id, name, email, role, is_active, created_at, updated_at FROM users"
	var args []interface{}
	if roleFilter != "" {
		query += " WHERE role = $1"
		args = append(args, roleFilter)
	}

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		log.Printf("Error querying users: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve users"})
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt); err != nil {
			log.Printf("Error scanning user row: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Error processing user data"})
		}
		users = append(users, user)
	}

	return c.JSON(fiber.Map{"status": "success", "data": fiber.Map{"users": users}})
}

// HandleGetUserByID fetches a single user by their ID.
func HandleGetUserByID(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	userID := c.Params("userId")

	var user models.User
	query := "SELECT id, name, email, role, is_active, created_at, updated_at FROM users WHERE id = $1"
	err := db.QueryRow(ctx, query, userID).Scan(&user.ID, &user.Name, &user.Email, &user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		log.Printf("Error fetching user with ID %s: %v", userID, err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "User not found"})
	}

	return c.JSON(fiber.Map{"status": "success", "data": user})
}

// HandleCreateUser creates a new user.
func HandleCreateUser(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create user"})
	}

	var user models.User
	query := `INSERT INTO users (name, email, password, role) VALUES ($1, $2, $3, $4) RETURNING id, name, email, role, is_active, created_at, updated_at`
	err = db.QueryRow(ctx, query, req.Name, req.Email, string(hashedPassword), req.Role).Scan(&user.ID, &user.Name, &user.Email, &user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		log.Printf("Error creating user: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create user"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "data": user})
}

// HandleUpdateUser updates an existing user.
func HandleUpdateUser(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	userID := c.Params("userId")

	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Role  string `json:"role"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}

	var user models.User
	query := `UPDATE users SET name = $1, email = $2, role = $3, updated_at = NOW() WHERE id = $4 RETURNING id, name, email, role, is_active, created_at, updated_at`
	err := db.QueryRow(ctx, query, req.Name, req.Email, req.Role, userID).Scan(&user.ID, &user.Name, &user.Email, &user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		log.Printf("Error updating user with ID %s: %v", userID, err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "User not found"})
	}

	return c.JSON(fiber.Map{"status": "success", "data": user})
}

// HandleHardDeleteUser permanently deletes a user.
func HandleHardDeleteUser(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	userID := c.Params("userId")

	query := "DELETE FROM users WHERE id = $1"
	_, err := db.Exec(ctx, query, userID)
	if err != nil {
		log.Printf("Error deleting user with ID %s: %v", userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to delete user"})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// HandleGetMerchantsForSelection fetches a list of merchants for selection.
func HandleGetMerchantsForSelection(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	query := "SELECT id, name FROM users WHERE role = 'merchant' AND is_active = TRUE"
	rows, err := db.Query(ctx, query)
	if err != nil {
		log.Printf("Error querying merchants for selection: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve merchants"})
	}
	defer rows.Close()

	var merchants []models.UserSelectionItem
	for rows.Next() {
		var merchant models.UserSelectionItem
		if err := rows.Scan(&merchant.ID, &merchant.Name); err != nil {
			log.Printf("Error scanning merchant for selection row: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Error processing merchant data"})
		}
		merchants = append(merchants, merchant)
	}

	return c.JSON(fiber.Map{"status": "success", "data": merchants})
}
