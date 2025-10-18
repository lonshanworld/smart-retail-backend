package handlers

import (
	"app/database"
	"app/models"
	"fmt"
	"log"
	"math"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// HandleGetAdmins fetches a paginated list of admin users.
// GET /api/v1/admin/admins
func HandleGetAdmins(c *fiber.Ctx) error {
	db := database.GetDB()

	// --- Pagination Parameters ---
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20")) // Corresponds to 'limit' in frontend
	pageSize := limit
	offset := (page - 1) * pageSize

	// --- Sorting Parameters (example, you can expand this) ---
	sortBy := c.Query("sortBy", "created_at:desc")

	// --- Build Query ---
	var args []interface{}
	argId := 1

	// Base query for admins
	baseQuery := "FROM users WHERE role = 'admin'"

	// --- Get Total Count ---
	var totalItems int
	countQuery := "SELECT COUNT(*) " + baseQuery
	if err := db.QueryRow(countQuery).Scan(&totalItems); err != nil {
		log.Printf("Error counting admin users: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to count admin users"})
	}

	// --- Get Paginated Data ---
	query := fmt.Sprintf(
		"SELECT id, name, email, role, is_active, created_at, updated_at FROM users WHERE role = 'admin' ORDER BY %s LIMIT $%d OFFSET $%d",
		parseSortBy(sortBy), argId, argId+1,
	)
	args = append(args, pageSize, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("Error querying for admin users: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve admin users"})
	}
	defer rows.Close()

	admins := []models.User{}
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
			log.Printf("Error scanning admin user row: %v", err)
			continue
		}
		admins = append(admins, u)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Error after iterating over admin user rows: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Error processing admin user data"})
	}

	// --- Construct Response ---
	pagination := models.PaginationInfo{
		TotalItems:  totalItems,
		TotalPages:  int(math.Ceil(float64(totalItems) / float64(pageSize))),
		CurrentPage: page,
		PageSize:    pageSize,
	}

	response := models.PaginatedAdminUsersResponse{
		Data:       admins,
		Pagination: pagination,
	}

	return c.JSON(fiber.Map{"status": "success", "data": response})
}

// parseSortBy converts a "field:direction" string into a SQL ORDER BY clause.
// This is a simplified version and should be expanded for security and more complex cases.
func parseSortBy(sortBy string) string {
	parts := strings.Split(sortBy, ":")
	if len(parts) == 2 {
		field := parts[0]
		direction := strings.ToUpper(parts[1])
		if (direction == "ASC" || direction == "DESC") && (field == "createdAt" || field == "name" || field == "email") {
			// A real implementation should use a whitelist to prevent SQL injection
			if field == "createdAt" {
				field = "created_at" // map frontend name to db column name
			}
			return field + " " + direction
		}
	}
	// Default sort order
	return "created_at DESC"
}
