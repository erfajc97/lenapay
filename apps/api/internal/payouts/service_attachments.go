package payouts

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/erfajc97/lenapay/apps/api/internal/payouts/store"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/db"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/upload"
)

// MaxAttachments is how many documents one payout can carry.
const MaxAttachments = 10

func (s *service) UploadAttachment(ctx context.Context, p auth.Principal, payoutID uuid.UUID, in AttachmentUpload) (Attachment, error) {
	if err := requireMoney(p, auth.RoleOperator, false); err != nil {
		return Attachment{}, err
	}
	content, err := io.ReadAll(io.LimitReader(in.Content, upload.MaxBytes+1))
	if err != nil {
		return Attachment{}, fmt.Errorf("payouts: read attachment: %w", err)
	}
	if len(content) > upload.MaxBytes {
		return Attachment{}, apperr.New(apperr.CodeBodyTooLarge, nil)
	}
	contentType := upload.Sniff(content) // the magic bytes decide, never the declared type (A13)
	if contentType == "" {
		return Attachment{}, apperr.New(apperr.CodePayoutAttachmentInvalid, map[string]any{"reason": "type"})
	}
	if _, err := load(ctx, store.New(s.deps.Pool), payoutID, p.OrgID); err != nil {
		return Attachment{}, err // payout.not_found before anything is stored
	}
	a := Attachment{
		ID: ids.New(), FileName: upload.CleanFileName(in.FileName), ContentType: contentType,
		SizeBytes: int64(len(content)), UploadedBy: p.UserID, CreatedAt: s.deps.Clock.Now(),
	}
	key := "org/" + p.OrgID.String() + "/payouts/" + payoutID.String() + "/" + a.ID.String() // T9: tenant segment
	// Stored before the transaction (no I/O to the store inside it, AG11): an object whose row
	// never commits is unreachable, because only rows point at objects.
	if err := s.deps.Documents.Put(ctx, key, contentType, bytes.NewReader(content)); err != nil {
		return Attachment{}, fmt.Errorf("payouts: store attachment: %w", err)
	}
	err = db.WithTx(ctx, s.deps.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := store.New(tx)
		if _, err := lockInOrg(ctx, q, payoutID, p.OrgID); err != nil { // serializes the count below
			return err
		}
		n, err := q.CountPayoutAttachments(ctx, store.CountPayoutAttachmentsParams{PayoutID: payoutID, OrgID: p.OrgID})
		if err != nil {
			return fmt.Errorf("payouts: count attachments: %w", err)
		}
		if n >= MaxAttachments {
			return apperr.New(apperr.CodePayoutAttachmentInvalid, map[string]any{"reason": "limit", "max": MaxAttachments})
		}
		if err := q.InsertPayoutAttachment(ctx, store.InsertPayoutAttachmentParams{
			ID: a.ID, OrgID: p.OrgID, PayoutID: payoutID, FileName: a.FileName, ContentType: a.ContentType,
			SizeBytes: a.SizeBytes, Sha256: upload.SHA256(content), StorageKey: key, UploadedBy: p.UserID, CreatedAt: a.CreatedAt,
		}); err != nil {
			return fmt.Errorf("payouts: insert attachment: %w", err)
		}
		return s.audit(ctx, tx, p.OrgID, &p.UserID, "payout.attachment_upload", payoutID, "")
	})
	if err != nil {
		return Attachment{}, err
	}
	return a, nil
}

func attachmentsFromRows(rows []store.ListPayoutAttachmentsRow) []Attachment {
	out := make([]Attachment, 0, len(rows))
	for _, r := range rows {
		out = append(out, Attachment{
			ID: r.ID, FileName: r.FileName, ContentType: r.ContentType, SizeBytes: r.SizeBytes,
			UploadedBy: r.UploadedBy, CreatedAt: r.CreatedAt,
		})
	}
	return out
}
