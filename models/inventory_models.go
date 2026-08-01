package models

import "time"

type MeasurementGroup struct {
	ID        string    `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type MeasurementGroupRequest struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type UnitDefinition struct {
	ID                 string    `json:"id"`
	MeasurementGroupID *string   `json:"measurementGroupId,omitempty"`
	Code               string    `json:"code"`
	Name               string    `json:"name"`
	Symbol             *string   `json:"symbol,omitempty"`
	AllowsDecimal      bool      `json:"allowsDecimal"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type UnitDefinitionRequest struct {
	MeasurementGroupID *string `json:"measurementGroupId,omitempty"`
	Code               string  `json:"code"`
	Name               string  `json:"name"`
	Symbol             *string `json:"symbol,omitempty"`
	AllowsDecimal      *bool   `json:"allowsDecimal,omitempty"`
}

type StockItemConfiguration struct {
	StockItemID           string    `json:"stockItemId"`
	TrackBatches          bool      `json:"trackBatches"`
	TrackExpiry           bool      `json:"trackExpiry"`
	TrackUniqueAssets     bool      `json:"trackUniqueAssets"`
	TrackReservations     bool      `json:"trackReservations"`
	AllowUnitConversions  bool      `json:"allowUnitConversions"`
	AllowPackBreaking     bool      `json:"allowPackBreaking"`
	AllowMultipleBarcodes bool      `json:"allowMultipleBarcodes"`
	CreatedAt             time.Time `json:"createdAt"`
}

type StockItemConfigurationRequest struct {
	TrackBatches          *bool `json:"trackBatches,omitempty"`
	TrackExpiry           *bool `json:"trackExpiry,omitempty"`
	TrackUniqueAssets     *bool `json:"trackUniqueAssets,omitempty"`
	TrackReservations     *bool `json:"trackReservations,omitempty"`
	AllowUnitConversions  *bool `json:"allowUnitConversions,omitempty"`
	AllowPackBreaking     *bool `json:"allowPackBreaking,omitempty"`
	AllowMultipleBarcodes *bool `json:"allowMultipleBarcodes,omitempty"`
}

type StockItemUnit struct {
	ID               string  `json:"id"`
	StockItemID      string  `json:"stockItemId"`
	UnitID           string  `json:"unitId"`
	ConversionToBase float64 `json:"conversionToBase"`
	IsBaseUnit       bool    `json:"isBaseUnit"`
	IsSalesUnit      bool    `json:"isSalesUnit"`
	IsPurchaseUnit   bool    `json:"isPurchaseUnit"`
	AllowsFractional bool    `json:"allowsFractional"`
	Position         int     `json:"position"`
}

type StockItemUnitRequest struct {
	UnitID           string  `json:"unitId"`
	ConversionToBase float64 `json:"conversionToBase"`
	IsBaseUnit       bool    `json:"isBaseUnit"`
	IsSalesUnit      bool    `json:"isSalesUnit"`
	IsPurchaseUnit   bool    `json:"isPurchaseUnit"`
	AllowsFractional bool    `json:"allowsFractional"`
	Position         int     `json:"position"`
}

type StockItemUnitConversion struct {
	ID          string  `json:"id"`
	StockItemID string  `json:"stockItemId"`
	FromUnitID  string  `json:"fromUnitId"`
	ToUnitID    string  `json:"toUnitId"`
	Factor      float64 `json:"factor"`
}

type StockItemUnitConversionRequest struct {
	FromUnitID string  `json:"fromUnitId"`
	ToUnitID   string  `json:"toUnitId"`
	Factor     float64 `json:"factor"`
}

type InventoryIdentifierType struct {
	ID              string    `json:"id"`
	MerchantID      string    `json:"merchantId"`
	Code            string    `json:"code"`
	Name            string    `json:"name"`
	Description     *string   `json:"description,omitempty"`
	ValidationRegex *string   `json:"validationRegex,omitempty"`
	IsActive        bool      `json:"isActive"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type InventoryIdentifierTypeRequest struct {
	Code            string  `json:"code"`
	Name            string  `json:"name"`
	Description     *string `json:"description,omitempty"`
	ValidationRegex *string `json:"validationRegex,omitempty"`
	IsActive        *bool   `json:"isActive,omitempty"`
}

type BarcodeRegistryEntry struct {
	ID             string                 `json:"id"`
	MerchantID     string                 `json:"merchantId"`
	Code           string                 `json:"code"`
	NormalizedCode string                 `json:"normalizedCode"`
	OwnerType      string                 `json:"ownerType"`
	OwnerID        string                 `json:"ownerId"`
	IsPrimary      bool                   `json:"isPrimary"`
	IsGenerated    bool                   `json:"isGenerated"`
	IsActive       bool                   `json:"isActive"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt      time.Time              `json:"createdAt"`
}

type BarcodeRegistryRequest struct {
	Code        string                 `json:"code"`
	OwnerType   string                 `json:"ownerType"`
	OwnerID     string                 `json:"ownerId"`
	IsPrimary   bool                   `json:"isPrimary"`
	IsGenerated bool                   `json:"isGenerated"`
	IsActive    *bool                  `json:"isActive,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type InventoryBatch struct {
	ID                string     `json:"id"`
	MerchantID        string     `json:"merchantId"`
	ShopID            string     `json:"shopId"`
	InventoryItemID   string     `json:"inventoryItemId"`
	ProductID         string     `json:"productId"`
	StockItemID       *string    `json:"stockItemId,omitempty"`
	BatchCode         string     `json:"batchCode"`
	QuantityReceived  float64    `json:"quantityReceived"`
	QuantityRemaining float64    `json:"quantityRemaining"`
	UnitCost          float64    `json:"unitCost"`
	ManufactureDate   *time.Time `json:"manufactureDate,omitempty"`
	ExpiryDate        *time.Time `json:"expiryDate,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
}

type InventoryBatchRequest struct {
	BatchCode        string     `json:"batchCode"`
	QuantityReceived float64    `json:"quantityReceived"`
	UnitCost         float64    `json:"unitCost"`
	ManufactureDate  *time.Time `json:"manufactureDate,omitempty"`
	ExpiryDate       *time.Time `json:"expiryDate,omitempty"`
}

type InventoryReservation struct {
	ID              string     `json:"id"`
	MerchantID      string     `json:"merchantId"`
	ShopID          string     `json:"shopId"`
	InventoryItemID string     `json:"inventoryItemId"`
	ProductID       string     `json:"productId"`
	StockItemID     *string    `json:"stockItemId,omitempty"`
	UnitID          *string    `json:"unitId,omitempty"`
	ReferenceID     *string    `json:"referenceId,omitempty"`
	ReservationKey  string     `json:"reservationKey"`
	Quantity        float64    `json:"quantity"`
	BaseQuantity    *float64   `json:"baseQuantity,omitempty"`
	Status          string     `json:"status"`
	ReleasedAt      *time.Time `json:"releasedAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type InventoryReservationRequest struct {
	ShopID         string   `json:"shopId"`
	ReservationKey string   `json:"reservationKey"`
	Quantity       float64  `json:"quantity"`
	BaseQuantity   *float64 `json:"baseQuantity,omitempty"`
	UnitID         *string  `json:"unitId,omitempty"`
	ReferenceID    *string  `json:"referenceId,omitempty"`
}

type InventorySerial struct {
	ID              string    `json:"id"`
	MerchantID      string    `json:"merchantId"`
	ShopID          string    `json:"shopId"`
	InventoryItemID string    `json:"inventoryItemId"`
	ProductID       string    `json:"productId"`
	StockItemID     *string   `json:"stockItemId,omitempty"`
	SerialNumber    string    `json:"serialNumber"`
	Status          string    `json:"status"`
	ReferenceID     *string   `json:"referenceId,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
}

type InventorySerialRequest struct {
	ShopID       string  `json:"shopId"`
	SerialNumber string  `json:"serialNumber"`
	ReferenceID  *string `json:"referenceId,omitempty"`
	Status       *string `json:"status,omitempty"`
}

type InventoryAsset struct {
	ID              string                 `json:"id"`
	MerchantID      string                 `json:"merchantId"`
	ShopID          string                 `json:"shopId"`
	InventoryItemID string                 `json:"inventoryItemId"`
	BatchID         *string                `json:"batchId,omitempty"`
	AssetTag        string                 `json:"assetTag"`
	Status          string                 `json:"status"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt       time.Time              `json:"createdAt"`
}

type InventoryAssetRequest struct {
	ShopID   string                 `json:"shopId"`
	BatchID  *string                `json:"batchId,omitempty"`
	AssetTag string                 `json:"assetTag"`
	Status   *string                `json:"status,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type InventoryAssetIdentifier struct {
	ID               string `json:"id"`
	AssetID          string `json:"assetId"`
	IdentifierTypeID string `json:"identifierTypeId"`
	Value            string `json:"value"`
	NormalizedValue  string `json:"normalizedValue"`
	IsPrimary        bool   `json:"isPrimary"`
}

type InventoryAssetIdentifierRequest struct {
	IdentifierTypeID string `json:"identifierTypeId"`
	Value            string `json:"value"`
	IsPrimary        bool   `json:"isPrimary"`
}
