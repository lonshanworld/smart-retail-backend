package handlers

import (
	"app/database"
	"app/models"
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// HandleGetAdmins fetches a paginated list of admin users.
// GET /api/v1/admin/admins
func HandleGetAdmins(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	// --- Pagination Parameters ---
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset := (page - 1) * limit

	// --- Sorting Parameters ---
	sortBy := c.Query("sortBy", "created_at:desc")

	// --- Build Query ---
	// Base query for admins
	baseQuery := "FROM users WHERE role = 'admin'"

	// --- Get Total Count ---
	var totalItems int
	countQuery := "SELECT COUNT(*) " + baseQuery
	if err := db.QueryRow(ctx, countQuery).Scan(&totalItems); err != nil {
		log.Printf("Error counting admin users: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to count admin users"})
	}

	// --- Get Paginated Data ---
	query := fmt.Sprintf(
		"SELECT id, name, email, role, is_active, created_at, updated_at %s ORDER BY %s LIMIT %d OFFSET %d",
		baseQuery,
		parseSortBy(sortBy),
		limit,
		offset,
	)

	rows, err := db.Query(ctx, query)
	if err != nil {
		log.Printf("Error querying for admin users: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to retrieve admin users"})
	}
	defer rows.Close()

	admins := []models.User{}
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
			log.Printf("Error scanning admin user row: %v", err)
			continue // Skip problematic rows
		}
		admins = append(admins, u)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Error after iterating over admin user rows: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Error processing admin user data"})
	}

	// --- Construct Response ---
	pagination := models.Pagination{
		TotalItems:  totalItems,
		TotalPages:  int(math.Ceil(float64(totalItems) / float64(limit))),
		CurrentPage: page,
		PageSize:    limit,
	}

	response := models.PaginatedUsersResponse{
		Data:       admins,
		Pagination: pagination,
	}

	return c.JSON(response) // Return the correctly structured response directly
}

// parseSortBy converts a "field:direction" string into a SQL ORDER BY clause.
func parseSortBy(sortBy string) string {
	parts := strings.Split(sortBy, ":")
	if len(parts) == 2 {
		field := parts[0]
		direction := strings.ToUpper(parts[1])
		// Whitelist fields to prevent SQL injection
		allowedFields := map[string]string{
			"createdAt": "created_at",
			"name":      "name",
			"email":     "email",
		}
		dbField, ok := allowedFields[field]
		if ok && (direction == "ASC" || direction == "DESC") {
			return dbField + " " + direction
		}
	}
	// Default sort order
	return "created_at DESC"
}
