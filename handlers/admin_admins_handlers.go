package handlers

import (
	"context"
	"fmt"
	"math"
	"strconv"

	"app/database"
	"app/models"

	"github.com/gofiber/fiber/v2"
)

// HandleGetAdmins fetches a paginated list of admin users.
func HandleGetAdmins(c *fiber.Ctx) error {
	// Parse query parameters for pagination
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit
	ctx := context.Background()

	// Get total count of admins for pagination
	var totalItems int
	countQuery := "SELECT COUNT(*) FROM admins"
	err := database.DB.QueryRow(ctx, countQuery).Scan(&totalItems)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to count admin users"})
	}

	// Fetch the paginated list of admins
	query := `
		SELECT id, name, email, is_active, created_at, updated_at
		FROM admins
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := database.DB.Query(ctx, query, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to retrieve admin users"})
	}
	defer rows.Close()

	admins := make([]models.User, 0)
	for rows.Next() {
		var admin models.User
		admin.Role = "admin" // Explicitly set the role
		if err := rows.Scan(&admin.ID, &admin.Name, &admin.Email, &admin.IsActive, &admin.CreatedAt, &admin.UpdatedAt); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to scan admin user data"})
		}
		admins = append(admins, admin)
	}

	// Construct the final response object
	response := models.PaginatedAdminsResponse{
		Data: admins,
		Pagination: models.Pagination{
			TotalItems:  totalItems,
			TotalPages:  int(math.Ceil(float64(totalItems) / float64(limit))),
			CurrentPage: page,
		},
	}

	return c.JSON(response)
}
