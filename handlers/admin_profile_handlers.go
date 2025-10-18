package handlers

import (
	"context"
	"app/database"
	"app/models"
	"database/sql"
	"fmt"
	"log"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

// HandleUpdateAdminProfile updates the profile of the currently logged-in admin.
// PUT /api/v1/admin/profile
func HandleUpdateAdminProfile(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(string)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "Unauthorized"})
	}

	db := database.GetDB()
	ctx := context.Background()
	var updates map[string]interface{}
	if err := c.BodyParser(&updates); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid JSON"})
	}

	var setParts []string
	var args []interface{}
	argId := 1

	// Handle name update
	if name, ok := updates["name"]; ok {
		if nameStr, ok := name.(string); ok && nameStr != "" {
			setParts = append(setParts, fmt.Sprintf("name = $%d", argId))
			args = append(args, nameStr)
			argId++
		}
	}

	// Handle password update
	if password, ok := updates["password"]; ok {
		if passwordStr, ok := password.(string); ok && passwordStr != "" {
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(passwordStr), bcrypt.DefaultCost)
			if err != nil {
				log.Printf("Error hashing new password for user %s: %v", userID, err)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Error processing new password"})
			}
			setParts = append(setParts, fmt.Sprintf("password_hash = $%d", argId))
			args = append(args, string(hashedPassword))
			argId++
		}
	}

	if len(setParts) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "No updatable fields provided"})
	}

	query := fmt.Sprintf("UPDATE users SET %s, updated_at = NOW() WHERE id = $%d AND role = 'admin' RETURNING id, name, email, role, is_active, phone, assigned_shop_id, merchant_id, created_at, updated_at", fmt.Sprintf(",", setParts), argId)
	args = append(args, userID)

	var updatedUser models.User
	var phone, assignedShopID, merchantID sql.NullString

	err := db.QueryRow(ctx, query, args...).Scan(
		&updatedUser.ID, &updatedUser.Name, &updatedUser.Email, &updatedUser.Role, &updatedUser.IsActive,
		&phone, &assignedShopID, &merchantID,
		&updatedUser.CreatedAt, &updatedUser.UpdatedAt,
	)

	if err != nil {
		log.Printf("Error updating admin profile for user %s: %v", userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update profile"})
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

// HandleGetAdminProfile fetches the profile of the currently logged-in admin.
// GET /api/v1/admin/profile
func HandleGetAdminProfile(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(string)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "Unauthorized"})
	}

	db := database.GetDB()
	ctx := context.Background()
	query := "SELECT id, name, email, role, is_active, phone, assigned_shop_id, merchant_id, created_at, updated_at FROM users WHERE id = $1 AND role = 'admin'"

	var user models.User
	var phone, assignedShopID, merchantID sql.NullString

	err := db.QueryRow(ctx, query, userID).Scan(
		&user.ID, &user.Name, &user.Email, &user.Role, &user.IsActive,
		&phone, &assignedShopID, &merchantID,
		&user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Admin profile not found"})
		}
		log.Printf("Error fetching admin profile for user %s: %v", userID, err)
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
