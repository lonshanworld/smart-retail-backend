package handlers

import (
	"app/database"
	"app/models"
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

// HandleGetAssignedShop godoc
// @Summary Get staff's assigned shop
// @Description Fetches the details of the shop assigned to the currently logged-in staff member.
// @Tags Staff
// @Accept  json
// @Produce  json
// @Security ApiKeyAuth
// @Success 200 {object} models.Shop
// @Failure 401 {object} fiber.Map{message=string}
// @Failure 404 {object} fiber.Map{message=string}
// @Failure 500 {object} fiber.Map{message=string}
// @Router /api/v1/staff/assigned-shop [get]
func HandleGetAssignedShop(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	userID := claims["userId"].(string)

	var user models.User
	// First, get the user's assigned_shop_id
	userQuery := `SELECT assigned_shop_id FROM users WHERE id = $1`
	err := db.QueryRow(ctx, userQuery, userID).Scan(&user.AssignedShopID)
	if err != nil || user.AssignedShopID == nil {
		log.Printf("Error fetching assigned shop ID for user %s: %v", userID, err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Assigned shop not found for this user"})
	}

	var shop models.Shop
	shopQuery := `SELECT id, name, merchant_id, address, phone, is_active, is_primary, created_at, updated_at FROM shops WHERE id = $1`
	err = db.QueryRow(ctx, shopQuery, *user.AssignedShopID).Scan(
		&shop.ID, &shop.Name, &shop.MerchantID, &shop.Address, &shop.Phone, &shop.IsActive, &shop.IsPrimary, &shop.CreatedAt, &shop.UpdatedAt,
	)
	if err != nil {
		log.Printf("Error fetching shop details with ID %s: %v", *user.AssignedShopID, err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Shop details not found"})
	}

	return c.JSON(fiber.Map{"status": "success", "data": shop})
}

// HandleGetStaffProfile godoc
// @Summary Get staff profile
// @Description Fetches the profile of the currently logged-in staff member.
// @Tags Staff
// @Accept  json
// @Produce  json
// @Security ApiKeyAuth
// @Success 200 {object} models.User
// @Failure 401 {object} fiber.Map{message=string}
// @Failure 404 {object} fiber.Map{message=string}
// @Router /api/v1/staff/profile [get]
func HandleGetStaffProfile(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	userID := claims["userId"].(string)

	var user models.User
	query := `
        SELECT
            u.id, u.name, u.email, u.role, s.name as shop_name, sc.salary, sc.pay_frequency
        FROM users u
        LEFT JOIN shops s ON u.assigned_shop_id = s.id
        LEFT JOIN staff_contracts sc ON u.id = sc.staff_id
        WHERE u.id = $1
    `
	err := db.QueryRow(ctx, query, userID).Scan(
		&user.ID, &user.Name, &user.Email, &user.Role, &user.ShopName, &user.Salary, &user.PayFrequency,
	)

	if err != nil {
		log.Printf("Error fetching staff profile for user %s: %v", userID, err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Staff profile not found"})
	}

	return c.JSON(fiber.Map{"status": "success", "data": user})
}

// HandleGetSalaryHistory godoc
// @Summary Get staff salary history
// @Description Fetches the salary payment history for the currently logged-in staff member.
// @Tags Staff
// @Accept  json
// @Produce  json
// @Security ApiKeyAuth
// @Success 200 {array} models.Salary
// @Failure 401 {object} fiber.Map{message=string}
// @Failure 500 {object} fiber.Map{message=string}
// @Router /api/v1/staff/salary [get]
func HandleGetSalaryHistory(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	staffID := claims["userId"].(string)

	query := `SELECT id, staff_id, amount_paid, payment_date, notes FROM salary_payments WHERE staff_id = $1 ORDER BY payment_date DESC`
	rows, err := db.Query(ctx, query, staffID)
	if err != nil {
		log.Printf("Error querying salary history for staff %s: %v", staffID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve salary history"})
	}
	defer rows.Close()

	var salaries []models.Salary
	for rows.Next() {
		var salary models.Salary
		if err := rows.Scan(&salary.ID, &salary.StaffID, &salary.Amount, &salary.PaymentDate, &salary.Notes); err != nil {
			log.Printf("Error scanning salary row: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Error processing salary data"})
		}
		salaries = append(salaries, salary)
	}

	if err := rows.Err(); err != nil {
        log.Printf("Error iterating salary rows: %v", err)
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to process salary history"})
    }

	return c.JSON(fiber.Map{"status": "success", "data": salaries})
}
