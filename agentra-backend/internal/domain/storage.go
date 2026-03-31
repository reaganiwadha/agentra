package domain

import (
	"time"

	"github.com/google/uuid"
	"gopkg.in/guregu/null.v4"
)

type StorageType string

const (
	StorageSMB   StorageType = "smb"
	StorageMinIO StorageType = "minio"
)

type StorageConfig struct {
	ID             uuid.UUID   `db:"id"               json:"id"`
	OrganizationID uuid.UUID   `db:"organization_id"  json:"organization_id"`
	StorageType    StorageType `db:"storage_type"     json:"storage_type"`
	Endpoint       null.String `db:"endpoint"         json:"endpoint"`
	AccessKey      null.String `db:"access_key"       json:"access_key"`
	SecretKey      null.String `db:"secret_key"       json:"-"`
	Bucket         null.String `db:"bucket"           json:"bucket"`
	BasePath       null.String `db:"base_path"        json:"base_path"`
	OutputBasePath null.String `db:"output_base_path" json:"output_base_path"`
	IsActive       bool        `db:"is_active"        json:"is_active"`
	HasSecret      bool        `db:"-"                json:"has_secret"`
	CreatedAt      time.Time   `db:"created_at"       json:"created_at"`
	UpdatedAt      time.Time   `db:"updated_at"       json:"updated_at"`
}
