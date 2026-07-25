package handlers

import (
	"app/database"
	"app/models"
	"log"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

// HandleInitializeAdmin creates the first admin user if none exists
func HandleInitializeAdmin(c *fiber.Ctx) error {
	// Check initialization token
	initToken := os.Getenv("INIT_TOKEN")
	if initToken == "" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  "error",
			"message": "INIT_TOKEN not configured",
		})
	}

	providedToken := c.Get("X-Init-Token")
	if providedToken != initToken {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid initialization token",
		})
	}

	// Parse request body
	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := c.BodyParser(&req); err != nil {
		log.Printf("Init request body parse error: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid request body",
		})
	}
	if strings.TrimSpace(req.Name) == "" || len(req.Name) > 255 || strings.TrimSpace(req.Email) == "" || len(req.Email) > 255 || len(req.Password) < 8 || len(req.Password) > 128 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Valid name, email, and password are required"})
	}

	// Check if a user with this email already exists (any role)
	var existingCount int
	err := database.GetDB().QueryRow(c.Context(),
		"SELECT COUNT(*) FROM users WHERE email = $1", req.Email).Scan(&existingCount)
	if err != nil {
		log.Printf("Database error checking email uniqueness: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Database error",
		})
	}

	if existingCount > 0 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"status":  "error",
			"message": "User with this email already exists",
		})
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Error processing password",
		})
	}

	// Create admin user
	var user models.User
	err = database.GetDB().QueryRow(c.Context(),
		`INSERT INTO users (name, email, password_hash, role, is_active)
		 VALUES ($1, $2, $3, 'admin', true)
		 RETURNING id, name, email, role, is_active, created_at, updated_at`,
		req.Name, req.Email, string(hashedPassword),
	).Scan(
		&user.ID, &user.Name, &user.Email, &user.Role, &user.IsActive,
		&user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		log.Printf("Error creating admin user: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Error creating admin user",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status":  "success",
		"message": "Admin user created successfully",
		"data":    user,
	})
}
