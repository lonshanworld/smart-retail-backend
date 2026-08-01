package handlers

import (
	"app/database"
	"app/models"
	"context"
	"database/sql"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v4"
)

var identifierSortFields = map[string]string{"code": "code", "name": "name", "createdAt": "created_at", "updatedAt": "updated_at"}
var barcodeSortFields = map[string]string{"code": "code", "createdAt": "created_at"}

func HandleListInventoryIdentifierTypes(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	q := getCatalogListQuery(c, "code", identifierSortFields)
	where := " WHERE merchant_id=$1"
	args := []interface{}{merchantID}
	if q.Search != "" {
		where += " AND (code ILIKE $2 OR name ILIKE $2)"
		args = append(args, "%"+q.Search+"%")
	}
	db := database.GetDB()
	ctx := context.Background()
	var total int64
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM inventory_identifier_types"+where, args...).Scan(&total); err != nil {
		return fiber.NewError(500, "failed to count identifier types")
	}
	rows, err := db.Query(ctx, "SELECT id,merchant_id,code,name,description,validation_regex,is_active,created_at,updated_at FROM inventory_identifier_types"+where+q.orderBy()+" LIMIT $"+itoa(len(args)+1)+" OFFSET $"+itoa(len(args)+2), append(args, q.PageSize, q.Offset)...)
	if err != nil {
		return fiber.NewError(500, "failed to list identifier types")
	}
	defer rows.Close()
	items := make([]models.InventoryIdentifierType, 0)
	for rows.Next() {
		var item models.InventoryIdentifierType
		var desc, regex sql.NullString
		if err := rows.Scan(&item.ID, &item.MerchantID, &item.Code, &item.Name, &desc, &regex, &item.IsActive, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return fiber.NewError(500, "failed to read identifier type")
		}
		if desc.Valid {
			item.Description = &desc.String
		}
		if regex.Valid {
			item.ValidationRegex = &regex.String
		}
		items = append(items, item)
	}
	return c.JSON(paginatedResponse(items, total, q))
}

func HandleCreateInventoryIdentifierType(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	var req models.InventoryIdentifierTypeRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.Name) == "" {
		return fiber.NewError(400, "code and name are required")
	}
	if req.ValidationRegex != nil {
		if _, err := regexp.Compile(*req.ValidationRegex); err != nil {
			return fiber.NewError(400, "validationRegex is invalid")
		}
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	var item models.InventoryIdentifierType
	err = database.GetDB().QueryRow(context.Background(), `INSERT INTO inventory_identifier_types(merchant_id,code,name,description,validation_regex,is_active) VALUES($1,$2,$3,$4,$5,$6) RETURNING id,merchant_id,code,name,description,validation_regex,is_active,created_at,updated_at`, merchantID, strings.ToUpper(strings.TrimSpace(req.Code)), strings.TrimSpace(req.Name), req.Description, req.ValidationRegex, active).Scan(&item.ID, &item.MerchantID, &item.Code, &item.Name, &item.Description, &item.ValidationRegex, &item.IsActive, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "identifier type code already exists")
		}
		return fiber.NewError(500, "failed to create identifier type")
	}
	return c.Status(201).JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleUpdateInventoryIdentifierType(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	var req models.InventoryIdentifierTypeRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.Name) == "" {
		return fiber.NewError(400, "code and name are required")
	}
	if req.ValidationRegex != nil {
		if _, err := regexp.Compile(*req.ValidationRegex); err != nil {
			return fiber.NewError(400, "validationRegex is invalid")
		}
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	var item models.InventoryIdentifierType
	var desc, regex sql.NullString
	err = database.GetDB().QueryRow(context.Background(), `UPDATE inventory_identifier_types SET code=$1,name=$2,description=$3,validation_regex=$4,is_active=$5,updated_at=NOW() WHERE id=$6 AND merchant_id=$7 RETURNING id,merchant_id,code,name,description,validation_regex,is_active,created_at,updated_at`, strings.ToUpper(strings.TrimSpace(req.Code)), strings.TrimSpace(req.Name), req.Description, req.ValidationRegex, active, c.Params("identifierTypeId"), merchantID).Scan(&item.ID, &item.MerchantID, &item.Code, &item.Name, &desc, &regex, &item.IsActive, &item.CreatedAt, &item.UpdatedAt)
	if err == pgx.ErrNoRows {
		return fiber.NewError(404, "identifier type not found")
	}
	if err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "identifier type code already exists")
		}
		return fiber.NewError(500, "failed to update identifier type")
	}
	if desc.Valid {
		item.Description = &desc.String
	}
	if regex.Valid {
		item.ValidationRegex = &regex.String
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleDeleteInventoryIdentifierType(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	result, err := database.GetDB().Exec(context.Background(), `DELETE FROM inventory_identifier_types WHERE id=$1 AND merchant_id=$2`, c.Params("identifierTypeId"), merchantID)
	if err != nil {
		return fiber.NewError(409, "identifier type cannot be deleted because it is in use")
	}
	if result.RowsAffected() == 0 {
		return fiber.NewError(404, "identifier type not found")
	}
	return c.SendStatus(204)
}

func barcodeOwnerExists(ctx context.Context, merchantID, ownerType, ownerID string) (bool, error) {
	var ok bool
	var query string
	switch ownerType {
	case "PRODUCT":
		query = "SELECT EXISTS(SELECT 1 FROM products WHERE id=$1 AND merchant_id=$2)"
	case "VARIANT":
		query = "SELECT EXISTS(SELECT 1 FROM product_variants WHERE id=$1 AND merchant_id=$2)"
	case "STOCK_ITEM":
		query = "SELECT EXISTS(SELECT 1 FROM stock_items WHERE id=$1 AND merchant_id=$2)"
	case "ASSET":
		query = "SELECT EXISTS(SELECT 1 FROM inventory_assets WHERE id=$1 AND merchant_id=$2)"
	case "BATCH":
		query = "SELECT EXISTS(SELECT 1 FROM inventory_batches WHERE id=$1 AND merchant_id=$2)"
	case "UNIT":
		query = "SELECT EXISTS(SELECT 1 FROM unit_definitions WHERE id=$1)"
	default:
		return false, nil
	}
	err := database.GetDB().QueryRow(ctx, query, ownerID, merchantID).Scan(&ok)
	return ok, err
}

func HandleListMerchantBarcodes(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	q := getCatalogListQuery(c, "createdAt", barcodeSortFields)
	where := " WHERE merchant_id=$1"
	args := []interface{}{merchantID}
	if q.Search != "" {
		where += " AND (code ILIKE $2 OR normalized_code ILIKE $2)"
		args = append(args, "%"+q.Search+"%")
	}
	if ownerType := strings.TrimSpace(c.Query("ownerType")); ownerType != "" {
		where += " AND owner_type=$" + itoa(len(args)+1)
		args = append(args, strings.ToUpper(ownerType))
	}
	db := database.GetDB()
	ctx := context.Background()
	var total int64
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM barcode_registry"+where, args...).Scan(&total); err != nil {
		return fiber.NewError(500, "failed to count barcodes")
	}
	rows, err := db.Query(ctx, "SELECT id,merchant_id,code,normalized_code,owner_type,owner_id,is_primary,is_generated,is_active,metadata,created_at FROM barcode_registry"+where+q.orderBy()+" LIMIT $"+itoa(len(args)+1)+" OFFSET $"+itoa(len(args)+2), append(args, q.PageSize, q.Offset)...)
	if err != nil {
		return fiber.NewError(500, "failed to list barcodes")
	}
	defer rows.Close()
	items := make([]models.BarcodeRegistryEntry, 0)
	for rows.Next() {
		var item models.BarcodeRegistryEntry
		var metadata []byte
		if err := rows.Scan(&item.ID, &item.MerchantID, &item.Code, &item.NormalizedCode, &item.OwnerType, &item.OwnerID, &item.IsPrimary, &item.IsGenerated, &item.IsActive, &metadata, &item.CreatedAt); err != nil {
			return fiber.NewError(500, "failed to read barcode")
		}
		item.Metadata = map[string]interface{}{}
		if len(metadata) > 0 {
			_ = json.Unmarshal(metadata, &item.Metadata)
		}
		items = append(items, item)
	}
	return c.JSON(paginatedResponse(items, total, q))
}

func HandleCreateMerchantBarcode(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	var req models.BarcodeRegistryRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.OwnerID) == "" {
		return fiber.NewError(400, "code and ownerId are required")
	}
	req.OwnerType = strings.ToUpper(strings.TrimSpace(req.OwnerType))
	ok, err := barcodeOwnerExists(context.Background(), merchantID, req.OwnerType, req.OwnerID)
	if err != nil {
		return fiber.NewError(500, "failed to validate barcode owner")
	}
	if !ok {
		return fiber.NewError(400, "barcode owner does not belong to this merchant")
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	metadata, _ := json.Marshal(req.Metadata)
	var item models.BarcodeRegistryEntry
	err = database.GetDB().QueryRow(context.Background(), `INSERT INTO barcode_registry(merchant_id,code,normalized_code,owner_type,owner_id,is_primary,is_generated,is_active,metadata) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id,merchant_id,code,normalized_code,owner_type,owner_id,is_primary,is_generated,is_active,metadata,created_at`, merchantID, strings.TrimSpace(req.Code), strings.ToUpper(strings.TrimSpace(req.Code)), req.OwnerType, req.OwnerID, req.IsPrimary, req.IsGenerated, active, metadata).Scan(&item.ID, &item.MerchantID, &item.Code, &item.NormalizedCode, &item.OwnerType, &item.OwnerID, &item.IsPrimary, &item.IsGenerated, &item.IsActive, &metadata, &item.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "barcode already exists for this merchant")
		}
		return fiber.NewError(500, "failed to create barcode")
	}
	item.Metadata = req.Metadata
	return c.Status(201).JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleUpdateMerchantBarcode(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	var req models.BarcodeRegistryRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.OwnerID) == "" {
		return fiber.NewError(400, "code and ownerId are required")
	}
	req.OwnerType = strings.ToUpper(strings.TrimSpace(req.OwnerType))
	ok, err := barcodeOwnerExists(context.Background(), merchantID, req.OwnerType, req.OwnerID)
	if err != nil {
		return fiber.NewError(500, "failed to validate barcode owner")
	}
	if !ok {
		return fiber.NewError(400, "barcode owner does not belong to this merchant")
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	metadata, _ := json.Marshal(req.Metadata)
	var item models.BarcodeRegistryEntry
	err = database.GetDB().QueryRow(context.Background(), `UPDATE barcode_registry SET code=$1,normalized_code=$2,owner_type=$3,owner_id=$4,is_primary=$5,is_generated=$6,is_active=$7,metadata=$8 WHERE id=$9 AND merchant_id=$10 RETURNING id,merchant_id,code,normalized_code,owner_type,owner_id,is_primary,is_generated,is_active,metadata,created_at`, strings.TrimSpace(req.Code), strings.ToUpper(strings.TrimSpace(req.Code)), req.OwnerType, req.OwnerID, req.IsPrimary, req.IsGenerated, active, metadata, c.Params("barcodeId"), merchantID).Scan(&item.ID, &item.MerchantID, &item.Code, &item.NormalizedCode, &item.OwnerType, &item.OwnerID, &item.IsPrimary, &item.IsGenerated, &item.IsActive, &metadata, &item.CreatedAt)
	if err == pgx.ErrNoRows {
		return fiber.NewError(404, "barcode not found")
	}
	if err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "barcode already exists for this merchant")
		}
		return fiber.NewError(500, "failed to update barcode")
	}
	item.Metadata = req.Metadata
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleDeleteMerchantBarcode(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	result, err := database.GetDB().Exec(context.Background(), `DELETE FROM barcode_registry WHERE id=$1 AND merchant_id=$2`, c.Params("barcodeId"), merchantID)
	if err != nil {
		return fiber.NewError(500, "failed to delete barcode")
	}
	if result.RowsAffected() == 0 {
		return fiber.NewError(404, "barcode not found")
	}
	return c.SendStatus(204)
}
