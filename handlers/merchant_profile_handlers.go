package handlers

import (
	"app/database"
	"app/middleware"
	"app/models"
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

// HandleGetMerchantProfile handles fetching the current merchant's profile
func HandleGetMerchantProfile(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	userId := claims.UserID

	var userProfile models.User
	query := "SELECT id, name, email, phone, role, created_at, updated_at FROM users WHERE id = $1 AND role = 'merchant'"

	err = db.QueryRow(ctx, query, userId).Scan(&userProfile.ID, &userProfile.Name, &userProfile.Email, &userProfile.Phone, &userProfile.Role, &userProfile.CreatedAt, &userProfile.UpdatedAt)
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

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	userId := claims.UserID

	clientOperationID := c.Get("X-Client-Operation-Id")
	if clientOperationID == "" {
		clientOperationID = c.Query("clientOperationId")
	}
	if clientOperationID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "clientOperationId is required"})
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}
	defer tx.Rollback(ctx)
	claimed, err := claimInventoryOperation(ctx, tx, clientOperationID, "merchant_update_profile", userId, nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to start operation"})
	}
	if !claimed {
		return HandleGetMerchantProfile(c)
	}
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
		setClauses = append(setClauses, fmt.Sprintf("password_hash = $%d", paramIndex))
		args = append(args, hashedPassword)
		paramIndex++
	}

	// If nothing to update, return current profile
	if len(setClauses) == 0 {
		var current models.User
		selectQuery := "SELECT id, name, email, phone, role, created_at, updated_at FROM users WHERE id = $1 AND role = 'merchant'"
		if err := tx.QueryRow(ctx, selectQuery, userId).Scan(&current.ID, &current.Name, &current.Email, &current.Phone, &current.Role, &current.CreatedAt, &current.UpdatedAt); err != nil {
			if err == sql.ErrNoRows {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Merchant not found"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
		}
		if err := tx.Commit(ctx); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to commit operation"})
		}
		return c.JSON(fiber.Map{"status": "success", "data": current})
	}

	setClauses = append(setClauses, "updated_at = NOW()")
	queryUpdate := fmt.Sprintf("UPDATE users SET %s WHERE id = $%d AND role = 'merchant' RETURNING id, name, email, phone, role, created_at, updated_at", strings.Join(setClauses, ", "), paramIndex)
	args = append(args, userId)

	var updatedUser models.User
	if err := tx.QueryRow(ctx, queryUpdate, args...).Scan(&updatedUser.ID, &updatedUser.Name, &updatedUser.Email, &updatedUser.Phone, &updatedUser.Role, &updatedUser.CreatedAt, &updatedUser.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Merchant not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update profile"})
	}

	if err := tx.Commit(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to commit operation"})
	}

	return c.JSON(fiber.Map{"status": "success", "data": updatedUser})
}
