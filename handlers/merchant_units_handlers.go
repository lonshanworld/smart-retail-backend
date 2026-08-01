package handlers

import (
	"app/database"
	"app/models"
	"context"
	"database/sql"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v4"
)

var measurementSortFields = map[string]string{"code": "code", "name": "name", "createdAt": "created_at"}
var unitSortFields = map[string]string{"code": "code", "name": "name", "createdAt": "created_at"}

func HandleListMeasurementGroups(c *fiber.Ctx) error {
	q := getCatalogListQuery(c, "code", measurementSortFields)
	where := ""
	args := []interface{}{}
	if q.Search != "" {
		where = " WHERE code ILIKE $1 OR name ILIKE $1"
		args = append(args, "%"+q.Search+"%")
	}
	db := database.GetDB()
	ctx := context.Background()
	var total int64
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM measurement_groups"+where, args...).Scan(&total); err != nil {
		return fiber.NewError(500, "failed to count measurement groups")
	}
	rows, err := db.Query(ctx, "SELECT id,code,name,created_at,updated_at FROM measurement_groups"+where+q.orderBy()+" LIMIT $"+itoa(len(args)+1)+" OFFSET $"+itoa(len(args)+2), append(args, q.PageSize, q.Offset)...)
	if err != nil {
		return fiber.NewError(500, "failed to list measurement groups")
	}
	defer rows.Close()
	items := make([]models.MeasurementGroup, 0)
	for rows.Next() {
		var item models.MeasurementGroup
		if err := rows.Scan(&item.ID, &item.Code, &item.Name, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return fiber.NewError(500, "failed to read measurement group")
		}
		items = append(items, item)
	}
	return c.JSON(paginatedResponse(items, total, q))
}

func HandleCreateMeasurementGroup(c *fiber.Ctx) error {
	var req models.MeasurementGroupRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.Name) == "" {
		return fiber.NewError(400, "code and name are required")
	}
	var item models.MeasurementGroup
	err := database.GetDB().QueryRow(context.Background(), `INSERT INTO measurement_groups(code,name) VALUES($1,$2) RETURNING id,code,name,created_at,updated_at`, strings.ToUpper(strings.TrimSpace(req.Code)), strings.TrimSpace(req.Name)).Scan(&item.ID, &item.Code, &item.Name, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "measurement group code already exists")
		}
		return fiber.NewError(500, "failed to create measurement group")
	}
	return c.Status(201).JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleUpdateMeasurementGroup(c *fiber.Ctx) error {
	var req models.MeasurementGroupRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.Name) == "" {
		return fiber.NewError(400, "code and name are required")
	}
	var item models.MeasurementGroup
	err := database.GetDB().QueryRow(context.Background(), `UPDATE measurement_groups SET code=$1,name=$2,updated_at=NOW() WHERE id=$3 RETURNING id,code,name,created_at,updated_at`, strings.ToUpper(strings.TrimSpace(req.Code)), strings.TrimSpace(req.Name), c.Params("groupId")).Scan(&item.ID, &item.Code, &item.Name, &item.CreatedAt, &item.UpdatedAt)
	if err == pgx.ErrNoRows {
		return fiber.NewError(404, "measurement group not found")
	}
	if err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "measurement group code already exists")
		}
		return fiber.NewError(500, "failed to update measurement group")
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleDeleteMeasurementGroup(c *fiber.Ctx) error {
	result, err := database.GetDB().Exec(context.Background(), `DELETE FROM measurement_groups WHERE id=$1`, c.Params("groupId"))
	if err != nil {
		return fiber.NewError(409, "measurement group cannot be deleted because units use it")
	}
	if result.RowsAffected() == 0 {
		return fiber.NewError(404, "measurement group not found")
	}
	return c.SendStatus(204)
}

func HandleListUnitDefinitions(c *fiber.Ctx) error {
	q := getCatalogListQuery(c, "code", unitSortFields)
	where := ""
	args := []interface{}{}
	if q.Search != "" {
		where = " WHERE (u.code ILIKE $1 OR u.name ILIKE $1 OR COALESCE(u.symbol,'') ILIKE $1)"
		args = append(args, "%"+q.Search+"%")
	}
	if groupID := strings.TrimSpace(c.Query("measurementGroupId")); groupID != "" {
		if where == "" {
			where = " WHERE"
		} else {
			where += " AND"
		}
		where += " u.measurement_group_id=$" + itoa(len(args)+1)
		args = append(args, groupID)
	}
	db := database.GetDB()
	ctx := context.Background()
	var total int64
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM unit_definitions u"+where, args...).Scan(&total); err != nil {
		return fiber.NewError(500, "failed to count unit definitions")
	}
	rows, err := db.Query(ctx, "SELECT u.id,u.measurement_group_id,u.code,u.name,u.symbol,u.allows_decimal,u.created_at,u.updated_at FROM unit_definitions u"+where+q.orderBy()+" LIMIT $"+itoa(len(args)+1)+" OFFSET $"+itoa(len(args)+2), append(args, q.PageSize, q.Offset)...)
	if err != nil {
		return fiber.NewError(500, "failed to list unit definitions")
	}
	defer rows.Close()
	items := make([]models.UnitDefinition, 0)
	for rows.Next() {
		var item models.UnitDefinition
		var group, symbol sql.NullString
		if err := rows.Scan(&item.ID, &group, &item.Code, &item.Name, &symbol, &item.AllowsDecimal, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return fiber.NewError(500, "failed to read unit definition")
		}
		if group.Valid {
			item.MeasurementGroupID = &group.String
		}
		if symbol.Valid {
			item.Symbol = &symbol.String
		}
		items = append(items, item)
	}
	return c.JSON(paginatedResponse(items, total, q))
}

func HandleCreateUnitDefinition(c *fiber.Ctx) error {
	var req models.UnitDefinitionRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.Name) == "" {
		return fiber.NewError(400, "code and name are required")
	}
	allows := true
	if req.AllowsDecimal != nil {
		allows = *req.AllowsDecimal
	}
	var item models.UnitDefinition
	err := database.GetDB().QueryRow(context.Background(), `INSERT INTO unit_definitions(measurement_group_id,code,name,symbol,allows_decimal) VALUES($1,$2,$3,$4,$5) RETURNING id,measurement_group_id,code,name,symbol,allows_decimal,created_at,updated_at`, nullableStringValue(req.MeasurementGroupID), strings.ToUpper(strings.TrimSpace(req.Code)), strings.TrimSpace(req.Name), nullableStringValue(req.Symbol), allows).Scan(&item.ID, &item.MeasurementGroupID, &item.Code, &item.Name, &item.Symbol, &item.AllowsDecimal, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "unit code already exists")
		}
		return fiber.NewError(500, "failed to create unit definition")
	}
	return c.Status(201).JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleUpdateUnitDefinition(c *fiber.Ctx) error {
	var req models.UnitDefinitionRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.Name) == "" {
		return fiber.NewError(400, "code and name are required")
	}
	allows := true
	if req.AllowsDecimal != nil {
		allows = *req.AllowsDecimal
	}
	var item models.UnitDefinition
	var group, symbol sql.NullString
	err := database.GetDB().QueryRow(context.Background(), `UPDATE unit_definitions SET measurement_group_id=$1,code=$2,name=$3,symbol=$4,allows_decimal=$5,updated_at=NOW() WHERE id=$6 RETURNING id,measurement_group_id,code,name,symbol,allows_decimal,created_at,updated_at`, nullableStringValue(req.MeasurementGroupID), strings.ToUpper(strings.TrimSpace(req.Code)), strings.TrimSpace(req.Name), nullableStringValue(req.Symbol), allows, c.Params("unitId")).Scan(&item.ID, &group, &item.Code, &item.Name, &symbol, &item.AllowsDecimal, &item.CreatedAt, &item.UpdatedAt)
	if err == pgx.ErrNoRows {
		return fiber.NewError(404, "unit definition not found")
	}
	if err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "unit code already exists")
		}
		return fiber.NewError(500, "failed to update unit definition")
	}
	if group.Valid {
		item.MeasurementGroupID = &group.String
	}
	if symbol.Valid {
		item.Symbol = &symbol.String
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleDeleteUnitDefinition(c *fiber.Ctx) error {
	result, err := database.GetDB().Exec(context.Background(), `DELETE FROM unit_definitions WHERE id=$1`, c.Params("unitId"))
	if err != nil {
		return fiber.NewError(409, "unit definition cannot be deleted because it is in use")
	}
	if result.RowsAffected() == 0 {
		return fiber.NewError(404, "unit definition not found")
	}
	return c.SendStatus(204)
}

func stockItemOwned(ctx context.Context, stockItemID, merchantID string) (bool, error) {
	var ok bool
	err := database.GetDB().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM stock_items WHERE id=$1 AND merchant_id=$2)`, stockItemID, merchantID).Scan(&ok)
	return ok, err
}

func HandleGetStockItemConfiguration(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	ok, err := stockItemOwned(context.Background(), c.Params("stockItemId"), merchantID)
	if err != nil {
		return fiber.NewError(500, "failed to validate stock item")
	}
	if !ok {
		return fiber.NewError(404, "stock item not found")
	}
	var item models.StockItemConfiguration
	err = database.GetDB().QueryRow(context.Background(), `SELECT stock_item_id,track_batches,track_expiry,track_unique_assets,track_reservations,allow_unit_conversions,allow_pack_breaking,allow_multiple_barcodes,created_at FROM stock_item_configurations WHERE stock_item_id=$1`, c.Params("stockItemId")).Scan(&item.StockItemID, &item.TrackBatches, &item.TrackExpiry, &item.TrackUniqueAssets, &item.TrackReservations, &item.AllowUnitConversions, &item.AllowPackBreaking, &item.AllowMultipleBarcodes, &item.CreatedAt)
	if err == pgx.ErrNoRows {
		return c.JSON(fiber.Map{"status": "success", "success": true, "data": models.StockItemConfiguration{StockItemID: c.Params("stockItemId")}})
	}
	if err != nil {
		return fiber.NewError(500, "failed to retrieve stock configuration")
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleUpsertStockItemConfiguration(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	ok, err := stockItemOwned(context.Background(), c.Params("stockItemId"), merchantID)
	if err != nil {
		return fiber.NewError(500, "failed to validate stock item")
	}
	if !ok {
		return fiber.NewError(404, "stock item not found")
	}
	var req models.StockItemConfigurationRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(400, "invalid request body")
	}
	b := func(v *bool) bool {
		if v != nil {
			return *v
		}
		return false
	}
	var item models.StockItemConfiguration
	err = database.GetDB().QueryRow(context.Background(), `INSERT INTO stock_item_configurations(stock_item_id,track_batches,track_expiry,track_unique_assets,track_reservations,allow_unit_conversions,allow_pack_breaking,allow_multiple_barcodes) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(stock_item_id) DO UPDATE SET track_batches=EXCLUDED.track_batches,track_expiry=EXCLUDED.track_expiry,track_unique_assets=EXCLUDED.track_unique_assets,track_reservations=EXCLUDED.track_reservations,allow_unit_conversions=EXCLUDED.allow_unit_conversions,allow_pack_breaking=EXCLUDED.allow_pack_breaking,allow_multiple_barcodes=EXCLUDED.allow_multiple_barcodes RETURNING stock_item_id,track_batches,track_expiry,track_unique_assets,track_reservations,allow_unit_conversions,allow_pack_breaking,allow_multiple_barcodes,created_at`, c.Params("stockItemId"), b(req.TrackBatches), b(req.TrackExpiry), b(req.TrackUniqueAssets), b(req.TrackReservations), b(req.AllowUnitConversions), b(req.AllowPackBreaking), b(req.AllowMultipleBarcodes)).Scan(&item.StockItemID, &item.TrackBatches, &item.TrackExpiry, &item.TrackUniqueAssets, &item.TrackReservations, &item.AllowUnitConversions, &item.AllowPackBreaking, &item.AllowMultipleBarcodes, &item.CreatedAt)
	if err != nil {
		return fiber.NewError(500, "failed to save stock configuration")
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

var stockUnitSortFields = map[string]string{"position": "position", "conversionToBase": "conversion_to_base"}

func HandleListStockItemUnits(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	ok, err := stockItemOwned(context.Background(), c.Params("stockItemId"), merchantID)
	if err != nil {
		return fiber.NewError(500, "failed to validate stock item")
	}
	if !ok {
		return fiber.NewError(404, "stock item not found")
	}
	q := getCatalogListQuery(c, "position", stockUnitSortFields)
	where := " WHERE su.stock_item_id=$1"
	args := []interface{}{c.Params("stockItemId")}
	if q.Search != "" {
		where += " AND (u.code ILIKE $2 OR u.name ILIKE $2 OR COALESCE(u.symbol,'') ILIKE $2)"
		args = append(args, "%"+q.Search+"%")
	}
	db := database.GetDB()
	ctx := context.Background()
	var total int64
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM stock_item_units su JOIN unit_definitions u ON u.id=su.unit_id"+where, args...).Scan(&total); err != nil {
		return fiber.NewError(500, "failed to count stock item units")
	}
	rows, err := db.Query(ctx, "SELECT su.id,su.stock_item_id,su.unit_id,su.conversion_to_base,su.is_base_unit,su.is_sales_unit,su.is_purchase_unit,su.allows_fractional,su.position FROM stock_item_units su JOIN unit_definitions u ON u.id=su.unit_id"+where+q.orderBy()+" LIMIT $"+itoa(len(args)+1)+" OFFSET $"+itoa(len(args)+2), append(args, q.PageSize, q.Offset)...)
	if err != nil {
		return fiber.NewError(500, "failed to list stock item units")
	}
	defer rows.Close()
	items := make([]models.StockItemUnit, 0)
	for rows.Next() {
		var item models.StockItemUnit
		if err := rows.Scan(&item.ID, &item.StockItemID, &item.UnitID, &item.ConversionToBase, &item.IsBaseUnit, &item.IsSalesUnit, &item.IsPurchaseUnit, &item.AllowsFractional, &item.Position); err != nil {
			return fiber.NewError(500, "failed to read stock item unit")
		}
		items = append(items, item)
	}
	return c.JSON(paginatedResponse(items, total, q))
}

func HandleCreateStockItemUnit(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	ok, err := stockItemOwned(context.Background(), c.Params("stockItemId"), merchantID)
	if err != nil {
		return fiber.NewError(500, "failed to validate stock item")
	}
	if !ok {
		return fiber.NewError(404, "stock item not found")
	}
	var req models.StockItemUnitRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.UnitID) == "" || req.ConversionToBase <= 0 {
		return fiber.NewError(400, "unitId and a positive conversionToBase are required")
	}
	var item models.StockItemUnit
	err = database.GetDB().QueryRow(context.Background(), `INSERT INTO stock_item_units(stock_item_id,unit_id,conversion_to_base,is_base_unit,is_sales_unit,is_purchase_unit,allows_fractional,position) SELECT $1,$2,$3,$4,$5,$6,$7,$8 WHERE EXISTS(SELECT 1 FROM unit_definitions WHERE id=$2) RETURNING id,stock_item_id,unit_id,conversion_to_base,is_base_unit,is_sales_unit,is_purchase_unit,allows_fractional,position`, c.Params("stockItemId"), req.UnitID, req.ConversionToBase, req.IsBaseUnit, req.IsSalesUnit, req.IsPurchaseUnit, req.AllowsFractional, req.Position).Scan(&item.ID, &item.StockItemID, &item.UnitID, &item.ConversionToBase, &item.IsBaseUnit, &item.IsSalesUnit, &item.IsPurchaseUnit, &item.AllowsFractional, &item.Position)
	if err == pgx.ErrNoRows {
		return fiber.NewError(404, "unit definition not found")
	}
	if err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "unit is already assigned to this stock item")
		}
		return fiber.NewError(500, "failed to create stock item unit")
	}
	return c.Status(201).JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleUpdateStockItemUnit(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	var req models.StockItemUnitRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.UnitID) == "" || req.ConversionToBase <= 0 {
		return fiber.NewError(400, "unitId and a positive conversionToBase are required")
	}
	var item models.StockItemUnit
	err = database.GetDB().QueryRow(context.Background(), `UPDATE stock_item_units su SET unit_id=$1,conversion_to_base=$2,is_base_unit=$3,is_sales_unit=$4,is_purchase_unit=$5,allows_fractional=$6,position=$7 WHERE su.id=$8 AND EXISTS(SELECT 1 FROM stock_items si WHERE si.id=su.stock_item_id AND si.merchant_id=$9) AND EXISTS(SELECT 1 FROM unit_definitions WHERE id=$1) RETURNING su.id,su.stock_item_id,su.unit_id,su.conversion_to_base,su.is_base_unit,su.is_sales_unit,su.is_purchase_unit,su.allows_fractional,su.position`, req.UnitID, req.ConversionToBase, req.IsBaseUnit, req.IsSalesUnit, req.IsPurchaseUnit, req.AllowsFractional, req.Position, c.Params("stockUnitId"), merchantID).Scan(&item.ID, &item.StockItemID, &item.UnitID, &item.ConversionToBase, &item.IsBaseUnit, &item.IsSalesUnit, &item.IsPurchaseUnit, &item.AllowsFractional, &item.Position)
	if err == pgx.ErrNoRows {
		return fiber.NewError(404, "stock item unit not found")
	}
	if err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "unit is already assigned to this stock item")
		}
		return fiber.NewError(500, "failed to update stock item unit")
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleDeleteStockItemUnit(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	result, err := database.GetDB().Exec(context.Background(), `DELETE FROM stock_item_units su USING stock_items si WHERE su.id=$1 AND si.id=su.stock_item_id AND si.merchant_id=$2`, c.Params("stockUnitId"), merchantID)
	if err != nil {
		return fiber.NewError(409, "stock item unit cannot be deleted because it is in use")
	}
	if result.RowsAffected() == 0 {
		return fiber.NewError(404, "stock item unit not found")
	}
	return c.SendStatus(204)
}

var conversionSortFields = map[string]string{"factor": "factor"}

func HandleListStockItemUnitConversions(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	ok, err := stockItemOwned(context.Background(), c.Params("stockItemId"), merchantID)
	if err != nil {
		return fiber.NewError(500, "failed to validate stock item")
	}
	if !ok {
		return fiber.NewError(404, "stock item not found")
	}
	q := getCatalogListQuery(c, "factor", conversionSortFields)
	where := " WHERE c.stock_item_id=$1"
	args := []interface{}{c.Params("stockItemId")}
	db := database.GetDB()
	ctx := context.Background()
	var total int64
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM stock_item_unit_conversions c"+where, args...).Scan(&total); err != nil {
		return fiber.NewError(500, "failed to count unit conversions")
	}
	rows, err := db.Query(ctx, "SELECT c.id,c.stock_item_id,c.from_unit_id,c.to_unit_id,c.factor FROM stock_item_unit_conversions c"+where+q.orderBy()+" LIMIT $2 OFFSET $3", append(args, q.PageSize, q.Offset)...)
	if err != nil {
		return fiber.NewError(500, "failed to list unit conversions")
	}
	defer rows.Close()
	items := make([]models.StockItemUnitConversion, 0)
	for rows.Next() {
		var item models.StockItemUnitConversion
		if err := rows.Scan(&item.ID, &item.StockItemID, &item.FromUnitID, &item.ToUnitID, &item.Factor); err != nil {
			return fiber.NewError(500, "failed to read unit conversion")
		}
		items = append(items, item)
	}
	return c.JSON(paginatedResponse(items, total, q))
}

func HandleCreateStockItemUnitConversion(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	ok, err := stockItemOwned(context.Background(), c.Params("stockItemId"), merchantID)
	if err != nil {
		return fiber.NewError(500, "failed to validate stock item")
	}
	if !ok {
		return fiber.NewError(404, "stock item not found")
	}
	var req models.StockItemUnitConversionRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.FromUnitID) == "" || strings.TrimSpace(req.ToUnitID) == "" || req.Factor <= 0 {
		return fiber.NewError(400, "fromUnitId, toUnitId, and positive factor are required")
	}
	var item models.StockItemUnitConversion
	err = database.GetDB().QueryRow(context.Background(), `INSERT INTO stock_item_unit_conversions(stock_item_id,from_unit_id,to_unit_id,factor) SELECT $1,$2,$3,$4 WHERE EXISTS(SELECT 1 FROM stock_item_units WHERE stock_item_id=$1 AND unit_id=$2) AND EXISTS(SELECT 1 FROM stock_item_units WHERE stock_item_id=$1 AND unit_id=$3) RETURNING id,stock_item_id,from_unit_id,to_unit_id,factor`, c.Params("stockItemId"), req.FromUnitID, req.ToUnitID, req.Factor).Scan(&item.ID, &item.StockItemID, &item.FromUnitID, &item.ToUnitID, &item.Factor)
	if err == pgx.ErrNoRows {
		return fiber.NewError(400, "both units must be assigned to this stock item")
	}
	if err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "unit conversion already exists")
		}
		return fiber.NewError(500, "failed to create unit conversion")
	}
	return c.Status(201).JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleDeleteStockItemUnitConversion(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	result, err := database.GetDB().Exec(context.Background(), `DELETE FROM stock_item_unit_conversions c USING stock_items si WHERE c.id=$1 AND si.id=c.stock_item_id AND si.merchant_id=$2`, c.Params("conversionId"), merchantID)
	if err != nil {
		return fiber.NewError(500, "failed to delete unit conversion")
	}
	if result.RowsAffected() == 0 {
		return fiber.NewError(404, "unit conversion not found")
	}
	return c.SendStatus(204)
}
