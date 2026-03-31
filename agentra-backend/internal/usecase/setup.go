package usecase

import (
	"context"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/reaganiwadha/agentra/internal/bootstrap"
	"github.com/reaganiwadha/agentra/internal/domain"
	"github.com/reaganiwadha/agentra/internal/repository/postgres"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/guregu/null.v4"
)

type SetupUsecase struct {
	orgs    *postgres.OrgRepo
	users   *postgres.UserRepo
	storage *postgres.StorageRepo
}

type SetupStorageOption struct {
	Type            domain.StorageType `json:"type"`
	Label           string             `json:"label"`
	Description     string             `json:"description"`
	StartsEmpty     bool               `json:"starts_empty"`
	DefaultEndpoint string             `json:"default_endpoint"`
	DefaultAccess   string             `json:"default_access_key"`
	DefaultSecret   string             `json:"default_secret_key"`
	DefaultBucket   string             `json:"default_bucket"`
	DefaultBasePath string             `json:"default_base_path"`
	DefaultOutput   string             `json:"default_output_base_path"`
}

func NewSetupUsecase(orgs *postgres.OrgRepo, users *postgres.UserRepo, storage *postgres.StorageRepo) *SetupUsecase {
	return &SetupUsecase{orgs: orgs, users: users, storage: storage}
}

func (u *SetupUsecase) NeedsSetup(ctx context.Context) (bool, error) {
	if bootstrap.IsForcedMode() {
		return true, nil
	}

	count, err := u.users.CountAdmins(ctx)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

func (u *SetupUsecase) Validate(ctx context.Context, token, organizationName, email, password string) error {
	count, err := u.users.CountAdmins(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrSetupDone.New("setup already completed")
	}
	if !bootstrap.Validate(token) {
		return ErrInvalidSetupToken.New("invalid or expired setup token")
	}
	if strings.TrimSpace(organizationName) == "" || strings.TrimSpace(email) == "" || strings.TrimSpace(password) == "" {
		return ErrForbidden.New("organization name, email, and password are required")
	}
	return nil
}

func (u *SetupUsecase) Run(ctx context.Context, token, organizationName, email, password string, storageIn StorageInput) error {
	if err := u.Validate(ctx, token, organizationName, email, password); err != nil {
		return err
	}

	orgCount, err := u.orgs.Count(ctx)
	if err != nil {
		return err
	}
	if orgCount == 0 {
		name := strings.TrimSpace(organizationName)
		if name == "" {
			name = "Agentra"
		}

		now := time.Now()
		if err := u.orgs.Create(ctx, domain.Organization{
			ID:        uuid.New(),
			Name:      name,
			Slug:      slugFromName(name),
			IsActive:  true,
			CreatedAt: now,
		}); err != nil {
			return err
		}
	}

	org, err := u.orgs.First(ctx)
	if err != nil {
		return err
	}

	now := time.Now()
	cfg := domain.StorageConfig{
		ID:             uuid.New(),
		OrganizationID: org.ID,
		StorageType:    storageIn.StorageType,
		Endpoint:       null.StringFrom(strings.TrimSpace(storageIn.Endpoint)),
		AccessKey:      null.StringFrom(strings.TrimSpace(storageIn.AccessKey)),
		SecretKey:      null.StringFrom(strings.TrimSpace(storageIn.SecretKey)),
		Bucket:         null.StringFrom(strings.TrimSpace(storageIn.Bucket)),
		BasePath:       null.StringFrom(strings.TrimSpace(storageIn.BasePath)),
		OutputBasePath: null.StringFrom(strings.TrimSpace(storageIn.OutputBasePath)),
		IsActive:       true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := u.storage.Upsert(ctx, cfg); err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	if err := u.users.Create(ctx, domain.User{
		ID:             uuid.New(),
		OrganizationID: org.ID,
		Email:          email,
		PasswordHash:   string(hash),
		Role:           domain.RoleAdmin,
		IsActive:       true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}); err != nil {
		return err
	}

	if !bootstrap.ValidateAndClear(token) {
		return ErrInvalidSetupToken.New("invalid or expired setup token")
	}
	return nil
}

var nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)

func slugFromName(name string) string {
	slug := strings.ToLower(strings.TrimSpace(name))
	slug = nonSlugChars.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "agentra"
	}
	return slug
}

func DefaultSetupStorage() StorageInput {
	return StorageInput{
		StorageType:    domain.StorageMinIO,
		Endpoint:       envOr("MINIO_ENDPOINT", "http://localhost:9000"),
		AccessKey:      envOr("MINIO_ACCESS_KEY", "minioadmin"),
		SecretKey:      envOr("MINIO_SECRET_KEY", "minioadmin"),
		Bucket:         envOr("MINIO_BUCKET", "agentra-media"),
		BasePath:       envOr("MINIO_BASE_PATH", "media"),
		OutputBasePath: envOr("MINIO_OUTPUT_BASE_PATH", "renders"),
	}
}

func (u *SetupUsecase) AvailableStorageOptions() []SetupStorageOption {
	d := DefaultSetupStorage()
	return []SetupStorageOption{
		{
			Type:            d.StorageType,
			Label:           "MinIO (default)",
			Description:     "Built-in object storage. Starts with an empty media library.",
			StartsEmpty:     true,
			DefaultEndpoint: d.Endpoint,
			DefaultAccess:   d.AccessKey,
			DefaultSecret:   d.SecretKey,
			DefaultBucket:   d.Bucket,
			DefaultBasePath: d.BasePath,
			DefaultOutput:   d.OutputBasePath,
		},
	}
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func (u *SetupUsecase) RequestResetToken() string {
	return bootstrap.ForceResetToken()
}
