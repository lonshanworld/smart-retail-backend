package handlers

import (
	"app/database"
	"app/models"
	"context"
	"log"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

// HandleGetNotifications handles fetching a paginated list of notifications.
func HandleGetNotifications(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	recipientId := claims["userId"].(string)

	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize", "20"))
	offset := (page - 1) * pageSize

	// Get total count
	var totalCount int
	countQuery := "SELECT COUNT(*) FROM notifications WHERE recipient_user_id = $1"
	err := db.QueryRow(ctx, countQuery, recipientId).Scan(&totalCount)
	if err != nil {
		log.Printf("Error counting notifications: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}

	// Get paginated notifications
	query := `
		SELECT id, recipient_user_id, title, message, notification_type, related_entity_id, related_entity_type, is_read, created_at, updated_at
		FROM notifications
		WHERE recipient_user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := db.Query(ctx, query, recipientId, pageSize, offset)
	if err != nil {
		log.Printf("Error fetching notifications: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}
	defer rows.Close()

	notifications := make([]models.Notification, 0)
	for rows.Next() {
		var n models.Notification
		if err := rows.Scan(&n.ID, &n.RecipientUserID, &n.Title, &n.Message, &n.Type, &n.RelatedEntityID, &n.RelatedEntityType, &n.IsRead, &n.CreatedAt, &n.UpdatedAt); err != nil {
			log.Printf("Error scanning notification: %v", err)
			continue
		}
		notifications = append(notifications, n)
	}

	totalPages := (totalCount + pageSize - 1) / pageSize

	return c.JSON(fiber.Map{
		"success": true,
		"message": "success",
		"data":    notifications,
		"meta": fiber.Map{
			"totalItems":  totalCount,
			"currentPage": page,
			"pageSize":    pageSize,
			"totalPages":  totalPages,
		},
	})
}

// HandleGetUnreadNotificationsCount handles fetching the count of unread notifications.
func HandleGetUnreadNotificationsCount(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	recipientId := claims["userId"].(string)

	var count int
	query := "SELECT COUNT(*) FROM notifications WHERE recipient_user_id = $1 AND is_read = FALSE"
	err := db.QueryRow(ctx, query, recipientId).Scan(&count)
	if err != nil {
		log.Printf("Error counting unread notifications: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Database error"})
	}

	return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"count": count}})
}

// HandleMarkNotificationAsRead handles marking a specific notification as read.
func HandleMarkNotificationAsRead(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	recipientId := claims["userId"].(string)
	notificationId := c.Params("notificationId")

	query := "UPDATE notifications SET is_read = TRUE, updated_at = NOW() WHERE id = $1 AND recipient_user_id = $2"
	res, err := db.Exec(ctx, query, notificationId, recipientId)
	if err != nil {
		log.Printf("Error marking notification as read: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Database error"})
	}

	if res.RowsAffected() == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Notification not found or you do not have permission to modify it."})
	}

	return c.JSON(fiber.Map{"success": true, "message": "Notification marked as read"})
}
