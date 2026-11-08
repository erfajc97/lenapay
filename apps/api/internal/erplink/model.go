package erplink

import (
	"time"

	"github.com/google/uuid"

	"lenapay/apps/api/internal/platform/auth"
	"lenapay/apps/api/internal/platform/config"
)

// Caps of the ERP credentials (contract CreateErpLinkRequest).
const (
	maxUsername = 120
	maxPassword = 128
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
	// Summary is what the import copied and who it invited; nil until the import has run.
	Summary *ImportSummary
}

// ImportSummary is the result of a link's import.
type ImportSummary struct {
	Projects                   int
	Buyers                     int
	SkippedBuyers              int
	UsersInvited               int
	UsersAlreadyMembers        int
	UsersWithoutPaymentsAccess int
	InvitedUsers               []InvitedUser
}

// InvitedUser is an ERP user the import invited, with the role after the linker cap.
type InvitedUser struct {
	Email string    `json:"email"`
	Role  auth.Role `json:"role"`
}
