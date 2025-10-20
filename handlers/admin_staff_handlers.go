package handlers

import (
	"app/database"
	"app/models"
	"context"
	"database/sql"
	"log"
	"math"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// HandleGetAllStaff handles the GET /api/admin/staff endpoint
func HandleGetAllStaff(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset := (page - 1) * limit

	// Get total count
	var totalItems int
	countQuery := "SELECT COUNT(*) FROM users WHERE role = 'staff'"
	if err := db.QueryRow(ctx, countQuery).Scan(&totalItems); err != nil {
		log.Printf("Error counting staff: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to count staff"})
	}

	// Get paginated data
	query := `
		SELECT u.id, u.name, u.email, u.role, u.is_active, u.created_at, u.updated_at, m.name as merchant_name
		FROM users u
		LEFT JOIN users m ON u.merchant_id = m.id
		WHERE u.role = 'staff'
		ORDER BY u.created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := db.Query(ctx, query, limit, offset)
	if err != nil {
		log.Printf("Error fetching staff: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to retrieve staff"})
	}
	defer rows.Close()

	var staff []models.User
	for rows.Next() {
		var user models.User
		var merchantName sql.NullString
		if err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt, &merchantName); err != nil {
			log.Printf("Error scanning staff row: %v", err)
			continue
		}
		if merchantName.Valid {
			// This is a bit of a hack, as the User model doesn't have a MerchantName field.
			// In a real application, you might want to create a specific struct for this response.
			// For now, we'll just log it.
			log.Printf("Staff member %s is part of merchant %s", user.ID, merchantName.String)
		}
		staff = append(staff, user)
	}

	pagination := models.Pagination{
		TotalItems:  totalItems,
		TotalPages:  int(math.Ceil(float64(totalItems) / float64(limit))),
		CurrentPage: page,
		PageSize:    limit,
	}

	return c.JSON(models.PaginatedUsersResponse{
		Data:       staff,
		Pagination: pagination,
	})
}
