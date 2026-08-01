package models

import "time"

type AuditLog struct {
	ID         string                 `json:"id"`
	ActorID    *string                `json:"actorId,omitempty"`
	Action     string                 `json:"action"`
	EntityType string                 `json:"entityType"`
	EntityID   string                 `json:"entityId"`
	BeforeData map[string]interface{} `json:"beforeData,omitempty"`
	AfterData  map[string]interface{} `json:"afterData,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	IPAddress  *string                `json:"ipAddress,omitempty"`
	UserAgent  *string                `json:"userAgent,omitempty"`
	CreatedAt  time.Time              `json:"createdAt"`
}

type SystemSetting struct {
	ID        string                 `json:"id"`
	Key       string                 `json:"key"`
	Value     map[string]interface{} `json:"value"`
	UpdatedBy *string                `json:"updatedBy,omitempty"`
	CreatedAt time.Time              `json:"createdAt"`
	UpdatedAt time.Time              `json:"updatedAt"`
}

type SystemSettingRequest struct { Value map[string]interface{} `json:"value"` }

type SyncLog struct {
	ID              string     `json:"id"`
	MerchantID      string     `json:"merchantId"`
	DeviceID        *string    `json:"deviceId,omitempty"`
	TotalSales      int        `json:"totalSales"`
	SuccessfulSyncs int        `json:"successfulSyncs"`
	FailedSyncs     int        `json:"failedSyncs"`
	ErrorMessage    *string    `json:"errorMessage,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
}
