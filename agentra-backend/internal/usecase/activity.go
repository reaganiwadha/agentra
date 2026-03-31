package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/reaganiwadha/agentra/internal/domain"
	"github.com/reaganiwadha/agentra/internal/repository/postgres"
)

type ActivityEmitInput struct {
	OrganizationID uuid.UUID
	Source         domain.ActivitySource
	SubjectType    string
	SubjectID      uuid.UUID
	EventType      domain.ActivityEventType
	Level          string
	Message        string
	Payload        map[string]any
	IsActive       *bool
}

type ActivitySnapshot struct {
	Active bool                 `json:"active"`
	Latest *domain.ActivityLog  `json:"latest,omitempty"`
	Logs   []domain.ActivityLog `json:"logs"`
}

type ActivityUsecase struct {
	repo *postgres.ActivityLogRepo

	mu   sync.RWMutex
	subs map[uuid.UUID]map[chan domain.ActivityLog]struct{}
}

func NewActivityUsecase(repo *postgres.ActivityLogRepo) *ActivityUsecase {
	return &ActivityUsecase{
		repo: repo,
		subs: make(map[uuid.UUID]map[chan domain.ActivityLog]struct{}),
	}
}

func (u *ActivityUsecase) Emit(ctx context.Context, in ActivityEmitInput) error {
	now := time.Now().UTC()
	active := false
	if in.IsActive != nil {
		active = *in.IsActive
	} else {
		active = defaultActiveForEventType(in.EventType)
	}

	var subjectID uuid.NullUUID
	if in.SubjectID != uuid.Nil {
		subjectID = uuid.NullUUID{UUID: in.SubjectID, Valid: true}
	}
	payloadRaw := json.RawMessage(`{}`)
	if len(in.Payload) > 0 {
		if b, err := json.Marshal(in.Payload); err == nil {
			payloadRaw = b
		}
	}

	log := domain.ActivityLog{
		ID:             uuid.New(),
		OrganizationID: in.OrganizationID,
		Source:         in.Source,
		SubjectType:    in.SubjectType,
		SubjectID:      subjectID,
		EventType:      in.EventType,
		Level:          nonEmptyTrim(in.Level, "info"),
		Message:        in.Message,
		Payload:        payloadRaw,
		IsActive:       active,
		CreatedAt:      now,
	}
	if err := u.repo.Create(ctx, log); err != nil {
		return err
	}
	u.publish(log)
	return nil
}

func (u *ActivityUsecase) Snapshot(ctx context.Context, orgID uuid.UUID, limit int) (ActivitySnapshot, error) {
	logs, err := u.repo.ListRecentByOrg(ctx, orgID, limit)
	if err != nil {
		return ActivitySnapshot{}, err
	}

	active, err := u.repo.HasActiveSince(ctx, orgID, time.Now().UTC().Add(-45*time.Second))
	if err != nil {
		return ActivitySnapshot{}, err
	}

	var latest *domain.ActivityLog
	if len(logs) > 0 {
		latest = &logs[0]
	}
	return ActivitySnapshot{
		Active: active,
		Latest: latest,
		Logs:   logs,
	}, nil
}

func (u *ActivityUsecase) Status(ctx context.Context, orgID uuid.UUID) (bool, *domain.ActivityLog, error) {
	log, err := u.repo.LatestByOrg(ctx, orgID)
	if err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			active, activeErr := u.repo.HasActiveSince(ctx, orgID, time.Now().UTC().Add(-45*time.Second))
			if activeErr != nil {
				return false, nil, activeErr
			}
			return active, nil, nil
		}
		return false, nil, err
	}
	active, err := u.repo.HasActiveSince(ctx, orgID, time.Now().UTC().Add(-45*time.Second))
	if err != nil {
		return false, nil, err
	}
	return active, &log, nil
}

func (u *ActivityUsecase) ListRecent(ctx context.Context, orgID uuid.UUID, limit int) ([]domain.ActivityLog, error) {
	return u.repo.ListRecentByOrg(ctx, orgID, limit)
}

func (u *ActivityUsecase) Subscribe(orgID uuid.UUID) (<-chan domain.ActivityLog, func()) {
	ch := make(chan domain.ActivityLog, 64)

	u.mu.Lock()
	if u.subs[orgID] == nil {
		u.subs[orgID] = make(map[chan domain.ActivityLog]struct{})
	}
	u.subs[orgID][ch] = struct{}{}
	u.mu.Unlock()

	unsubscribe := func() {
		u.mu.Lock()
		defer u.mu.Unlock()
		if subs, ok := u.subs[orgID]; ok {
			delete(subs, ch)
			if len(subs) == 0 {
				delete(u.subs, orgID)
			}
		}
		close(ch)
	}
	return ch, unsubscribe
}

func (u *ActivityUsecase) publish(log domain.ActivityLog) {
	u.mu.RLock()
	subs := u.subs[log.OrganizationID]
	if len(subs) == 0 {
		u.mu.RUnlock()
		return
	}
	list := make([]chan domain.ActivityLog, 0, len(subs))
	for ch := range subs {
		list = append(list, ch)
	}
	u.mu.RUnlock()

	for _, ch := range list {
		select {
		case ch <- log:
		default:
		}
	}
}

func defaultActiveForEventType(t domain.ActivityEventType) bool {
	switch t {
	case domain.ActivityEventStarted, domain.ActivityEventProgress:
		return true
	default:
		return false
	}
}

func nonEmptyTrim(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return strings.TrimSpace(v)
}
