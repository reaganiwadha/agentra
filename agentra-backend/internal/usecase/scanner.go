package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"
	storageadapter "github.com/reaganiwadha/agentra/internal/adapter/storage"
	"github.com/reaganiwadha/agentra/internal/domain"
	"github.com/reaganiwadha/agentra/internal/repository/postgres"
	"github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v4"
)

type ScannerUsecase struct {
	storage  *postgres.StorageRepo
	projects *postgres.ProjectRepo
	media    *postgres.MediaAssetRepo
	log      *logrus.Logger
}

func NewScannerUsecase(
	storage *postgres.StorageRepo,
	projects *postgres.ProjectRepo,
	media *postgres.MediaAssetRepo,
	log *logrus.Logger,
) *ScannerUsecase {
	return &ScannerUsecase{storage: storage, projects: projects, media: media, log: log}
}

func (u *ScannerUsecase) Scan(ctx context.Context) error {
	cfg, err := u.storage.GetActive(ctx)
	if err != nil {
		return nil // no active storage configured yet
	}

	adapter, err := storageadapter.NewAdapter(cfg)
	if err != nil {
		return err
	}

	projects, err := u.projects.List(ctx, cfg.OrganizationID)
	if err != nil {
		return err
	}

	for _, project := range projects {
		if !project.StorageSubpath.Valid {
			continue
		}
		if err := u.scanProject(ctx, adapter, cfg, project); err != nil {
			u.log.WithError(err).WithField("project_id", project.ID).Warn("scanner: project scan failed")
		}
	}
	return nil
}

func (u *ScannerUsecase) scanProject(ctx context.Context, adapter storageadapter.Adapter, cfg domain.StorageConfig, project domain.Project) error {
	basePath := cfg.BasePath.String
	if project.StorageSubpath.Valid {
		basePath = basePath + "/" + project.StorageSubpath.String
	}

	files, err := adapter.ListFiles(ctx, basePath)
	if err != nil {
		return err
	}

	now := time.Now()
	for _, f := range files {
		asset := domain.MediaAsset{
			ID:             uuid.New(),
			OrganizationID: project.OrganizationID,
			ProjectID:      uuid.NullUUID{UUID: project.ID, Valid: true},
			Filename:       f.Filename,
			StoragePath:    f.StoragePath,
			FileSizeBytes:  null.IntFrom(f.SizeBytes),
			Status:         domain.MediaPending,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := u.media.Upsert(ctx, asset); err != nil {
			u.log.WithError(err).WithField("storage_path", f.StoragePath).Warn("scanner: upsert failed")
		}
	}
	return nil
}
