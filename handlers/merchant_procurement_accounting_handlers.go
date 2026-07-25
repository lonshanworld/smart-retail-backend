package handlers

import (
	"app/database"
	"app/middleware"
	"context"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"strconv"
)

type purchaseItemRequest struct {
	ProductID   string  `json:"productId"`
	StockItemID string  `json:"stockItemId"`
	Quantity    float64 `json:"quantity"`
	UnitCost    float64 `json:"unitCost"`
}
type purchaseOrderRequest struct {
	ShopID            string                `json:"shopId"`
	SupplierID        string                `json:"supplierId"`
	ClientOperationID string                `json:"clientOperationId"`
	Items             []purchaseItemRequest `json:"items"`
}

func HandleListPurchaseOrders(c *fiber.Ctx) error {
	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	db, ctx := database.GetDB(), context.Background()
	page, _ := strconv.Atoi(c.Query("page", "1"))
	size, _ := strconv.Atoi(c.Query("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	off := (page - 1) * size
	where := ` WHERE merchant_id=$1`
	args := []interface{}{claims.UserID}
	if v := c.Query("shopId"); v != "" {
		where += fmt.Sprintf(" AND shop_id=$%d", len(args)+1)
		args = append(args, v)
	}
	if v := c.Query("status"); v != "" {
		where += fmt.Sprintf(" AND status=$%d", len(args)+1)
		args = append(args, v)
	}
	var total int
	if err = db.QueryRow(ctx, `SELECT COUNT(*) FROM purchase_orders`+where, args...).Scan(&total); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to count purchase orders"})
	}
	q := `SELECT id,shop_id,supplier_id,status,subtotal,tax,total,created_at,updated_at FROM purchase_orders` + where + ` ORDER BY created_at DESC LIMIT $` + strconv.Itoa(len(args)+1) + ` OFFSET $` + strconv.Itoa(len(args)+2)
	args = append(args, size, off)
	rows, err := db.Query(ctx, q, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to list purchase orders"})
	}
	defer rows.Close()
	orders := make([]fiber.Map, 0)
	for rows.Next() {
		var id, shop, supplier, status string
		var subtotal, tax, total float64
		var created, updated interface{}
		if rows.Scan(&id, &shop, &supplier, &status, &subtotal, &tax, &total, &created, &updated) == nil {
			orders = append(orders, fiber.Map{"id": id, "shopId": shop, "supplierId": supplier, "status": status, "subtotal": subtotal, "tax": tax, "total": total, "createdAt": created, "updatedAt": updated})
		}
	}
	return c.JSON(fiber.Map{"status": "success", "data": orders, "pagination": fiber.Map{"totalItems": total, "totalPages": (total + size - 1) / size, "currentPage": page, "pageSize": size}})
}

func HandleCreatePurchaseOrder(c *fiber.Ctx) error {
	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	var req purchaseOrderRequest
	if err = c.BodyParser(&req); err != nil || req.ShopID == "" || req.SupplierID == "" || req.ClientOperationID == "" || len(req.Items) == 0 {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "shopId, supplierId, clientOperationId, and items are required"})
	}
	db, ctx := database.GetDB(), context.Background()
	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to start purchase order"})
	}
	defer tx.Rollback(ctx)
	claimed, err := claimInventoryOperation(ctx, tx, req.ClientOperationID, "merchant_create_purchase_order", claims.UserID, &req.ShopID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to start purchase operation"})
	}
	if !claimed {
		return c.JSON(fiber.Map{"status": "success", "message": "Purchase order already processed"})
	}
	var ok int
	if err = tx.QueryRow(ctx, `SELECT COUNT(*) FROM shops WHERE id=$1 AND merchant_id=$2`, req.ShopID, claims.UserID).Scan(&ok); err != nil || ok == 0 {
		return c.Status(403).JSON(fiber.Map{"status": "error", "message": "Shop access denied"})
	}
	if err = tx.QueryRow(ctx, `SELECT COUNT(*) FROM suppliers WHERE id=$1 AND merchant_id=$2`, req.SupplierID, claims.UserID).Scan(&ok); err != nil || ok == 0 {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "Supplier not found"})
	}
	var subtotal float64
	for _, i := range req.Items {
		if i.ProductID == "" || i.Quantity <= 0 || i.UnitCost < 0 {
			return c.Status(400).JSON(fiber.Map{"status": "error", "message": "Invalid purchase item"})
		}
		var validItem int
		itemQuery := `SELECT COUNT(*) FROM stock_items WHERE merchant_id=$1 AND product_id=$2 AND ($3='' OR id=$3)`
		if err = tx.QueryRow(ctx, itemQuery, claims.UserID, i.ProductID, i.StockItemID).Scan(&validItem); err != nil || validItem == 0 {
			return c.Status(400).JSON(fiber.Map{"status": "error", "message": "Product or stock item does not belong to this merchant"})
		}
		subtotal += i.Quantity * i.UnitCost
	}
	var orderID string
	if err = tx.QueryRow(ctx, `INSERT INTO purchase_orders(merchant_id,shop_id,supplier_id,subtotal,total) VALUES($1,$2,$3,$4,$4) RETURNING id`, claims.UserID, req.ShopID, req.SupplierID, subtotal).Scan(&orderID); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to create purchase order"})
	}
	for _, i := range req.Items {
		if _, err = tx.Exec(ctx, `INSERT INTO purchase_order_items(purchase_order_id,product_id,stock_item_id,quantity,base_quantity,unit_cost,total_cost) VALUES($1,$2,$3,$4,$4,$5,$6)`, orderID, i.ProductID, nullableString(i.StockItemID), i.Quantity, i.UnitCost, i.Quantity*i.UnitCost); err != nil {
			return c.Status(400).JSON(fiber.Map{"status": "error", "message": "Invalid purchase item reference"})
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to commit purchase order"})
	}
	return c.Status(201).JSON(fiber.Map{"status": "success", "data": fiber.Map{"id": orderID, "status": "DRAFT", "subtotal": subtotal, "total": subtotal}})
}

type receiveRequest struct {
	RequestKey string `json:"requestKey"`
	Items      []struct {
		ProductID   string  `json:"productId"`
		StockItemID string  `json:"stockItemId"`
		Quantity    float64 `json:"quantity"`
		UnitCost    float64 `json:"unitCost"`
		BatchCode   string  `json:"batchCode"`
	} `json:"items"`
}

func HandleReceivePurchaseOrder(c *fiber.Ctx) error {
	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	var req receiveRequest
	if err = c.BodyParser(&req); err != nil || req.RequestKey == "" || len(req.Items) == 0 {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "requestKey and receipt items are required"})
	}
	db, ctx := database.GetDB(), context.Background()
	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to start receipt"})
	}
	defer tx.Rollback(ctx)
	var shopID string
	var receiptID string
	if err = tx.QueryRow(ctx, `SELECT po.shop_id FROM purchase_orders po WHERE po.id=$1 AND po.merchant_id=$2`, c.Params("orderId"), claims.UserID).Scan(&shopID); err != nil {
		return c.Status(404).JSON(fiber.Map{"status": "error", "message": "Purchase order not found"})
	}
	if err = tx.QueryRow(ctx, `INSERT INTO goods_receipts(request_key,purchase_order_id,received_by) VALUES($1,$2,$3) RETURNING id`, req.RequestKey, c.Params("orderId"), claims.UserID).Scan(&receiptID); err != nil {
		return c.Status(409).JSON(fiber.Map{"status": "error", "message": "Receipt already processed or invalid"})
	}
	for _, i := range req.Items {
		if i.StockItemID == "" {
			return c.Status(400).JSON(fiber.Map{"status": "error", "message": "stockItemId is required for goods receipt"})
		}
		var productID string
		if i.StockItemID != "" {
			if err = tx.QueryRow(ctx, `SELECT product_id FROM stock_items WHERE id=$1 AND merchant_id=$2`, i.StockItemID, claims.UserID).Scan(&productID); err != nil {
				return c.Status(400).JSON(fiber.Map{"status": "error", "message": "Stock item not found"})
			}
		} else {
			productID = i.ProductID
		}
		if i.Quantity <= 0 {
			return c.Status(400).JSON(fiber.Map{"status": "error", "message": "Receipt quantity must be positive"})
		}
		if _, err = tx.Exec(ctx, `INSERT INTO goods_receipt_items(goods_receipt_id,product_id,stock_item_id,quantity,base_quantity,unit_cost,batch_code) VALUES($1,$2,$3,$4,$4,$5,$6)`, receiptID, productID, nullableString(i.StockItemID), i.Quantity, i.UnitCost, nullableString(i.BatchCode)); err != nil {
			return c.Status(400).JSON(fiber.Map{"status": "error", "message": "Invalid receipt item"})
		}
		var invID string
		if err = tx.QueryRow(ctx, `SELECT id FROM inventory_items WHERE shop_id=$1 AND stock_item_id=$2 FOR UPDATE`, shopID, i.StockItemID).Scan(&invID); err == nil {
			_, err = tx.Exec(ctx, `UPDATE inventory_items SET quantity_on_hand=quantity_on_hand+$1,updated_at=NOW() WHERE id=$2`, i.Quantity, invID)
		} else {
			err = tx.QueryRow(ctx, `INSERT INTO inventory_items(merchant_id,shop_id,product_id,stock_item_id,quantity_on_hand) VALUES($1,$2,$3,$4,$5) RETURNING id`, claims.UserID, shopID, productID, i.StockItemID, i.Quantity).Scan(&invID)
		}
		if err != nil {
			return c.Status(409).JSON(fiber.Map{"status": "error", "message": "Failed to update inventory"})
		}
		if _, err = tx.Exec(ctx, `INSERT INTO inventory_movements(merchant_id,shop_id,inventory_item_id,product_id,stock_item_id,movement_type,quantity,base_quantity,reference_type,reference_id,event_key,unit_cost) VALUES($1,$2,$3,$4,$5,'IN',$6,$6,'GOODS_RECEIPT',$7,$8,$9)`, claims.UserID, shopID, invID, productID, nullableString(i.StockItemID), i.Quantity, receiptID, fmt.Sprintf("%s:%s", req.RequestKey, i.StockItemID), i.UnitCost); err != nil {
			return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to record inventory movement"})
		}
	}
	_, _ = tx.Exec(ctx, `UPDATE purchase_orders SET status='RECEIVED',updated_at=NOW() WHERE id=$1`, c.Params("orderId"))
	if err = tx.Commit(ctx); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to commit receipt"})
	}
	return c.JSON(fiber.Map{"status": "success", "data": fiber.Map{"receiptId": receiptID, "status": "RECEIVED"}})
}

func HandleListAccounts(c *fiber.Ctx) error {
	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	db, ctx := database.GetDB(), context.Background()
	page, _ := strconv.Atoi(c.Query("page", "1"))
	size, _ := strconv.Atoi(c.Query("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	where := " WHERE merchant_id=$1"
	args := []interface{}{claims.UserID}
	if v := c.Query("shopId"); v != "" {
		where += fmt.Sprintf(" AND shop_id=$%d", len(args)+1)
		args = append(args, v)
	}
	if v := c.Query("accountType"); v != "" {
		where += fmt.Sprintf(" AND account_type=$%d", len(args)+1)
		args = append(args, v)
	}
	if v := c.Query("search"); v != "" {
		where += fmt.Sprintf(" AND (code ILIKE $%d OR name ILIKE $%d)", len(args)+1, len(args)+1)
		args = append(args, "%"+v+"%")
	}
	var total int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM accounts"+where, args...).Scan(&total); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to count accounts"})
	}
	query := fmt.Sprintf("SELECT id,shop_id,code,name,account_type,is_active FROM accounts%s ORDER BY code LIMIT $%d OFFSET $%d", where, len(args)+1, len(args)+2)
	args = append(args, size, (page-1)*size)
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to list accounts"})
	}
	defer rows.Close()
	out := make([]fiber.Map, 0)
	for rows.Next() {
		var id, code, name, typ string
		var shop *string
		var active bool
		if rows.Scan(&id, &shop, &code, &name, &typ, &active) == nil {
			out = append(out, fiber.Map{"id": id, "shopId": shop, "code": code, "name": name, "accountType": typ, "isActive": active})
		}
	}
	return c.JSON(fiber.Map{"status": "success", "data": out, "pagination": fiber.Map{"totalItems": total, "totalPages": (total + size - 1) / size, "currentPage": page, "pageSize": size}})
}

func HandleCreateAccount(c *fiber.Ctx) error {
	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	var req struct {
		ShopID      *string `json:"shopId"`
		Code        string  `json:"code"`
		Name        string  `json:"name"`
		AccountType string  `json:"accountType"`
	}
	if err = c.BodyParser(&req); err != nil || req.Code == "" || req.Name == "" || req.AccountType == "" {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "code, name, and accountType are required"})
	}
	db, ctx := database.GetDB(), context.Background()
	if req.ShopID != nil && *req.ShopID != "" {
		var owned bool
		if err = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM shops WHERE id=$1 AND merchant_id=$2)`, *req.ShopID, claims.UserID).Scan(&owned); err != nil {
			return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to verify shop"})
		}
		if !owned {
			return c.Status(403).JSON(fiber.Map{"status": "error", "message": "Shop access denied"})
		}
	}
	var id string
	if err = db.QueryRow(ctx, `INSERT INTO accounts(merchant_id,shop_id,code,name,account_type) VALUES($1,$2,$3,$4,$5) RETURNING id`, claims.UserID, req.ShopID, req.Code, req.Name, req.AccountType).Scan(&id); err != nil {
		return c.Status(409).JSON(fiber.Map{"status": "error", "message": "Account could not be created"})
	}
	return c.Status(201).JSON(fiber.Map{"status": "success", "data": fiber.Map{"id": id, "code": req.Code, "name": req.Name, "accountType": req.AccountType}})
}

func HandleCreateJournalEntry(c *fiber.Ctx) error {
	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	var req struct {
		ShopID        *string `json:"shopId"`
		ReferenceType string  `json:"referenceType"`
		ReferenceID   *string `json:"referenceId"`
		EventKey      string  `json:"eventKey"`
		Description   string  `json:"description"`
		Lines         []struct {
			AccountID string  `json:"accountId"`
			Debit     float64 `json:"debit"`
			Credit    float64 `json:"credit"`
		} `json:"lines"`
	}
	if err = c.BodyParser(&req); err != nil || req.Description == "" || req.EventKey == "" || len(req.Lines) < 2 {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "eventKey, description, and at least two journal lines are required"})
	}
	var debit, credit float64
	for _, line := range req.Lines {
		if line.AccountID == "" || line.Debit < 0 || line.Credit < 0 || (line.Debit > 0 && line.Credit > 0) || (line.Debit == 0 && line.Credit == 0) {
			return c.Status(400).JSON(fiber.Map{"status": "error", "message": "Each journal line must contain either a positive debit or credit"})
		}
		debit += line.Debit
		credit += line.Credit
	}
	if debit != credit {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "Journal debits and credits must balance"})
	}
	db, ctx := database.GetDB(), context.Background()
	if req.ShopID != nil && *req.ShopID != "" {
		var owned bool
		if err = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM shops WHERE id=$1 AND merchant_id=$2)`, *req.ShopID, claims.UserID).Scan(&owned); err != nil {
			return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to verify shop"})
		}
		if !owned {
			return c.Status(403).JSON(fiber.Map{"status": "error", "message": "Shop access denied"})
		}
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to start journal entry"})
	}
	defer tx.Rollback(ctx)
	claimed, err := claimInventoryOperation(ctx, tx, req.EventKey, "merchant_create_journal_entry", claims.UserID, req.ShopID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to start journal operation"})
	}
	if !claimed {
		return c.JSON(fiber.Map{"status": "success", "message": "Journal entry already processed"})
	}
	var id string
	if err = tx.QueryRow(ctx, `INSERT INTO journal_entries(merchant_id,shop_id,reference_type,reference_id,event_key,description) VALUES($1,$2,$3,$4,$5,$6) RETURNING id`, claims.UserID, req.ShopID, req.ReferenceType, req.ReferenceID, nullableString(req.EventKey), req.Description).Scan(&id); err != nil {
		return c.Status(409).JSON(fiber.Map{"status": "error", "message": "Journal entry could not be created"})
	}
	for _, line := range req.Lines {
		result, execErr := tx.Exec(ctx, `INSERT INTO journal_lines(journal_entry_id,account_id,debit,credit) SELECT $1,id,$2,$3 FROM accounts WHERE id=$4 AND merchant_id=$5`, id, line.Debit, line.Credit, line.AccountID, claims.UserID)
		if execErr != nil || result.RowsAffected() != 1 {
			return c.Status(400).JSON(fiber.Map{"status": "error", "message": "Invalid account in journal line"})
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to commit journal entry"})
	}
	return c.Status(201).JSON(fiber.Map{"status": "success", "data": fiber.Map{"id": id, "debit": debit, "credit": credit}})
}
