package handlers

import (
	"app/database"
	"app/middleware"
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v4"
)

type supportMessageDTO struct {
	ID           string    `json:"id"`
	TicketID     string    `json:"ticketId"`
	SenderRole   string    `json:"senderRole"`
	Content      string    `json:"content"`
	IsAdminReply bool      `json:"isAdminReply"`
	CreatedAt    time.Time `json:"createdAt"`
}

type supportTicketDTO struct {
	ID            string              `json:"id"`
	MerchantID    string              `json:"merchantId"`
	ShopID        string              `json:"shopId"`
	Subject       string              `json:"subject"`
	Status        string              `json:"status"`
	Priority      string              `json:"priority"`
	CustomerName  *string             `json:"customerName,omitempty"`
	CustomerEmail *string             `json:"customerEmail,omitempty"`
	CustomerPhone *string             `json:"customerPhone,omitempty"`
	CreatedAt     time.Time           `json:"createdAt"`
	UpdatedAt     time.Time           `json:"updatedAt"`
	Messages      []supportMessageDTO `json:"messages"`
}

type createSupportTicketRequest struct {
	Subject           string  `json:"subject"`
	Message           string  `json:"message"`
	Priority          string  `json:"priority"`
	CustomerName      *string `json:"customerName"`
	CustomerEmail     *string `json:"customerEmail"`
	CustomerPhone     *string `json:"customerPhone"`
	ClientOperationID string  `json:"clientOperationId"`
}

type replySupportTicketRequest struct {
	Content           string `json:"content"`
	ClientOperationID string `json:"clientOperationId"`
}

type updateSupportTicketStatusRequest struct {
	Status            string `json:"status"`
	Priority          string `json:"priority"`
	ClientOperationID string `json:"clientOperationId"`
}

func resolveSupportScope(c *fiber.Ctx) (merchantID string, shopID string, role string, err error) {
	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return "", "", "", err
	}

	db := database.GetDB()
	ctx := context.Background()

	role = strings.ToLower(strings.TrimSpace(claims.Role))
	switch role {
	case "staff":
		var assignedShopID sql.NullString
		var staffMerchantID sql.NullString
		err = db.QueryRow(
			ctx,
			"SELECT assigned_shop_id, merchant_id FROM users WHERE id = $1 AND role = 'staff'",
			claims.UserID,
		).Scan(&assignedShopID, &staffMerchantID)
		if err != nil {
			return "", "", role, fiber.NewError(fiber.StatusForbidden, "Unable to resolve staff shop")
		}
		if !assignedShopID.Valid || assignedShopID.String == "" {
			return "", "", role, fiber.NewError(fiber.StatusForbidden, "Staff is not assigned to a shop")
		}
		if !staffMerchantID.Valid || staffMerchantID.String == "" {
			return "", "", role, fiber.NewError(fiber.StatusForbidden, "Unable to resolve staff merchant")
		}
		return staffMerchantID.String, assignedShopID.String, role, nil

	case "merchant":
		merchantID = claims.UserID
		shopID = strings.TrimSpace(c.Query("shopId"))
		if shopID == "" {
			shopID = strings.TrimSpace(c.Params("shopId"))
		}
		if shopID == "" {
			return "", "", role, fiber.NewError(fiber.StatusBadRequest, "shopId is required for merchant support operations")
		}

		var shopMerchantID string
		err = db.QueryRow(ctx, "SELECT merchant_id FROM shops WHERE id = $1", shopID).Scan(&shopMerchantID)
		if err != nil {
			if err == pgx.ErrNoRows {
				return "", "", role, fiber.NewError(fiber.StatusNotFound, "Shop not found")
			}
			return "", "", role, fiber.NewError(fiber.StatusInternalServerError, "Failed to verify shop ownership")
		}
		if shopMerchantID != merchantID {
			return "", "", role, fiber.NewError(fiber.StatusForbidden, "Access to shop denied")
		}
		return merchantID, shopID, role, nil
	default:
		return "", "", role, fiber.NewError(fiber.StatusForbidden, "Support module is only available for merchant and staff users")
	}
}

func senderRoleFromClaimsRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "staff":
		return "STAFF"
	case "merchant", "admin":
		return "ADMIN"
	default:
		return "CUSTOMER"
	}
}

func normalizePriority(priority string) string {
	p := strings.ToUpper(strings.TrimSpace(priority))
	switch p {
	case "LOW", "MEDIUM", "HIGH":
		return p
	default:
		return "MEDIUM"
	}
}

func normalizeStatus(status string) (string, bool) {
	s := strings.ToUpper(strings.TrimSpace(status))
	switch s {
	case "OPEN", "IN_PROGRESS", "CLOSED":
		return s, true
	default:
		return "", false
	}
}

func fetchSupportMessages(ctx context.Context, ticketID string) ([]supportMessageDTO, error) {
	db := database.GetDB()
	rows, err := db.Query(
		ctx,
		`SELECT id, ticket_id, sender_role, content, is_admin_reply, created_at
		 FROM support_messages
		 WHERE ticket_id = $1
		 ORDER BY created_at ASC`,
		ticketID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]supportMessageDTO, 0)
	for rows.Next() {
		var msg supportMessageDTO
		if err := rows.Scan(&msg.ID, &msg.TicketID, &msg.SenderRole, &msg.Content, &msg.IsAdminReply, &msg.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

func HandleListShopSupportTickets(c *fiber.Ctx) error {
	merchantID, shopID, _, err := resolveSupportScope(c)
	if err != nil {
		return err
	}

	db := database.GetDB()
	ctx := context.Background()
	status := strings.TrimSpace(c.Query("status"))

	query := `
		SELECT id, merchant_id, shop_id, subject, status, priority, customer_name, customer_email, customer_phone, created_at, updated_at
		FROM support_tickets
		WHERE merchant_id = $1 AND shop_id = $2
	`
	args := []interface{}{merchantID, shopID}
	if status != "" {
		normalized, ok := normalizeStatus(status)
		if !ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "success": false, "message": "Invalid status filter"})
		}
		query += " AND status = $3"
		args = append(args, normalized)
	}
	query += " ORDER BY updated_at DESC"

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to load support tickets"})
	}
	defer rows.Close()

	tickets := make([]supportTicketDTO, 0)
	for rows.Next() {
		var ticket supportTicketDTO
		var customerName, customerEmail, customerPhone sql.NullString
		if err := rows.Scan(
			&ticket.ID,
			&ticket.MerchantID,
			&ticket.ShopID,
			&ticket.Subject,
			&ticket.Status,
			&ticket.Priority,
			&customerName,
			&customerEmail,
			&customerPhone,
			&ticket.CreatedAt,
			&ticket.UpdatedAt,
		); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to parse support tickets"})
		}
		if customerName.Valid {
			ticket.CustomerName = &customerName.String
		}
		if customerEmail.Valid {
			ticket.CustomerEmail = &customerEmail.String
		}
		if customerPhone.Valid {
			ticket.CustomerPhone = &customerPhone.String
		}

		messages, err := fetchSupportMessages(ctx, ticket.ID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to load ticket messages"})
		}
		ticket.Messages = messages
		tickets = append(tickets, ticket)
	}

	return c.JSON(fiber.Map{"status": "success", "success": true, "data": tickets})
}

func HandleCreateShopSupportTicket(c *fiber.Ctx) error {
	merchantID, shopID, role, err := resolveSupportScope(c)
	if err != nil {
		return err
	}

	var req createSupportTicketRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "success": false, "message": "Invalid request body"})
	}

	req.Subject = strings.TrimSpace(req.Subject)
	req.Message = strings.TrimSpace(req.Message)
	req.ClientOperationID = strings.TrimSpace(req.ClientOperationID)
	if req.ClientOperationID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "success": false, "message": "clientOperationId is required"})
	}
	if req.Subject == "" || req.Message == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "success": false, "message": "subject and message are required"})
	}

	db := database.GetDB()
	ctx := context.Background()
	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to start transaction"})
	}
	defer tx.Rollback(ctx)
	claimed, err := claimInventoryOperation(ctx, tx, req.ClientOperationID, "support_ticket_create", merchantID, &shopID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to start operation"})
	}
	if !claimed {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "success": true, "message": "Operation already processed"})
	}

	var ticket supportTicketDTO
	var customerName, customerEmail, customerPhone sql.NullString
	err = db.QueryRow(
		ctx,
		`INSERT INTO support_tickets (merchant_id, shop_id, subject, status, priority, customer_name, customer_email, customer_phone)
		 VALUES ($1, $2, $3, 'OPEN', $4, $5, $6, $7)
		 RETURNING id, merchant_id, shop_id, subject, status, priority, customer_name, customer_email, customer_phone, created_at, updated_at`,
		merchantID,
		shopID,
		req.Subject,
		normalizePriority(req.Priority),
		req.CustomerName,
		req.CustomerEmail,
		req.CustomerPhone,
	).Scan(
		&ticket.ID,
		&ticket.MerchantID,
		&ticket.ShopID,
		&ticket.Subject,
		&ticket.Status,
		&ticket.Priority,
		&customerName,
		&customerEmail,
		&customerPhone,
		&ticket.CreatedAt,
		&ticket.UpdatedAt,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to create support ticket"})
	}
	if customerName.Valid {
		ticket.CustomerName = &customerName.String
	}
	if customerEmail.Valid {
		ticket.CustomerEmail = &customerEmail.String
	}
	if customerPhone.Valid {
		ticket.CustomerPhone = &customerPhone.String
	}

	var message supportMessageDTO
	err = db.QueryRow(
		ctx,
		`INSERT INTO support_messages (ticket_id, sender_role, content, is_admin_reply)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, ticket_id, sender_role, content, is_admin_reply, created_at`,
		ticket.ID,
		senderRoleFromClaimsRole(role),
		req.Message,
		false,
	).Scan(&message.ID, &message.TicketID, &message.SenderRole, &message.Content, &message.IsAdminReply, &message.CreatedAt)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to create support message"})
	}

	ticket.Messages = []supportMessageDTO{message}
	if err := tx.Commit(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to commit transaction"})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "success": true, "data": ticket})
}

func HandleGetShopSupportTicketByID(c *fiber.Ctx) error {
	merchantID, shopID, _, err := resolveSupportScope(c)
	if err != nil {
		return err
	}

	ticketID := strings.TrimSpace(c.Params("ticketId"))
	if ticketID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "success": false, "message": "ticketId is required"})
	}

	db := database.GetDB()
	ctx := context.Background()

	var ticket supportTicketDTO
	var customerName, customerEmail, customerPhone sql.NullString
	err = db.QueryRow(
		ctx,
		`SELECT id, merchant_id, shop_id, subject, status, priority, customer_name, customer_email, customer_phone, created_at, updated_at
		 FROM support_tickets
		 WHERE id = $1 AND merchant_id = $2 AND shop_id = $3`,
		ticketID,
		merchantID,
		shopID,
	).Scan(
		&ticket.ID,
		&ticket.MerchantID,
		&ticket.ShopID,
		&ticket.Subject,
		&ticket.Status,
		&ticket.Priority,
		&customerName,
		&customerEmail,
		&customerPhone,
		&ticket.CreatedAt,
		&ticket.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "success": false, "message": "Ticket not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to load ticket"})
	}

	if customerName.Valid {
		ticket.CustomerName = &customerName.String
	}
	if customerEmail.Valid {
		ticket.CustomerEmail = &customerEmail.String
	}
	if customerPhone.Valid {
		ticket.CustomerPhone = &customerPhone.String
	}

	messages, err := fetchSupportMessages(ctx, ticket.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to load ticket messages"})
	}
	ticket.Messages = messages

	return c.JSON(fiber.Map{"status": "success", "success": true, "data": ticket})
}

func HandleReplyShopSupportTicket(c *fiber.Ctx) error {
	merchantID, shopID, role, err := resolveSupportScope(c)
	if err != nil {
		return err
	}

	ticketID := strings.TrimSpace(c.Params("ticketId"))
	if ticketID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "success": false, "message": "ticketId is required"})
	}

	var req replySupportTicketRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "success": false, "message": "Invalid request body"})
	}
	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "success": false, "message": "content is required"})
	}

	db := database.GetDB()
	ctx := context.Background()

	var exists bool
	err = db.QueryRow(
		ctx,
		`SELECT EXISTS(
			SELECT 1 FROM support_tickets WHERE id = $1 AND merchant_id = $2 AND shop_id = $3
		)`,
		ticketID,
		merchantID,
		shopID,
	).Scan(&exists)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to validate ticket"})
	}
	if !exists {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "success": false, "message": "Ticket not found"})
	}

	var message supportMessageDTO
	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to start transaction"})
	}
	defer tx.Rollback(ctx)
	err = db.QueryRow(
		ctx,
		`INSERT INTO support_messages (ticket_id, sender_role, content, is_admin_reply)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, ticket_id, sender_role, content, is_admin_reply, created_at`,
		ticketID,
		senderRoleFromClaimsRole(role),
		req.Content,
		false,
	).Scan(&message.ID, &message.TicketID, &message.SenderRole, &message.Content, &message.IsAdminReply, &message.CreatedAt)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to add reply"})
	}

	_, _ = db.Exec(
		ctx,
		"UPDATE support_tickets SET updated_at = NOW() WHERE id = $1",
		ticketID,
	)
	if err := tx.Commit(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to commit transaction"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "success": true, "data": message})
}

func HandleUpdateShopSupportTicketStatus(c *fiber.Ctx) error {
	merchantID, shopID, _, err := resolveSupportScope(c)
	if err != nil {
		return err
	}

	ticketID := strings.TrimSpace(c.Params("ticketId"))
	if ticketID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "success": false, "message": "ticketId is required"})
	}

	var req updateSupportTicketStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "success": false, "message": "Invalid request body"})
	}

	status, ok := normalizeStatus(req.Status)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "success": false, "message": "Invalid status"})
	}
	priority := normalizePriority(req.Priority)

	db := database.GetDB()
	ctx := context.Background()

	var ticket supportTicketDTO
	var customerName, customerEmail, customerPhone sql.NullString
	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to start transaction"})
	}
	defer tx.Rollback(ctx)
	err = db.QueryRow(
		ctx,
		`UPDATE support_tickets
		 SET status = $1, priority = $2, updated_at = NOW()
		 WHERE id = $3 AND merchant_id = $4 AND shop_id = $5
		 RETURNING id, merchant_id, shop_id, subject, status, priority, customer_name, customer_email, customer_phone, created_at, updated_at`,
		status,
		priority,
		ticketID,
		merchantID,
		shopID,
	).Scan(
		&ticket.ID,
		&ticket.MerchantID,
		&ticket.ShopID,
		&ticket.Subject,
		&ticket.Status,
		&ticket.Priority,
		&customerName,
		&customerEmail,
		&customerPhone,
		&ticket.CreatedAt,
		&ticket.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "success": false, "message": "Ticket not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to update ticket"})
	}

	if customerName.Valid {
		ticket.CustomerName = &customerName.String
	}
	if customerEmail.Valid {
		ticket.CustomerEmail = &customerEmail.String
	}
	if customerPhone.Valid {
		ticket.CustomerPhone = &customerPhone.String
	}
	messages, err := fetchSupportMessages(ctx, ticket.ID)
	if err == nil {
		ticket.Messages = messages
	}
	if err := tx.Commit(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to commit transaction"})
	}

	return c.JSON(fiber.Map{"status": "success", "success": true, "data": ticket})
}
