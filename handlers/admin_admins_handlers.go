package handlers

import (
	"app/database"
	"app/models"
	"context"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// HandleGetAdmins handles the GET /api/admin/admins endpoint
func HandleGetAdmins(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("pageSize", c.Query("limit", "20")))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit
	search := strings.TrimSpace(c.Query("search"))
	active := c.Query("isActive")
	where := " WHERE role = 'admin'"
	args := []interface{}{}
	if search != "" {
		where += " AND (name ILIKE $1 OR email ILIKE $1)"
		args = append(args, "%"+search+"%")
	}
	if active != "" {
		if value, err := strconv.ParseBool(active); err == nil {
			where += " AND is_active = $" + strconv.Itoa(len(args)+1)
			args = append(args, value)
		}
	}

	// Get total count
	var totalItems int
	countQuery := "SELECT COUNT(*) FROM users" + where
	if err := db.QueryRow(ctx, countQuery, args...).Scan(&totalItems); err != nil {
		log.Printf("Error counting admins: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to count admins"})
	}

	// Get paginated data
	query := "SELECT id, name, email, role, is_active, created_at, updated_at FROM users" + where + " ORDER BY created_at DESC, id DESC LIMIT $" + strconv.Itoa(len(args)+1) + " OFFSET $" + strconv.Itoa(len(args)+2)
	args = append(args, limit, offset)
	rows, err := db.Query(ctx, query, args...)
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
