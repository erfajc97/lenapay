package erplink

import (
	"time"

	"github.com/google/uuid"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/config"
)

// ImportStatus of a link's snapshot import.
type ImportStatus string

// Import statuses.
const (
	ImportPending   ImportStatus = "pending"
	ImportRunning   ImportStatus = "running"
	ImportCompleted ImportStatus = "completed"
	ImportFailed    ImportStatus = "failed"
)

// Candidate says whether the org's CUIT exists in the ERP identity index.
type Candidate struct {
	Found       bool
	ERPOrgID    *string
	DisplayName *string
}

// Credentials are the ERP user's credentials for the extra confirmation (never stored).
type Credentials struct {
	Username string
	Password config.Secret
}

// Link is an org ↔ ERP org link.
type Link struct {
	ID           uuid.UUID
	ERPOrgID     string
	DisplayName  string
	LinkedBy     uuid.UUID
	LinkedAt     time.Time
	ImportStatus ImportStatus
	ImportedAt   *time.Time
}
