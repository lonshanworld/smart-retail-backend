package handlers

import (
	"app/database"
	"app/middleware"
	"app/models"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

// HandleGetStaffDashboardSummary retrieves a summary for the staff dashboard.
func HandleGetStaffDashboardSummary(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	staffID := claims.UserID

	// Get assigned shop name
	var shopName string
	shopQuery := "SELECT name FROM shops WHERE id = (SELECT assigned_shop_id FROM users WHERE id = $1)"
	if err := db.QueryRow(ctx, shopQuery, staffID).Scan(&shopName); err != nil {
		log.Printf("Error getting staff shop name: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve shop details"})
	}

	// Get sales and transactions for today
	var salesToday float64
	var transactionsToday int
	salesQuery := `
		SELECT COALESCE(SUM(total_amount), 0), COUNT(*)
		FROM sales
		WHERE staff_id = $1 AND sale_date >= $2
	`
	today := time.Now().Truncate(24 * time.Hour)
	if err := db.QueryRow(ctx, salesQuery, staffID, today).Scan(&salesToday, &transactionsToday); err != nil {
		log.Printf("Error getting staff sales summary: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve sales summary"})
	}

	// Get recent activities (e.g., last 5 sales)
	activityQuery := `
		SELECT id, total_amount, sale_date
		FROM sales
		WHERE staff_id = $1
		ORDER BY sale_date DESC
		LIMIT 5
	`
	rows, err := db.Query(ctx, activityQuery, staffID)
	if err != nil {
		log.Printf("Error getting recent activities: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve recent activities"})
	}
	defer rows.Close()

	var activities []models.StaffRecentActivity
	for rows.Next() {
		var activity models.StaffRecentActivity
		var saleTotal float64
		if err := rows.Scan(&activity.RelatedID, &saleTotal, &activity.Timestamp); err != nil {
			log.Printf("Error scanning recent activity: %v", err)
			continue
		}
		activity.Type = "sale"
		activity.Details = fmt.Sprintf("Sale of %.2f", saleTotal)
		activities = append(activities, activity)
	}

	summary := models.StaffDashboardSummaryResponse{
		AssignedShopName:  shopName,
		SalesToday:        salesToday,
		TransactionsToday: transactionsToday,
		RecentActivities:  activities,
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    summary,
	})
}
