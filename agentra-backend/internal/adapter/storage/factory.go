package storage

import (
	"fmt"

	"github.com/reaganiwadha/agentra/internal/domain"
)

func NewAdapter(cfg domain.StorageConfig) (Adapter, error) {
	switch cfg.StorageType {
	case domain.StorageSMB:
		return newSMBAdapter(cfg), nil
	case domain.StorageMinIO:
		return newMinIOAdapter(cfg)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", cfg.StorageType)
	}
}
