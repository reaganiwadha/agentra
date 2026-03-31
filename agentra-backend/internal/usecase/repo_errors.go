package usecase

import (
	"errors"

	"github.com/reaganiwadha/agentra/internal/repository/postgres"
)

func normalizeRepoErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, postgres.ErrNotFound):
		return ErrNotFound.New("not found")
	case errors.Is(err, postgres.ErrConflict):
		return ErrConflict.New("conflict")
	default:
		return err
	}
}
