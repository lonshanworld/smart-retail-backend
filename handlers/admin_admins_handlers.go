package handlers

import (
	"app/database"
	"app/models"
	"context"
	"log"
	"math"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// HandleGetAdmins handles the GET /api/admin/admins endpoint
func HandleGetAdmins(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset := (page - 1) * limit

	// Get total count
	var totalItems int
	countQuery := "SELECT COUNT(*) FROM users WHERE role = 'admin'"
	if err := db.QueryRow(ctx, countQuery).Scan(&totalItems); err != nil {
		log.Printf("Error counting admins: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to count admins"})
	}

	// Get paginated data
	query := "SELECT id, name, email, role, is_active, created_at, updated_at FROM users WHERE role = 'admin' ORDER BY created_at DESC LIMIT $1 OFFSET $2"
	rows, err := db.Query(ctx, query, limit, offset)
	if err != nil {
		log.Printf("Error fetching admins: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to retrieve admins"})
	}
	defer rows.Close()

	var admins []models.User
	for rows.Next() {
		var admin models.User
		if err := rows.Scan(&admin.ID, &admin.Name, &admin.Email, &admin.Role, &admin.IsActive, &admin.CreatedAt, &admin.UpdatedAt); err != nil {
			log.Printf("Error scanning admin row: %v", err)
			continue
		}
		admins = append(admins, admin)
	}

	pagination := models.Pagination{
		TotalItems:  totalItems,
		TotalPages:  int(math.Ceil(float64(totalItems) / float64(limit))),
		CurrentPage: page,
		PageSize:    limit,
	}

	return c.JSON(models.PaginatedUsersResponse{
		Data:       admins,
		Pagination: pagination,
	})
}
