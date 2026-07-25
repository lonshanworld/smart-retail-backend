package handlers

import (
	"app/database"
	"app/middleware"
	"context"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v4"
)

type MoveStockRequest struct {
	ClientOperationID string `json:"clientOperationId"`
	ItemID            string `json:"itemId"`
	FromShopID        string `json:"fromShopId"`
	ToShopID          string `json:"toShopId"`
	Quantity          int    `json:"quantity"`
}

func HandleMoveStock(c *fiber.Ctx) error {
	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	var req MoveStockRequest
	if err = c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}
	if req.ClientOperationID == "" || req.ItemID == "" || req.FromShopID == "" || req.ToShopID == "" || req.Quantity <= 0 {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "clientOperationId, itemId, both shops, and a positive quantity are required"})
	}
	db, ctx := database.GetDB(), context.Background()
	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to start transfer"})
	}
	defer tx.Rollback(ctx)
	var fromOwner, toOwner string
	if err = tx.QueryRow(ctx, `SELECT merchant_id FROM shops WHERE id=$1`, req.FromShopID).Scan(&fromOwner); err != nil {
		return c.Status(404).JSON(fiber.Map{"status": "error", "message": "Source shop not found"})
	}
	if err = tx.QueryRow(ctx, `SELECT merchant_id FROM shops WHERE id=$1`, req.ToShopID).Scan(&toOwner); err != nil {
		return c.Status(404).JSON(fiber.Map{"status": "error", "message": "Destination shop not found"})
	}
	if fromOwner != claims.UserID || toOwner != claims.UserID {
		return c.Status(403).JSON(fiber.Map{"status": "error", "message": "Shop access denied"})
	}
	claimed, err := claimInventoryOperation(ctx, tx, req.ClientOperationID, "merchant_move_stock", claims.UserID, &req.FromShopID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to start transfer"})
	}
	if !claimed {
		return c.JSON(fiber.Map{"status": "success", "message": "Stock transfer already processed"})
	}
	var productID string
	if err = tx.QueryRow(ctx, `SELECT product_id FROM stock_items WHERE id=$1 AND merchant_id=$2`, req.ItemID, claims.UserID).Scan(&productID); err != nil {
		return c.Status(404).JSON(fiber.Map{"status": "error", "message": "Stock item not found"})
	}
	var fromID string
	var fromQty float64
	if err = tx.QueryRow(ctx, `SELECT id,quantity_on_hand FROM inventory_items WHERE shop_id=$1 AND stock_item_id=$2 FOR UPDATE`, req.FromShopID, req.ItemID).Scan(&fromID, &fromQty); err != nil {
		return c.Status(404).JSON(fiber.Map{"status": "error", "message": "Item is not stocked in source shop"})
	}
	if fromQty < float64(req.Quantity) {
		return c.Status(409).JSON(fiber.Map{"status": "error", "message": "Insufficient stock in source shop"})
	}
	newFrom := fromQty - float64(req.Quantity)
	if _, err = tx.Exec(ctx, `UPDATE inventory_items SET quantity_on_hand=$1,updated_at=NOW() WHERE id=$2`, newFrom, fromID); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to update source stock"})
	}
	var toID string
	var newTo float64
	if err = tx.QueryRow(ctx, `SELECT id,quantity_on_hand FROM inventory_items WHERE shop_id=$1 AND stock_item_id=$2 FOR UPDATE`, req.ToShopID, req.ItemID).Scan(&toID, &newTo); err == pgx.ErrNoRows {
		err = tx.QueryRow(ctx, `INSERT INTO inventory_items(merchant_id,shop_id,product_id,stock_item_id,quantity_on_hand) VALUES($1,$2,$3,$4,$5) RETURNING id,quantity_on_hand`, claims.UserID, req.ToShopID, productID, req.ItemID, req.Quantity).Scan(&toID, &newTo)
	} else if err == nil {
		newTo += float64(req.Quantity)
		_, err = tx.Exec(ctx, `UPDATE inventory_items SET quantity_on_hand=$1,updated_at=NOW() WHERE id=$2`, newTo, toID)
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to update destination stock"})
	}
	for _, v := range []struct {
		shop, inv, typ string
		qty            float64
	}{{req.FromShopID, fromID, "OUT", float64(req.Quantity)}, {req.ToShopID, toID, "IN", float64(req.Quantity)}} {
		if _, err = tx.Exec(ctx, `INSERT INTO inventory_movements(merchant_id,shop_id,inventory_item_id,product_id,stock_item_id,movement_type,quantity,base_quantity,reference_type,reference_id,event_key,notes) VALUES($1,$2,$3,$4,$5,$6,$7,$7,'TRANSFER',NULL,$8,$9)`, claims.UserID, v.shop, v.inv, productID, req.ItemID, v.typ, v.qty, fmt.Sprintf("%s:%s", req.ClientOperationID, v.shop), fmt.Sprintf("Transfer between shops: %s -> %s", req.FromShopID, req.ToShopID)); err != nil {
			return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to record stock transfer"})
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to commit transfer"})
	}
	return c.JSON(fiber.Map{"status": "success", "message": "Stock moved successfully", "data": fiber.Map{"fromShopNewQuantity": newFrom, "toShopNewQuantity": newTo}})
}
