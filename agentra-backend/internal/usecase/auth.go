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

type AuthUsecase struct {
	users    *postgres.UserRepo
	sessions *postgres.SessionRepo
}

func NewAuthUsecase(users *postgres.UserRepo, sessions *postgres.SessionRepo) *AuthUsecase {
	return &AuthUsecase{users: users, sessions: sessions}
}

func (u *AuthUsecase) Login(ctx context.Context, email, password string) (domain.Session, error) {
	user, err := u.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			return domain.Session{}, ErrInvalidCredentials.New("invalid email or password")
		}
		return domain.Session{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return domain.Session{}, ErrInvalidCredentials.New("invalid email or password")
	}

	s := domain.Session{
		ID:        uuid.New(),
		UserID:    user.ID,
		CreatedAt: time.Now(),
	}
	if err := u.sessions.Create(ctx, s); err != nil {
		return domain.Session{}, err
	}
	return s, nil
}

func (u *AuthUsecase) Logout(ctx context.Context, sessionID uuid.UUID) error {
	return u.sessions.Delete(ctx, sessionID)
}
