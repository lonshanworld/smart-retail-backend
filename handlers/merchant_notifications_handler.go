package handlers

import (
	"app/database"
	"app/middleware"
	"app/models"
	"context"
	"log"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// HandleGetNotifications handles fetching a paginated list of notifications.
func HandleGetNotifications(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	recipientId := claims.UserID

	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	where := " WHERE recipient_user_id = $1"
	args := []interface{}{recipientId}
	if read := c.Query("isRead"); read != "" {
		if value, parseErr := strconv.ParseBool(read); parseErr == nil {
			where += " AND is_read = $2"
			args = append(args, value)
		}
	}
	if notificationType := strings.TrimSpace(c.Query("type")); notificationType != "" {
		where += " AND notification_type = $" + strconv.Itoa(len(args)+1)
		args = append(args, notificationType)
	}
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		where += " AND (title ILIKE $" + strconv.Itoa(len(args)+1) + " OR message ILIKE $" + strconv.Itoa(len(args)+1) + ")"
		args = append(args, "%"+search+"%")
	}

	// Get total count
	var totalCount int
	countQuery := "SELECT COUNT(*) FROM notifications" + where
	err = db.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		log.Printf("Error counting notifications: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}

	// Get paginated notifications
	query := `
		SELECT id, recipient_user_id, title, message, notification_type, related_entity_id, related_entity_type, is_read, created_at, updated_at
		FROM notifications
		` + where + `
		ORDER BY created_at DESC
		LIMIT $` + strconv.Itoa(len(args)+1) + ` OFFSET $` + strconv.Itoa(len(args)+2) + `
	`
	args = append(args, pageSize, offset)
	rows, err := db.Query(ctx, query, args...)
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
func HandleGetUnreadNotificationCount(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	recipientId := claims.UserID

	var count int
	query := "SELECT COUNT(*) FROM notifications WHERE recipient_user_id = $1 AND is_read = FALSE"
	err = db.QueryRow(ctx, query, recipientId).Scan(&count)
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

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	recipientId := claims.UserID
	notificationId := c.Params("notificationId")
	clientOperationID := c.Get("X-Client-Operation-Id")
	if clientOperationID == "" {
		clientOperationID = c.Query("clientOperationId")
	}
	if clientOperationID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "clientOperationId is required"})
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to start transaction"})
	}
	defer tx.Rollback(ctx)
	claimed, err := claimInventoryOperation(ctx, tx, clientOperationID, "mark_notification_read", recipientId, nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to start operation"})
	}
	if !claimed {
		return c.JSON(fiber.Map{"success": true, "message": "Notification marked as read"})
	}

	query := "UPDATE notifications SET is_read = TRUE, updated_at = NOW() WHERE id = $1 AND recipient_user_id = $2"
	res, err := tx.Exec(ctx, query, notificationId, recipientId)
	if err != nil {
		log.Printf("Error marking notification as read: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Database error"})
	}

	if res.RowsAffected() == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Notification not found or you do not have permission to modify it."})
	}

	if err := tx.Commit(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to commit transaction"})
	}
	return c.JSON(fiber.Map{"success": true, "message": "Notification marked as read"})
}
