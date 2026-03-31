package postgres

import (
	"github.com/reaganiwadha/agentra/internal/config"
	"context"
	"database/sql"
	"embed"
	"errors"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/postgres"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"github.com/sirupsen/logrus"
	"go.uber.org/fx"
)

//go:embed migrations/*.sql
var migrations embed.FS

var dialect = goqu.Dialect("postgres")

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}

func getErr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func New(lc fx.Lifecycle, cfg config.Config) (*sqlx.DB, error) {
	db, err := sqlx.Connect("postgres", cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return db.Close()
		},
	})
	return db, nil
}

func Migrate(db *sqlx.DB, log *logrus.Logger) error {
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	if err := goose.Up(db.DB, "migrations"); err != nil {
		return err
	}
	log.Info("migrations applied")
	return nil
}
