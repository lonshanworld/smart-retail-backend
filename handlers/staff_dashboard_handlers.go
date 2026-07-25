package handlers

import (
	"app/database"
	"app/middleware"
	"app/models"
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
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

	activityPage, _ := strconv.Atoi(c.Query("activityPage", "1"))
	activityPageSize, _ := strconv.Atoi(c.Query("activityPageSize", "5"))
	if activityPage < 1 {
		activityPage = 1
	}
	if activityPageSize < 1 || activityPageSize > 100 {
		activityPageSize = 5
	}
	activityWhere := " WHERE staff_id=$1"
	activityArgs := []interface{}{staffID}
	if from := strings.TrimSpace(c.Query("from")); from != "" {
		activityWhere += " AND sale_date >= $" + strconv.Itoa(len(activityArgs)+1)
		activityArgs = append(activityArgs, from)
	}
	if to := strings.TrimSpace(c.Query("to")); to != "" {
		activityWhere += " AND sale_date <= $" + strconv.Itoa(len(activityArgs)+1)
		activityArgs = append(activityArgs, to)
	}
	var activityTotal int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM sales"+activityWhere, activityArgs...).Scan(&activityTotal); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to count recent activities"})
	}
	// Recent activities are paginated so the summary cannot grow without bound.
	activityQuery := `
		SELECT id, total_amount, sale_date
		FROM sales
		` + activityWhere + `
		ORDER BY sale_date DESC, id DESC
		LIMIT $` + strconv.Itoa(len(activityArgs)+1) + ` OFFSET $` + strconv.Itoa(len(activityArgs)+2)
	activityArgs = append(activityArgs, activityPageSize, (activityPage-1)*activityPageSize)
	rows, err := db.Query(ctx, activityQuery, activityArgs...)
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
		"success":    true,
		"data":       summary,
		"pagination": fiber.Map{"totalItems": activityTotal, "totalPages": (activityTotal + activityPageSize - 1) / activityPageSize, "currentPage": activityPage, "pageSize": activityPageSize},
	})
}
