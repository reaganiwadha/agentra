package rageditor

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/reaganiwadha/agentra/internal/domain"
	"github.com/sirupsen/logrus"
)

func newTrace(jobID uuid.UUID, phase string, message string, payload map[string]any) domain.HighlightJobTrace {
	body, _ := json.Marshal(payload)
	logrus.WithFields(logrus.Fields{
		"job_id": jobID,
		"phase":  phase,
	}).Debug(message)
	return domain.HighlightJobTrace{
		ID:        uuid.New(),
		JobID:     jobID,
		Phase:     phase,
		Message:   message,
		Payload:   body,
		CreatedAt: time.Now(),
	}
}
