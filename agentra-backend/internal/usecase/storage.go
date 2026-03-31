package usecase

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	adapterStorage "github.com/reaganiwadha/agentra/internal/adapter/storage"
	"github.com/reaganiwadha/agentra/internal/domain"
	"github.com/reaganiwadha/agentra/internal/repository/postgres"
	"gopkg.in/guregu/null.v4"
)

type StorageUsecase struct {
	storage *postgres.StorageRepo
	orgs    *postgres.OrgRepo
}

func NewStorageUsecase(storage *postgres.StorageRepo, orgs *postgres.OrgRepo) *StorageUsecase {
	return &StorageUsecase{storage: storage, orgs: orgs}
}

func (u *StorageUsecase) Get(ctx context.Context, orgID uuid.UUID) (domain.StorageConfig, error) {
	cfg, err := u.storage.GetActiveByOrg(ctx, orgID)
	if err != nil {
		return domain.StorageConfig{}, ErrNotFound.New("no active storage config")
	}
	return cfg, nil
}

type StorageInput struct {
	StorageType    domain.StorageType
	Endpoint       string
	AccessKey      string
	SecretKey      string
	Bucket         string
	BasePath       string
	OutputBasePath string
}

func (u *StorageUsecase) Set(ctx context.Context, orgID uuid.UUID, in StorageInput) (domain.StorageConfig, error) {
	now := time.Now()
	cfg := domain.StorageConfig{
		ID:             uuid.New(),
		OrganizationID: orgID,
		StorageType:    in.StorageType,
		Endpoint:       null.StringFrom(in.Endpoint),
		AccessKey:      null.StringFrom(in.AccessKey),
		SecretKey:      null.StringFrom(in.SecretKey),
		Bucket:         null.StringFrom(in.Bucket),
		BasePath:       null.StringFrom(in.BasePath),
		OutputBasePath: null.StringFrom(in.OutputBasePath),
		IsActive:       true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := u.storage.Upsert(ctx, cfg); err != nil {
		return domain.StorageConfig{}, err
	}
	return cfg, nil
}

type StorageProbeResult struct {
	Healthy  bool      `json:"healthy"`
	CanRead  bool      `json:"can_read"`
	CanWrite bool      `json:"can_write"`
	Message  string    `json:"message"`
	Checked  time.Time `json:"checked_at"`
}

type StorageStatus struct {
	Configured bool                  `json:"configured"`
	Config     *domain.StorageConfig `json:"config,omitempty"`
	Probe      StorageProbeResult    `json:"probe"`
}

func (u *StorageUsecase) Validate(ctx context.Context, in StorageInput) (StorageProbeResult, error) {
	if in.StorageType != domain.StorageMinIO {
		return StorageProbeResult{
			Healthy: false,
			Message: "only minio validation is supported in setup right now",
			Checked: time.Now(),
		}, nil
	}

	cfg := toStorageConfig(uuid.Nil, in)
	return u.probe(ctx, cfg)
}

func (u *StorageUsecase) Status(ctx context.Context, orgID uuid.UUID) (StorageStatus, error) {
	cfg, err := u.Get(ctx, orgID)
	if err != nil {
		return StorageStatus{
			Configured: false,
			Probe: StorageProbeResult{
				Healthy: false,
				Message: "no active storage config",
				Checked: time.Now(),
			},
		}, nil
	}

	probe, probeErr := u.probe(ctx, cfg)
	if probeErr != nil {
		return StorageStatus{}, probeErr
	}

	cfg.HasSecret = cfg.SecretKey.String != ""
	return StorageStatus{
		Configured: true,
		Config:     &cfg,
		Probe:      probe,
	}, nil
}

func toStorageConfig(orgID uuid.UUID, in StorageInput) domain.StorageConfig {
	now := time.Now()
	return domain.StorageConfig{
		ID:             uuid.New(),
		OrganizationID: orgID,
		StorageType:    in.StorageType,
		Endpoint:       null.StringFrom(strings.TrimSpace(in.Endpoint)),
		AccessKey:      null.StringFrom(strings.TrimSpace(in.AccessKey)),
		SecretKey:      null.StringFrom(strings.TrimSpace(in.SecretKey)),
		Bucket:         null.StringFrom(strings.TrimSpace(in.Bucket)),
		BasePath:       null.StringFrom(strings.TrimSpace(in.BasePath)),
		OutputBasePath: null.StringFrom(strings.TrimSpace(in.OutputBasePath)),
		IsActive:       true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func (u *StorageUsecase) probe(ctx context.Context, cfg domain.StorageConfig) (StorageProbeResult, error) {
	checked := time.Now()
	if cfg.StorageType != domain.StorageMinIO {
		return StorageProbeResult{
			Healthy: false,
			Message: "realtime probing is currently implemented for minio only",
			Checked: checked,
		}, nil
	}

	if !cfg.Endpoint.Valid || !cfg.AccessKey.Valid || !cfg.SecretKey.Valid || !cfg.Bucket.Valid {
		return StorageProbeResult{
			Healthy: false,
			Message: "endpoint, access_key, secret_key, and bucket are required",
			Checked: checked,
		}, nil
	}

	adapter, err := adapterStorage.NewAdapter(cfg)
	if err != nil {
		return StorageProbeResult{
			Healthy: false,
			Message: fmt.Sprintf("adapter init failed: %v", err),
			Checked: checked,
		}, nil
	}

	probePath := strings.Trim(cfg.OutputBasePath.String, "/")
	if probePath != "" {
		probePath += "/"
	}
	probePath += path.Join(".agentra-probe", fmt.Sprintf("probe-%d.txt", time.Now().UnixNano()))

	payload := []byte("agentra storage probe")
	if err := adapter.WriteFile(ctx, probePath, bytes.NewReader(payload), int64(len(payload))); err != nil {
		return StorageProbeResult{
			Healthy:  false,
			CanRead:  false,
			CanWrite: false,
			Message:  fmt.Sprintf("write failed: %v", err),
			Checked:  checked,
		}, nil
	}

	canRead := false
	rc, _, err := adapter.GetFile(ctx, probePath)
	if err == nil {
		defer rc.Close()
		if _, readErr := io.ReadAll(rc); readErr == nil {
			canRead = true
		}
	}
	_ = adapter.DeleteFile(ctx, probePath)

	if !canRead {
		return StorageProbeResult{
			Healthy:  false,
			CanRead:  false,
			CanWrite: true,
			Message:  "write ok but read back failed",
			Checked:  checked,
		}, nil
	}

	return StorageProbeResult{
		Healthy:  true,
		CanRead:  true,
		CanWrite: true,
		Message:  "read/write probe succeeded",
		Checked:  checked,
	}, nil
}
