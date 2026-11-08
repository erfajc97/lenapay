package kyc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"lenapay/apps/api/internal/api"
	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/platform/auth"
)

// Handler implements the kyc operations of api.StrictServerInterface (HTTP only: generated
// request → domain input → Service → generated response, A1/AG1). The org comes from the
// guard's principal (T3).
type Handler struct {
	svc Service
}

// NewHandler returns the kyc handler.
func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

// GetKyc implements api.StrictServerInterface.
func (h *Handler) GetKyc(ctx context.Context, _ api.GetKycRequestObject) (api.GetKycResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	c, err := h.svc.Get(ctx, p)
	if err != nil {
		return nil, err
	}
	return api.GetKyc200JSONResponse(caseToAPI(c)), nil
}

// UpdateKyc implements api.StrictServerInterface.
func (h *Handler) UpdateKyc(ctx context.Context, req api.UpdateKycRequestObject) (api.UpdateKycResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	var in UpdateInput
	if req.Body.Company != nil {
		in.Company = companyFromAPI(*req.Body.Company)
	}
	if req.Body.Person != nil {
		in.Person = personFromAPI(*req.Body.Person)
	}
	c, err := h.svc.Update(ctx, p, in)
	if err != nil {
		return nil, err
	}
	return api.UpdateKyc200JSONResponse(caseToAPI(c)), nil
}

// Multipart field caps: text fields are tiny; the file is capped by the service (and the
// router's upload body limit).
const (
	maxTextField = 64
	maxParts     = 8
)

// UploadKycDocument implements api.StrictServerInterface.
func (h *Handler) UploadKycDocument(ctx context.Context, req api.UploadKycDocumentRequestObject) (api.UploadKycDocumentResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	in, err := readUpload(req.Body)
	if err != nil {
		return nil, err
	}
	d, err := h.svc.UploadDocument(ctx, p, in)
	if err != nil {
		return nil, err
	}
	return api.UploadKycDocument201JSONResponse(documentToAPI(d)), nil
}

// readUpload reads the multipart parts in any order: kind, representativeDni and file.
func readUpload(r *multipart.Reader) (DocumentUpload, error) {
	var in DocumentUpload
	var file []byte
	for range maxParts {
		part, err := r.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return DocumentUpload{}, apperr.Wrap(apperr.CodeValidation, err, nil)
		}
		if err := readPart(part, &in, &file); err != nil {
			return DocumentUpload{}, err
		}
	}
	if in.Kind == "" || file == nil {
		return DocumentUpload{}, apperr.New(apperr.CodeValidation, map[string]any{"field": "/file"})
	}
	in.Content = bytes.NewReader(file)
	return in, nil
}

func readPart(part *multipart.Part, in *DocumentUpload, file *[]byte) error {
	defer func() { _ = part.Close() }() // a close error on a fully read part carries nothing
	name := part.FormName()
	if name == "file" {
		b, err := io.ReadAll(io.LimitReader(part, MaxDocumentBytes+1))
		if err != nil {
			return err
		}
		*file, in.FileName, in.ContentType = b, part.FileName(), part.Header.Get("Content-Type")
		return nil
	}
	b, err := io.ReadAll(io.LimitReader(part, maxTextField+1))
	if err != nil {
		return err
	}
	if len(b) > maxTextField {
		return apperr.New(apperr.CodeValidation, map[string]any{"field": "/" + name})
	}
	switch name {
	case "kind":
		in.Kind = string(b)
	case "representativeDni":
		dni := string(b)
		in.RepresentativeDNI = &dni
	default:
		return apperr.New(apperr.CodeValidation, map[string]any{"field": "/" + name})
	}
	return nil
}

// SubmitKyc implements api.StrictServerInterface.
func (h *Handler) SubmitKyc(ctx context.Context, _ api.SubmitKycRequestObject) (api.SubmitKycResponseObject, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	c, err := h.svc.Submit(ctx, p)
	if err != nil {
		return nil, err
	}
	return api.SubmitKyc200JSONResponse(caseToAPI(c)), nil
}

// DecideKyc implements api.StrictServerInterface (internal API key; ops decision).
func (h *Handler) DecideKyc(ctx context.Context, req api.DecideKycRequestObject) (api.DecideKycResponseObject, error) {
	c, err := h.svc.Decide(ctx, req.CaseId, Decision{Approved: req.Body.Decision == api.KycDecisionRequestDecisionApproved, Reason: req.Body.Reason})
	if err != nil {
		return nil, err
	}
	return api.DecideKyc200JSONResponse(caseToAPI(c)), nil
}

func caseToAPI(c Case) api.KycCase {
	out := api.KycCase{
		Id: c.ID, OrgId: c.OrgID, Kind: api.KycCaseKind(c.Kind), Status: api.KycStatus(c.Status),
		Documents: make([]api.KycDocument, 0, len(c.Documents)), DecisionReason: c.DecisionReason,
		SubmittedAt: c.SubmittedAt, DecidedAt: c.DecidedAt, UpdatedAt: c.UpdatedAt,
	}
	if c.Company != nil {
		company := companyToAPI(*c.Company)
		out.Company = &company
	}
	if c.Person != nil {
		person := api.KycPersonData{
			FullName: c.Person.FullName, Dni: c.Person.DNI, BirthDate: openapi_types.Date{Time: c.Person.BirthDate},
			Address: addressToAPI(c.Person.Address),
		}
		out.Person = &person
	}
	for _, d := range c.Documents {
		out.Documents = append(out.Documents, documentToAPI(d))
	}
	return out
}

func documentToAPI(d Document) api.KycDocument {
	return api.KycDocument{
		Id: d.ID, Kind: api.KycDocumentKind(d.Kind), RepresentativeDni: d.RepresentativeDNI, FileName: d.FileName,
		ContentType: d.ContentType, SizeBytes: d.SizeBytes, CreatedAt: d.CreatedAt,
	}
}

func companyToAPI(c CompanyData) api.KycCompanyData {
	reps := make([]api.KycRepresentative, 0, len(c.Representatives))
	for _, r := range c.Representatives {
		reps = append(reps, api.KycRepresentative{FullName: r.FullName, Dni: r.DNI, Position: r.Position})
	}
	return api.KycCompanyData{Cuit: c.CUIT, LegalName: c.LegalName, LegalAddress: addressToAPI(c.LegalAddress), Representatives: reps}
}

func companyFromAPI(c api.KycCompanyData) *CompanyData {
	reps := make([]Representative, 0, len(c.Representatives))
	for _, r := range c.Representatives {
		reps = append(reps, Representative{FullName: r.FullName, DNI: r.Dni, Position: r.Position})
	}
	return &CompanyData{CUIT: c.Cuit, LegalName: c.LegalName, LegalAddress: addressFromAPI(c.LegalAddress), Representatives: reps}
}

func personFromAPI(p api.KycPersonData) *PersonData {
	return &PersonData{
		FullName: p.FullName, DNI: p.Dni, BirthDate: time.Date(p.BirthDate.Year(), p.BirthDate.Month(), p.BirthDate.Day(), 0, 0, 0, 0, time.UTC),
		Address: addressFromAPI(p.Address),
	}
}

// defaultCountry is the contract's Address.country default.
const defaultCountry = "AR"

func addressFromAPI(a api.Address) Address {
	out := Address{Street: a.Street, Number: a.Number, City: a.City, Province: a.Province, PostalCode: a.PostalCode, Country: defaultCountry}
	if a.Floor != nil {
		out.Floor = *a.Floor
	}
	if a.Country != nil {
		out.Country = *a.Country
	}
	return out
}

func addressToAPI(a Address) api.Address {
	out := api.Address{Street: a.Street, Number: a.Number, City: a.City, Province: a.Province, PostalCode: a.PostalCode}
	if a.Floor != "" {
		out.Floor = &a.Floor
	}
	if a.Country != "" {
		out.Country = &a.Country
	}
	return out
}
