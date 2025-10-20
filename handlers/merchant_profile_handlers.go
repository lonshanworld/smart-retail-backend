package handlers

import (
	"app/database"
	"app/models"
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

// HandleGetMerchantProfile handles fetching the current merchant's profile
func HandleGetMerchantProfile(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	userId := claims["userId"].(string)

	var userProfile models.User
	query := "SELECT id, name, email, phone, role, created_at, updated_at FROM users WHERE id = $1 AND role = 'merchant'"

	err := db.QueryRow(ctx, query, userId).Scan(&userProfile.ID, &userProfile.Name, &userProfile.Email, &userProfile.Phone, &userProfile.Role, &userProfile.CreatedAt, &userProfile.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Merchant not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}

	return c.JSON(fiber.Map{"status": "success", "data": userProfile})
}

// HandleUpdateMerchantProfile handles updating the current merchant's profile
func HandleUpdateMerchantProfile(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	userId := claims["userId"].(string)

	type UpdateRequest struct {
		Name     string `json:"name,omitempty"`
		Password string `json:"password,omitempty"`
	}

	var req UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}

	var setClauses []string
	args := make([]interface{}, 0)
	paramIndex := 1

	if req.Name != "" {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", paramIndex))
		args = append(args, req.Name)
		paramIndex++
	}

	if req.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to hash password"})
		}
		setClauses = append(setClauses, fmt.Sprintf("password = $%d", paramIndex))
		args = append(args, hashedPassword)
		paramIndex++
	}

	if len(setClauses) == 0 {
		// Nothing to update, so just return the current profile
		return HandleGetMerchantProfile(c)
	}

	setClauses = append(setClauses, "updated_at = NOW()")

	query := fmt.Sprintf("UPDATE users SET %s WHERE id = $%d AND role = 'merchant' RETURNING id, name, email, phone, role, created_at, updated_at", strings.Join(setClauses, ", "), paramIndex)
	args = append(args, userId)

	var updatedUser models.User
	err := db.QueryRow(ctx, query, args...).Scan(&updatedUser.ID, &updatedUser.Name, &updatedUser.Email, &updatedUser.Phone, &updatedUser.Role, &updatedUser.CreatedAt, &updatedUser.UpdatedAt)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update profile"})
	}

	return c.JSON(fiber.Map{"status": "success", "data": updatedUser})
}
