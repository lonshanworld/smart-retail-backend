package models

import (
	"encoding/json"
	"time"
)

type ProductVariant struct {
	ID         string          `json:"id"`
	MerchantID string          `json:"merchantId"`
	ProductID  string          `json:"productId"`
	Name       string          `json:"name"`
	SKU        string          `json:"sku"`
	Barcode    *string         `json:"barcode,omitempty"`
	Attributes json.RawMessage `json:"attributes"`
	IsActive   bool            `json:"isActive"`
	DeletedAt  *time.Time      `json:"deletedAt,omitempty"`
	CreatedAt  time.Time       `json:"createdAt"`
	UpdatedAt  time.Time       `json:"updatedAt"`
}

type ProductVariantRequest struct {
	Name       string          `json:"name"`
	SKU        string          `json:"sku"`
	Barcode    *string         `json:"barcode,omitempty"`
	Attributes json.RawMessage `json:"attributes,omitempty"`
	IsActive   *bool           `json:"isActive,omitempty"`
}

type ProductImage struct {
	ID                string                 `json:"id"`
	ProductID         string                 `json:"productId"`
	URL               string                 `json:"url"`
	SourceType        string                 `json:"sourceType"`
	OriginalURL       *string                `json:"originalUrl,omitempty"`
	StorageProvider   *string                `json:"storageProvider,omitempty"`
	StoragePublicID   *string                `json:"storagePublicId,omitempty"`
	StorageObjectName *string                `json:"storageObjectName,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	Position          int                    `json:"position"`
	CreatedAt         time.Time              `json:"createdAt"`
}

type ProductImageRequest struct {
	URL        string                 `json:"url"`
	SourceType string                 `json:"sourceType,omitempty"`
	Position   *int                   `json:"position,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

type AttributeDefinition struct {
	ID          string    `json:"id"`
	MerchantID  string    `json:"merchantId"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	ValueType   string    `json:"valueType"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type AttributeDefinitionRequest struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	ValueType   string  `json:"valueType"`
	Description *string `json:"description,omitempty"`
}

type AttributeOption struct {
	ID           string `json:"id"`
	DefinitionID string `json:"definitionId"`
	Value        string `json:"value"`
	Label        string `json:"label"`
	Position     int    `json:"position"`
}

type AttributeOptionRequest struct {
	Value    string `json:"value"`
	Label    string `json:"label"`
	Position *int   `json:"position,omitempty"`
}

type ProductAttributeAssignment struct {
	ID           string          `json:"id"`
	DefinitionID string          `json:"definitionId"`
	ProductID    string          `json:"productId"`
	VariantID    *string         `json:"variantId,omitempty"`
	ValueText    *string         `json:"valueText,omitempty"`
	ValueNumber  *float64        `json:"valueNumber,omitempty"`
	ValueBoolean *bool           `json:"valueBoolean,omitempty"`
	ValueJSON    json.RawMessage `json:"valueJson,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
}

type ProductAttributeAssignmentRequest struct {
	DefinitionID string          `json:"definitionId"`
	VariantID    *string         `json:"variantId,omitempty"`
	ValueText    *string         `json:"valueText,omitempty"`
	ValueNumber  *float64        `json:"valueNumber,omitempty"`
	ValueBoolean *bool           `json:"valueBoolean,omitempty"`
	ValueJSON    json.RawMessage `json:"valueJson,omitempty"`
}
