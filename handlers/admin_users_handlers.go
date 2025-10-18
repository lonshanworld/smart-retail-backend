package handlers

import (
	"app/database"
	"app/models"
	"log"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

func HandleCreateUserV2(c *fiber.Ctx) error {
	db := database.GetDB()

	// Parse the request body into a User model
	var user models.User
	if err := c.BodyParser(&user); err != nil {
		log.Printf("Error parsing user creation request: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request format"})
	}

	// Validate required fields
	if user.Email == "" || user.Password == "" || user.Role == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Missing required fields: email, password, and role"})
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to secure password"})
	}

	// Insert the new user into the database
	query := `
        INSERT INTO users (name, email, password_hash, role, is_active, phone, assigned_shop_id, merchant_id)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        RETURNING id, name, email, role, is_active, created_at, updated_at
    `
	var createdUser models.User
	err = db.QueryRow(query,
		user.Name, user.Email, string(hashedPassword), user.Role, user.IsActive,
		user.Phone, user.AssignedShopID, user.MerchantID,
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
