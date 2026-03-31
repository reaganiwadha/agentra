package usecase

import (
	"github.com/reaganiwadha/agentra/internal/domain"
	"github.com/reaganiwadha/agentra/internal/repository/postgres"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UserUsecase struct {
	users *postgres.UserRepo
}

func NewUserUsecase(users *postgres.UserRepo) *UserUsecase {
	return &UserUsecase{users: users}
}

func (u *UserUsecase) Create(ctx context.Context, orgID uuid.UUID, email, password string, role domain.UserRole) (domain.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, err
	}

	now := time.Now()
	user := domain.User{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Email:          email,
		PasswordHash:   string(hash),
		Role:           role,
		IsActive:       true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := u.users.Create(ctx, user); err != nil {
		if errors.Is(err, postgres.ErrConflict) {
			return domain.User{}, ErrConflict.New("email already in use")
		}
		return domain.User{}, err
	}
	return user, nil
}
