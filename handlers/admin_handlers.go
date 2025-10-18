package handlers

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"app/database"
	"app/models"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func HandleGetAdminDashboardSummary(c *fiber.Ctx) error {
	ctx := context.Background()
	var summary models.AdminDashboardSummary
	var err error

	// Helper function to create consistent error responses
	jsonError := func(message string) error {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": message})
	}

	err = database.DB.QueryRow(ctx, "SELECT COUNT(*) FROM merchants").Scan(&summary.TotalMerchants)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return jsonError("Failed to get total merchants")
	}

	err = database.DB.QueryRow(ctx, "SELECT COUNT(DISTINCT merchant_id) FROM shops WHERE is_active = TRUE").Scan(&summary.ActiveMerchants)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return jsonError("Failed to get active merchants")
	}

	err = database.DB.QueryRow(ctx, "SELECT COUNT(*) FROM staff").Scan(&summary.TotalStaff)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return jsonError("Failed to get total staff")
	}

	err = database.DB.QueryRow(ctx, "SELECT COUNT(*) FROM staff WHERE is_active = TRUE").Scan(&summary.ActiveStaff)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return jsonError("Failed to get active staff")
	}

	err = database.DB.QueryRow(ctx, "SELECT COUNT(*) FROM shops").Scan(&summary.TotalShops)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return jsonError("Failed to get total shops")
	}

	err = database.DB.QueryRow(ctx, "SELECT COALESCE(SUM(total_amount), 0) FROM sales").Scan(&summary.TotalSalesValue)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return jsonError("Failed to get total sales value")
	}

	err = database.DB.QueryRow(ctx, "SELECT COALESCE(SUM(total_amount), 0) FROM sales WHERE sale_date >= CURRENT_DATE").Scan(&summary.SalesToday)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return jsonError("Failed to get sales today")
	}

	err = database.DB.QueryRow(ctx, "SELECT COUNT(*) FROM sales WHERE sale_date >= CURRENT_DATE").Scan(&summary.TransactionsToday)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return jsonError("Failed to get transactions today")
	}

	return c.JSON(fiber.Map{"success": true, "data": summary})
}

func HandleAdminUpdateUser(c *fiber.Ctx) error {
	userID := c.Params("userId")
	var updates map[string]interface{}
	if err := c.BodyParser(&updates); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid JSON"})
	}

	tableName, role, err := findUserTableAndRole(userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	query, params := buildUpdateQuery(tableName, userID, updates)

	_, err = database.DB.Exec(context.Background(), query, params...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": fmt.Sprintf("Failed to update user: %v", err)})
	}

	updatedUser, err := fetchUser(userID, tableName, role)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": fmt.Sprintf("Failed to fetch updated user: %v", err)})
	}

	return c.JSON(fiber.Map{"success": true, "data": updatedUser})
}

func HandleGetUsers(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize
	ctx := context.Background()

	var totalCount int
	countQuery := `
        SELECT SUM(total) FROM (
            SELECT COUNT(*) as total FROM admins
            UNION ALL
            SELECT COUNT(*) as total FROM merchants
            UNION ALL
            SELECT COUNT(*) as total FROM staff
        ) as all_users`
	err := database.DB.QueryRow(ctx, countQuery).Scan(&totalCount)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to count users"})
	}

	query := `
        SELECT id, name, email, 'admin' as role, is_active, phone, assigned_shop_id, merchant_id, created_at, updated_at FROM admins
        UNION ALL
        SELECT id, name, email, 'merchant' as role, is_active, phone, NULL as assigned_shop_id, id as merchant_id, created_at, updated_at FROM merchants
        UNION ALL
        SELECT id, name, email, 'staff' as role, is_active, phone, shop_id as assigned_shop_id, merchant_id, created_at, updated_at FROM staff
        ORDER BY name ASC
        LIMIT $1 OFFSET $2`

	rows, err := database.DB.Query(ctx, query, pageSize, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to retrieve users"})
	}
	defer rows.Close()

	users := make([]models.User, 0)
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.Role, &user.IsActive, &user.Phone, &user.AssignedShopID, &user.MerchantID, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to scan user data"})
		}
		users = append(users, user)
	}

	response := models.AdminPaginatedUsersResponse{
		Users:       users,
		CurrentPage: page,
		PageSize:    pageSize,
		TotalCount:  totalCount,
		TotalPages:  int(math.Ceil(float64(totalCount) / float64(pageSize))),
	}

	return c.JSON(fiber.Map{"success": true, "data": response})
}

// --- Helper Functions (no changes needed below this line) ---

// findUserTableAndRole checks which table a user belongs to.
func findUserTableAndRole(userID string) (string, string, error) {
	ctx := context.Background()
	var exists bool

	if _, err := strconv.Atoi(userID); err != nil {
		return "", "", fmt.Errorf("invalid user ID format")
	}

	// Check admins table
	err := database.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM admins WHERE id = $1)", userID).Scan(&exists)
	if err == nil && exists {
		return "admins", "admin", nil
	}

	// Check merchants table
	err = database.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM merchants WHERE id = $1)", userID).Scan(&exists)
	if err == nil && exists {
		return "merchants", "merchant", nil
	}

	// Check staff table
	err = database.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM staff WHERE id = $1)", userID).Scan(&exists)
	if err == nil && exists {
		return "staff", "staff", nil
	}

	return "", "", fmt.Errorf("user with ID %s not found", userID)
}

// buildUpdateQuery constructs a dynamic SQL UPDATE query.
func buildUpdateQuery(tableName, userID string, updates map[string]interface{}) (string, []interface{}) {
	var setClauses []string
	var params []interface{}
	i := 1

	for key, value := range updates {
		// Basic validation to prevent updating sensitive fields or injecting malicious SQL
		if !isValidField(key) {
			continue
		}
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", key, i))
		params = append(params, value)
		i++
	}

	params = append(params, userID)
	query := fmt.Sprintf("UPDATE %s SET %s, updated_at = CURRENT_TIMESTAMP WHERE id = $%d", tableName, strings.Join(setClauses, ", "), i)

	return query, params
}

// isValidField is a simple allowlist of updatable fields.
func isValidField(field string) bool {
	switch field {
	case "name", "email", "phone", "is_active", "salary", "pay_frequency":
		return true
	default:
		return false
	}
}

// fetchUser fetches the full user details after an update.
func fetchUser(userID, tableName, role string) (*models.User, error) {
	var user models.User
	user.Role = role

	query := fmt.Sprintf(`SELECT id, name, email, phone, is_active, created_at, updated_at FROM %s WHERE id = $1`, tableName)
	err := database.DB.QueryRow(context.Background(), query, userID).Scan(
		&user.ID, &user.Name, &user.Email, &user.Phone, &user.IsActive, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &user, nil
}
