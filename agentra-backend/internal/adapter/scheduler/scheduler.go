package scheduler

import (
	"context"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/reaganiwadha/agentra/internal/usecase"
	"github.com/sirupsen/logrus"
	"go.uber.org/fx"
)

func Register(lc fx.Lifecycle, scanner *usecase.ScannerUsecase, analyzer *usecase.AnalyzerUsecase, runner *usecase.HighlightRunnerUsecase, log *logrus.Logger) error {
	s, err := gocron.NewScheduler()
	if err != nil {
		return err
	}

	_, err = s.NewJob(
		gocron.DurationJob(5*time.Minute),
		gocron.NewTask(func() {
			if err := scanner.Scan(context.Background()); err != nil {
				log.WithError(err).Error("storage scanner error")
			}
		}),
	)
	if err != nil {
		return err
	}

	_, err = s.NewJob(
		gocron.DurationJob(1*time.Second),
		gocron.NewTask(func() {
			if err := analyzer.Run(context.Background()); err != nil {
				log.WithError(err).Error("analyzer error")
			}
		}),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithStartAt(gocron.WithStartImmediately()),
	)
	if err != nil {
		return err
	}

	_, err = s.NewJob(
		gocron.DurationJob(1*time.Second),
		gocron.NewTask(func() {
			if err := runner.Run(context.Background()); err != nil {
				log.WithError(err).Error("highlight runner error")
			}
		}),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithStartAt(gocron.WithStartImmediately()),
	)
	if err != nil {
		return err
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			s.Start()
			log.Info("scheduler started (scanner: 5m, analyzer: 1s, highlight runner: 1s)")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return s.Shutdown()
		},
	})

	return nil
}
