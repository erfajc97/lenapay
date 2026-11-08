// Package apperr is the ONE error catalog of the API (A7, AG4): every error a client can see
// is an *Error with a Code from this file. Clients branch on Code, never on Message.
//
// Lanes APPEND their codes (G14: surgical edits, never reorder): the constant, its row in
// catalog(), its es-AR copy in the mobile i18n files, then a test asserting the code (FT3).
// A missing resource and someone else's resource are both 404 (never 403).
package apperr

import (
	"errors"
	"net/http"
)

// Code is a stable, machine-readable error code ("area.reason").
type Code string

// Generic request / server errors.
const (
	CodeValidation          Code = "validation.failed"
	CodeBodyTooLarge        Code = "request.too_large"
	CodeTimeout             Code = "request.timeout"
	CodeRateLimited         Code = "request.rate_limited"
	CodeNotFound            Code = "resource.not_found"
	CodeConflict            Code = "resource.conflict"
	CodeOperationInProgress Code = "operation.in_progress"
	CodeNotImplemented      Code = "internal.not_implemented"
	CodeInternal            Code = "internal.error"
)

// Auth and access (platform/auth, identity).
const (
	CodeUnauthenticated    Code = "auth.unauthenticated"
	CodeForbidden          Code = "auth.forbidden"
	CodeStepUpRequired     Code = "auth.step_up_required"
	CodeInvalidCredentials Code = "auth.invalid_credentials" //nolint:gosec // an error code, not a credential
	CodeMFARequired        Code = "auth.mfa_required"
	CodeEmailNotVerified   Code = "auth.email_not_verified"
	CodeEmailTaken         Code = "auth.email_taken"
	CodeCodeInvalid        Code = "auth.code_invalid"
	CodeRefreshInvalid     Code = "auth.refresh_invalid"
	CodeChallengeInvalid   Code = "auth.challenge_invalid"
	CodeDeviceNotFound     Code = "device.not_found"
)

// Idempotency (MO2).
const (
	CodeIdempotencyReused Code = "idempotency.key_reused"
)

// Organizations and team.
const (
	CodeOrgNotFound          Code = "org.not_found"
	CodeMemberAlreadyExists  Code = "member.already_exists"
	CodeMemberNotFound       Code = "member.not_found"
	CodeUserNotFound         Code = "user.not_found"
	CodeInviteNotFound       Code = "invite.not_found"
	CodeInviteExpired        Code = "invite.expired"
	CodeRoleEscalation       Code = "role.escalation_forbidden"
	CodeLastAdmin            Code = "org.last_admin"
	CodePersonalModeDisabled Code = "org.personal_mode_disabled"
)

// KYC.
const (
	CodeKYCNotApproved     Code = "kyc.not_approved"
	CodeKYCCaseNotFound    Code = "kyc.case_not_found"
	CodeKYCInvalidState    Code = "kyc.invalid_state"
	CodeKYCIncomplete      Code = "kyc.incomplete"
	CodeKYCDocumentInvalid Code = "kyc.document_invalid"
)

// Money, accounts and ledger.
const (
	CodeInvalidAmount        Code = "money.invalid_amount"
	CodeCurrencyMismatch     Code = "money.currency_mismatch"
	CodeAccountNotFound      Code = "account.not_found"
	CodeAccountArchived      Code = "account.archived"
	CodeAccountNotEmpty      Code = "account.not_empty"
	CodeInsufficientFunds    Code = "ledger.insufficient_funds"
	CodeTransferSameAccount  Code = "transfer.same_account"
	CodeHoldNotActive        Code = "ledger.hold_not_active"
	CodeAccountNotArchivable Code = "account.not_archivable"
	CodeMovementNotFound     Code = "movement.not_found"
)

// Payers, CVUs, payins and the PSP.
const (
	CodePayerNotFound       Code = "payer.not_found"
	CodeCVUNotFound         Code = "cvu.not_found"
	CodeCVUAlreadyExists    Code = "cvu.already_exists"
	CodeCVUAliasTaken       Code = "cvu.alias_taken"
	CodeCVUDisabled         Code = "cvu.disabled"
	CodePayinNotFound       Code = "payin.not_found"
	CodeWebhookSignature    Code = "webhook.bad_signature"
	CodeProviderUnavailable Code = "provider.unavailable"
	CodeDestinationNotFound Code = "destination.not_found"
	CodeDestinationInvalid  Code = "destination.invalid"
)

// Payouts.
const (
	CodePayoutNotFound          Code = "payout.not_found"
	CodePayoutInvalidState      Code = "payout.invalid_state"
	CodePayoutSelfApproval      Code = "payout.self_approval_forbidden"
	CodePayoutProofUnavailable  Code = "payout.proof_unavailable"
	CodePayoutAttachmentInvalid Code = "payout.attachment_invalid"
)

// Reconciliation and ERP link.
const (
	CodeReconciliationRunNotFound Code = "reconciliation.run_not_found"
	CodeERPLinkNotFound           Code = "erp_link.not_found"
	CodeERPNoCandidate            Code = "erp_link.no_candidate"
	CodeERPCredentialsInvalid     Code = "erp_link.credentials_invalid" //nolint:gosec // an error code, not a credential
	CodeERPAlreadyLinked          Code = "erp_link.already_linked"
)

// Payer view ("Mis pagos").
const (
	CodePayerLinkNotFound   Code = "payer_link.not_found"
	CodeInstallmentNotFound Code = "installment.not_found"
)

type entry struct {
	status  int
	message string
}

// catalog is a function, not a package variable, so nothing can mutate it at runtime.
func catalog() map[Code]entry {
	return map[Code]entry{
		CodeValidation:          {http.StatusUnprocessableEntity, "The request does not match the contract."},
		CodeBodyTooLarge:        {http.StatusRequestEntityTooLarge, "The request body is too large."},
		CodeTimeout:             {http.StatusGatewayTimeout, "The request took too long."},
		CodeRateLimited:         {http.StatusTooManyRequests, "Too many requests."},
		CodeNotFound:            {http.StatusNotFound, "Not found."},
		CodeConflict:            {http.StatusConflict, "The resource conflicts with an existing one."},
		CodeOperationInProgress: {http.StatusConflict, "A concurrent operation is in progress; retry with the same idempotency key."},
		CodeNotImplemented:      {http.StatusNotImplemented, "Not implemented yet."},
		CodeInternal:            {http.StatusInternalServerError, "Unexpected error."},

		CodeUnauthenticated:    {http.StatusUnauthorized, "Authentication required."},
		CodeForbidden:          {http.StatusForbidden, "Not allowed."},
		CodeStepUpRequired:     {http.StatusForbidden, "A fresh step-up token is required."},
		CodeInvalidCredentials: {http.StatusUnauthorized, "Invalid credentials."},
		CodeMFARequired:        {http.StatusUnauthorized, "A TOTP code is required."},
		CodeEmailNotVerified:   {http.StatusForbidden, "The email is not verified."},
		CodeEmailTaken:         {http.StatusConflict, "The email is already registered."},
		CodeCodeInvalid:        {http.StatusUnprocessableEntity, "The code is invalid or expired."},
		CodeRefreshInvalid:     {http.StatusUnauthorized, "The refresh token is invalid."},
		CodeChallengeInvalid:   {http.StatusUnprocessableEntity, "The step-up challenge is invalid or expired."},
		CodeDeviceNotFound:     {http.StatusNotFound, "Device not found."},

		CodeIdempotencyReused: {http.StatusConflict, "The idempotency key was used with a different request."},

		CodeOrgNotFound:          {http.StatusNotFound, "Organization not found."},
		CodeMemberAlreadyExists:  {http.StatusConflict, "The user is already a member."},
		CodeMemberNotFound:       {http.StatusNotFound, "Member not found."},
		CodeUserNotFound:         {http.StatusNotFound, "User not found."},
		CodeInviteNotFound:       {http.StatusNotFound, "Invite not found."},
		CodeInviteExpired:        {http.StatusGone, "The invite expired."},
		CodeRoleEscalation:       {http.StatusForbidden, "Cannot grant a role above your own."},
		CodeLastAdmin:            {http.StatusConflict, "The organization needs at least one admin."},
		CodePersonalModeDisabled: {http.StatusForbidden, "Personal mode is disabled."},

		CodeKYCNotApproved:     {http.StatusForbidden, "The organization is not KYC approved."},
		CodeKYCCaseNotFound:    {http.StatusNotFound, "KYC case not found."},
		CodeKYCInvalidState:    {http.StatusConflict, "The KYC case cannot change in its current state."},
		CodeKYCIncomplete:      {http.StatusUnprocessableEntity, "The KYC case is incomplete."},
		CodeKYCDocumentInvalid: {http.StatusUnprocessableEntity, "The document type or content is not accepted."},

		CodeInvalidAmount:        {http.StatusUnprocessableEntity, "The amount is invalid."},
		CodeCurrencyMismatch:     {http.StatusUnprocessableEntity, "The currencies do not match."},
		CodeAccountNotFound:      {http.StatusNotFound, "Account not found."},
		CodeAccountArchived:      {http.StatusConflict, "The account is archived."},
		CodeAccountNotEmpty:      {http.StatusConflict, "The account still has a balance."},
		CodeInsufficientFunds:    {http.StatusUnprocessableEntity, "Insufficient available balance."},
		CodeTransferSameAccount:  {http.StatusUnprocessableEntity, "Origin and destination accounts are the same."},
		CodeHoldNotActive:        {http.StatusConflict, "The hold is no longer active."},
		CodeAccountNotArchivable: {http.StatusConflict, "Only project accounts can be archived."},
		CodeMovementNotFound:     {http.StatusNotFound, "Movement not found."},

		CodePayerNotFound:       {http.StatusNotFound, "Payer not found."},
		CodeCVUNotFound:         {http.StatusNotFound, "CVU not found."},
		CodeCVUAlreadyExists:    {http.StatusConflict, "The payer already has a CVU in this currency."},
		CodeCVUAliasTaken:       {http.StatusConflict, "The alias is taken."},
		CodeCVUDisabled:         {http.StatusConflict, "The CVU is disabled."},
		CodePayinNotFound:       {http.StatusNotFound, "Payin not found."},
		CodeWebhookSignature:    {http.StatusUnauthorized, "Invalid webhook signature."},
		CodeProviderUnavailable: {http.StatusServiceUnavailable, "The payment provider is unavailable."},
		CodeDestinationNotFound: {http.StatusNotFound, "Destination not found at the provider."},
		CodeDestinationInvalid:  {http.StatusUnprocessableEntity, "The destination is not a valid CBU, CVU or alias."},

		CodePayoutNotFound:          {http.StatusNotFound, "Payout not found."},
		CodePayoutInvalidState:      {http.StatusConflict, "The payout cannot change in its current state."},
		CodePayoutSelfApproval:      {http.StatusForbidden, "The creator of a payout cannot approve it."},
		CodePayoutProofUnavailable:  {http.StatusConflict, "The proof exists only for settled payouts."},
		CodePayoutAttachmentInvalid: {http.StatusUnprocessableEntity, "The attachment type is not accepted or the payout has too many."},

		CodeReconciliationRunNotFound: {http.StatusNotFound, "Reconciliation run not found."},
		CodeERPLinkNotFound:           {http.StatusNotFound, "ERP link not found."},
		CodeERPNoCandidate:            {http.StatusNotFound, "The organization is not in the ERP identity index."},
		CodeERPCredentialsInvalid:     {http.StatusUnprocessableEntity, "The ERP credentials were rejected."},
		CodeERPAlreadyLinked:          {http.StatusConflict, "The organization is already linked."},

		CodePayerLinkNotFound:   {http.StatusNotFound, "Company not found in your payments."},
		CodeInstallmentNotFound: {http.StatusNotFound, "Installment not found in your payment plan."},
	}
}

// All lists every code of the catalog (the mobile locale test compares against it).
func All() []Code {
	codes := make([]Code, 0, len(catalog()))
	for c := range catalog() {
		codes = append(codes, c)
	}
	return codes
}

// Status is the HTTP status of a code; an unknown code is a 500, never a made-up status.
func (c Code) Status() int {
	if e, ok := catalog()[c]; ok {
		return e.status
	}
	return http.StatusInternalServerError
}

// Message is the developer-facing English text of a code (clients translate by code).
func (c Code) Message() string {
	if e, ok := catalog()[c]; ok {
		return e.message
	}
	return catalog()[CodeInternal].message
}

// Error is the only error type that crosses a layer boundary towards HTTP.
type Error struct {
	Code    Code
	Details map[string]any // ids and enums only: never PII, never free text
	cause   error
}

// New returns an error with a catalog code.
func New(code Code, details map[string]any) *Error { return &Error{Code: code, Details: details} }

// Wrap keeps the cause for logs (errors.Is / errors.As still see it); clients see only Code.
func Wrap(code Code, cause error, details map[string]any) *Error {
	return &Error{Code: code, Details: details, cause: cause}
}

// NotImplemented is what every E0 stub returns until its lane fills it in.
func NotImplemented() *Error { return New(CodeNotImplemented, nil) }

func (e *Error) Error() string {
	if e.cause != nil {
		return string(e.Code) + ": " + e.cause.Error()
	}
	return string(e.Code)
}

func (e *Error) Unwrap() error { return e.cause }

// IsCode reports whether err (or anything it wraps) is an *Error with that code.
func IsCode(err error, code Code) bool {
	return CodeOf(err) == code
}

// CodeOf returns the code of err, or "" when err is not an *Error.
func CodeOf(err error) Code {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}
