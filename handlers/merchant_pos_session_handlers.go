package handlers

import (
	"app/database"
	"app/middleware"
	"context"
	"github.com/gofiber/fiber/v2"
	"strconv"
	"strings"
)

func HandleOpenPOSSession(c *fiber.Ctx) error {
	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	var req struct {
		ShopID            string  `json:"shopId"`
		TerminalID        *string `json:"terminalId"`
		OpeningCash       float64 `json:"openingCash"`
		ClientOperationID string  `json:"clientOperationId"`
	}
	if err = c.BodyParser(&req); err != nil || req.ShopID == "" || strings.TrimSpace(req.ClientOperationID) == "" {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "shopId and clientOperationId are required"})
	}
	db, ctx := database.GetDB(), context.Background()
	var owner string
	if err = db.QueryRow(ctx, `SELECT merchant_id FROM shops WHERE id=$1`, req.ShopID).Scan(&owner); err != nil {
		return c.Status(404).JSON(fiber.Map{"status": "error", "message": "Shop not found"})
	}
	if owner != claims.UserID {
		return c.Status(403).JSON(fiber.Map{"status": "error", "message": "Shop access denied"})
	}
	if req.OpeningCash < 0 {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "openingCash cannot be negative"})
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to start POS session"})
	}
	defer tx.Rollback(ctx)
	claimed, err := claimInventoryOperation(ctx, tx, strings.TrimSpace(req.ClientOperationID), "open_pos_session", claims.UserID, &req.ShopID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to start POS session operation"})
	}
	if !claimed {
		return c.Status(200).JSON(fiber.Map{"status": "success", "message": "POS session operation already processed"})
	}
	var id string
	err = tx.QueryRow(ctx, `INSERT INTO pos_sessions(shop_id,terminal_id,user_id,cash_in_hand) VALUES($1,$2,$3,$4) RETURNING id`, req.ShopID, req.TerminalID, claims.UserID, req.OpeningCash).Scan(&id)
	if err != nil {
		return c.Status(409).JSON(fiber.Map{"status": "error", "message": "An open POS session already exists for this terminal or the session is invalid"})
	}
	if err = tx.Commit(ctx); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to commit POS session"})
	}
	return c.Status(201).JSON(fiber.Map{"status": "success", "data": fiber.Map{"id": id, "shopId": req.ShopID, "terminalId": req.TerminalID, "openingCash": req.OpeningCash, "status": "OPEN"}})
}

func HandleListPOSSessions(c *fiber.Ctx) error {
	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	shopID := c.Query("shopId")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	size, _ := strconv.Atoi(c.Query("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	db, ctx := database.GetDB(), context.Background()
	query := `SELECT ps.id,ps.shop_id,ps.terminal_id,ps.user_id,ps.opened_at,ps.closed_at,ps.cash_in_hand,ps.expected_cash,ps.counted_cash,ps.variance,ps.reconciled_at,ps.reconciled_by,ps.status FROM pos_sessions ps JOIN shops s ON s.id=ps.shop_id WHERE s.merchant_id=$1`
	args := []interface{}{claims.UserID}
	if shopID != "" {
		query += " AND ps.shop_id=$2"
		args = append(args, shopID)
	}
	countQuery := "SELECT COUNT(*) FROM pos_sessions ps JOIN shops s ON s.id=ps.shop_id WHERE s.merchant_id=$1"
	countArgs := []interface{}{claims.UserID}
	if shopID != "" {
		countQuery += " AND ps.shop_id=$2"
		countArgs = append(countArgs, shopID)
	}
	var total int
	if err = db.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to count POS sessions"})
	}
	query += " ORDER BY ps.opened_at DESC LIMIT $" + strconv.Itoa(len(args)+1) + " OFFSET $" + strconv.Itoa(len(args)+2)
	args = append(args, size, (page-1)*size)
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to list POS sessions"})
	}
	defer rows.Close()
	sessions := make([]fiber.Map, 0)
	for rows.Next() {
		var id, shop, user, status string
		var terminal, reconciled *string
		var opened, closed, reconciledAt interface{}
		var opening, expected, counted, variance *float64
		if err = rows.Scan(&id, &shop, &terminal, &user, &opened, &closed, &opening, &expected, &counted, &variance, &reconciledAt, &reconciled, &status); err == nil {
			sessions = append(sessions, fiber.Map{"id": id, "shopId": shop, "terminalId": terminal, "userId": user, "openedAt": opened, "closedAt": closed, "cashInHand": opening, "expectedCash": expected, "countedCash": counted, "variance": variance, "reconciledAt": reconciledAt, "reconciledBy": reconciled, "status": status})
		}
	}
	return c.JSON(fiber.Map{"status": "success", "data": sessions, "pagination": fiber.Map{"totalItems": total, "totalPages": (total + size - 1) / size, "currentPage": page, "pageSize": size}})
}

func HandleClosePOSSession(c *fiber.Ctx) error {
	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	var req struct {
		CountedCash       float64 `json:"countedCash"`
		ClientOperationID string  `json:"clientOperationId"`
	}
	if err = c.BodyParser(&req); err != nil || req.CountedCash < 0 || strings.TrimSpace(req.ClientOperationID) == "" {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "countedCash and clientOperationId are required"})
	}
	db, ctx := database.GetDB(), context.Background()
	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to start POS session operation"})
	}
	defer tx.Rollback(ctx)
	claimed, err := claimInventoryOperation(ctx, tx, strings.TrimSpace(req.ClientOperationID), "close_pos_session", claims.UserID, nil)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to start POS session operation"})
	}
	if !claimed {
		return c.Status(200).JSON(fiber.Map{"status": "success", "message": "POS session operation already processed"})
	}
	var expected float64
	if err = tx.QueryRow(ctx, `SELECT COALESCE(ps.cash_in_hand,0)+COALESCE((SELECT SUM(p.amount) FROM payments p JOIN sales s ON s.id=p.sale_id JOIN pos_transactions pt ON pt.sale_id=s.id WHERE pt.session_id=ps.id AND p.method='CASH' AND p.status='SUCCESS'),0) FROM pos_sessions ps JOIN shops sh ON sh.id=ps.shop_id WHERE ps.id=$1 AND sh.merchant_id=$2 AND ps.user_id=$2 AND ps.status='OPEN' FOR UPDATE`, c.Params("sessionId"), claims.UserID).Scan(&expected); err != nil {
		return c.Status(404).JSON(fiber.Map{"status": "error", "message": "Open POS session not found"})
	}
	var id string
	var variance float64
	if err = tx.QueryRow(ctx, `UPDATE pos_sessions SET closed_at=NOW(),expected_cash=$1,counted_cash=$2,variance=$2::numeric-$1::numeric,reconciled_at=NOW(),reconciled_by=$3,status='CLOSED' WHERE id=$4 AND user_id=$3 AND status='OPEN' RETURNING id,variance`, expected, req.CountedCash, claims.UserID, c.Params("sessionId")).Scan(&id, &variance); err != nil {
		return c.Status(409).JSON(fiber.Map{"status": "error", "message": "POS session is already closed or unavailable"})
	}
	if err = tx.Commit(ctx); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to commit POS session"})
	}
	return c.JSON(fiber.Map{"status": "success", "data": fiber.Map{"id": id, "expectedCash": expected, "countedCash": req.CountedCash, "variance": variance, "status": "CLOSED"}})
}
