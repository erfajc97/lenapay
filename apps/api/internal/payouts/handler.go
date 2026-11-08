package payouts

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"

	"lenapay/apps/api/internal/api"
	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/platform/auth"
	"lenapay/apps/api/internal/platform/money"
	"lenapay/apps/api/internal/platform/page"
	"lenapay/apps/api/internal/platform/upload"
	"lenapay/apps/api/internal/psp"
)

// Handler implements the payouts operations of api.StrictServerInterface (HTTP only: generated
// request → domain input → Service → generated response, A1/AG1). The org comes from the
// guard's principal (T3).
type Handler struct {
	svc Service
}

// NewHandler returns the payouts handler.
func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

// ResolveDestination implements api.StrictServerInterface.
func (h *Handler) ResolveDestination(ctx context.Context, req api.ResolveDestinationRequestObject) (api.ResolveDestinationResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	d, err := h.svc.ResolveDestination(ctx, p, req.Params.To)
	if err != nil {
		return nil, err
	}
	return api.ResolveDestination200JSONResponse(destinationToAPI(d)), nil
}

// QuotePayout implements api.StrictServerInterface.
func (h *Handler) QuotePayout(ctx context.Context, req api.QuotePayoutRequestObject) (api.QuotePayoutResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, apperr.New(apperr.CodeValidation, map[string]any{"field": "body"})
	}
	q, err := h.svc.Quote(ctx, p, QuoteInput{AccountID: req.Body.AccountId, Amount: moneyFromAPI(req.Body.Amount), To: req.Body.To})
	if err != nil {
		return nil, err
	}
	return api.QuotePayout200JSONResponse{
		Amount: moneyToAPI(q.Amount), Fee: moneyToAPI(q.Fee), Total: moneyToAPI(q.Total),
		Destination: destinationToAPI(q.Destination), AvailableAfter: moneyToAPI(q.AvailableAfter), Rail: api.Rail(q.Rail),
	}, nil
}

// CreatePayout implements api.StrictServerInterface.
func (h *Handler) CreatePayout(ctx context.Context, req api.CreatePayoutRequestObject) (api.CreatePayoutResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, apperr.New(apperr.CodeValidation, map[string]any{"field": "body"})
	}
	po, err := h.svc.Create(ctx, p, CreateInput{
		AccountID: req.Body.AccountId, Amount: moneyFromAPI(req.Body.Amount), To: req.Body.To,
		Concept: req.Body.Concept, IdempotencyKey: req.Params.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return api.CreatePayout201JSONResponse(PayoutToAPI(po)), nil
}

// GetPayout implements api.StrictServerInterface.
func (h *Handler) GetPayout(ctx context.Context, req api.GetPayoutRequestObject) (api.GetPayoutResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	po, err := h.svc.Get(ctx, p, req.PayoutId)
	if err != nil {
		return nil, err
	}
	return api.GetPayout200JSONResponse(PayoutToAPI(po)), nil
}

// ApprovePayout implements api.StrictServerInterface.
func (h *Handler) ApprovePayout(ctx context.Context, req api.ApprovePayoutRequestObject) (api.ApprovePayoutResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	po, err := h.svc.Approve(ctx, p, req.PayoutId, req.Params.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	return api.ApprovePayout200JSONResponse(PayoutToAPI(po)), nil
}

// RejectPayout implements api.StrictServerInterface.
func (h *Handler) RejectPayout(ctx context.Context, req api.RejectPayoutRequestObject) (api.RejectPayoutResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, apperr.New(apperr.CodeValidation, map[string]any{"field": "body"})
	}
	po, err := h.svc.Reject(ctx, p, req.PayoutId, req.Body.Reason)
	if err != nil {
		return nil, err
	}
	return api.RejectPayout200JSONResponse(PayoutToAPI(po)), nil
}

// CancelPayout implements api.StrictServerInterface.
func (h *Handler) CancelPayout(ctx context.Context, req api.CancelPayoutRequestObject) (api.CancelPayoutResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	po, err := h.svc.Cancel(ctx, p, req.PayoutId)
	if err != nil {
		return nil, err
	}
	return api.CancelPayout200JSONResponse(PayoutToAPI(po)), nil
}

// GetPayoutProof implements api.StrictServerInterface.
func (h *Handler) GetPayoutProof(ctx context.Context, req api.GetPayoutProofRequestObject) (api.GetPayoutProofResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	pdf, err := h.svc.Proof(ctx, p, req.PayoutId)
	if err != nil {
		return nil, err
	}
	return api.GetPayoutProof200ApplicationpdfResponse{Body: bytes.NewReader(pdf), ContentLength: int64(len(pdf))}, nil
}

// ListApprovals implements api.StrictServerInterface.
func (h *Handler) ListApprovals(ctx context.Context, req api.ListApprovalsRequestObject) (api.ListApprovalsResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	pr, err := page.ParseRequest(req.Params.Cursor, req.Params.Limit)
	if err != nil {
		return nil, err
	}
	pg, err := h.svc.ListApprovals(ctx, p, pr)
	if err != nil {
		return nil, err
	}
	items := make([]api.Payout, 0, len(pg.Items))
	for _, po := range pg.Items {
		items = append(items, PayoutToAPI(po))
	}
	return api.ListApprovals200JSONResponse{Items: items, NextCursor: pg.NextCursor}, nil
}

func moneyFromAPI(m api.Money) money.Amount { return money.New(m.Minor, money.Currency(m.Currency)) }

func moneyToAPI(a money.Amount) api.Money {
	return api.Money{Minor: a.Minor, Currency: api.Currency(a.Currency)}
}

func destinationToAPI(d psp.Destination) api.Destination {
	out := api.Destination{
		Type: api.DestinationType(d.Type), Value: d.Value, AccountNumber: d.AccountNumber,
		HolderName: d.HolderName, HolderCuit: d.HolderCUIT,
	}
	if d.BankName != "" {
		bank := d.BankName
		out.BankName = &bank
	}
	return out
}

// UploadPayoutAttachment implements api.StrictServerInterface.
func (h *Handler) UploadPayoutAttachment(ctx context.Context, req api.UploadPayoutAttachmentRequestObject) (api.UploadPayoutAttachmentResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	in, err := readAttachment(req.Body)
	if err != nil {
		return nil, err
	}
	a, err := h.svc.UploadAttachment(ctx, p, req.PayoutId, in)
	if err != nil {
		return nil, err
	}
	return api.UploadPayoutAttachment201JSONResponse(attachmentToAPI(a)), nil
}

// maxAttachmentParts bounds the multipart parts read (the contract has one: file).
const maxAttachmentParts = 4

// readAttachment reads the single "file" part; any other part is a validation error.
func readAttachment(r *multipart.Reader) (AttachmentUpload, error) {
	if r == nil {
		return AttachmentUpload{}, apperr.New(apperr.CodeValidation, map[string]any{"field": "/file"})
	}
	var in AttachmentUpload
	var file []byte
	for range maxAttachmentParts {
		part, err := r.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return AttachmentUpload{}, apperr.Wrap(apperr.CodeValidation, err, nil)
		}
		name := part.FormName()
		if name != "file" {
			_ = part.Close()
			return AttachmentUpload{}, apperr.New(apperr.CodeValidation, map[string]any{"field": "/" + name})
		}
		b, err := io.ReadAll(io.LimitReader(part, upload.MaxBytes+1))
		_ = part.Close() // a close error on a fully read part carries nothing
		if err != nil {
			return AttachmentUpload{}, apperr.Wrap(apperr.CodeValidation, err, nil)
		}
		file, in.FileName = b, part.FileName()
	}
	if file == nil {
		return AttachmentUpload{}, apperr.New(apperr.CodeValidation, map[string]any{"field": "/file"})
	}
	in.Content = bytes.NewReader(file)
	return in, nil
}

func attachmentToAPI(a Attachment) api.PayoutAttachment {
	return api.PayoutAttachment{
		Id: a.ID, FileName: a.FileName, ContentType: api.PayoutAttachmentContentType(a.ContentType),
		SizeBytes: a.SizeBytes, UploadedBy: a.UploadedBy, CreatedAt: a.CreatedAt,
	}
}

// PayoutToAPI builds the contract's Payout field by field (A5); payerview answers its pay
// operation with it too, so a payout looks the same on every door (G12).
func PayoutToAPI(po Payout) api.Payout {
	total, err := po.Total()
	if err != nil { // amount + fee was summed when the hold was taken; it cannot overflow now
		total = po.Amount
	}
	events := make([]api.PayoutEvent, 0, len(po.Events))
	for _, e := range po.Events {
		ev := api.PayoutEvent{ToStatus: api.PayoutStatus(e.ToStatus), ActorId: e.ActorID, Reason: e.Reason, CreatedAt: e.CreatedAt}
		if e.FromStatus != nil {
			from := api.PayoutStatus(*e.FromStatus)
			ev.FromStatus = &from
		}
		events = append(events, ev)
	}
	attachments := make([]api.PayoutAttachment, 0, len(po.Attachments))
	for _, a := range po.Attachments {
		attachments = append(attachments, attachmentToAPI(a))
	}
	return api.Payout{
		Attachments: attachments,
		Id:          po.ID, AccountId: po.AccountID, Amount: moneyToAPI(po.Amount), Fee: moneyToAPI(po.Fee), Total: moneyToAPI(total),
		Destination: destinationToAPI(po.Destination), Concept: po.Concept, Status: api.PayoutStatus(po.Status), Rail: api.Rail(po.Rail),
		CreatedBy: po.CreatedBy, ApprovedBy: po.ApprovedBy, RejectReason: po.RejectReason, FailureCode: po.FailureCode,
		Events: events, CreatedAt: po.CreatedAt, UpdatedAt: po.UpdatedAt, SettledAt: po.SettledAt,
	}
}
