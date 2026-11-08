package payouts

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"lenapay/apps/api/internal/payers"
	"lenapay/apps/api/internal/platform/apperr"
	"lenapay/apps/api/internal/psp"
)

// Destination formats (BCRA): a CBU or CVU is 22 digits and a CVU starts with "000"; an alias
// is 6 to 20 letters, digits, dots or hyphens.
var (
	accountNumberPattern = regexp.MustCompile(`^[0-9]{22}$`)
	aliasPattern         = regexp.MustCompile(`^[A-Za-z0-9.-]{6,20}$`)
	// holderDocPattern: a CUIT/CUIL (11) or, for a person's Lenapay CVU, a DNI (7-8).
	holderDocPattern = regexp.MustCompile(`^[0-9]{7,11}$`)
)

// ClassifyDestination says what the user typed, or destination.invalid. It is checked before
// asking the PSP, so malformed input never costs a provider call.
func ClassifyDestination(to string) (psp.DestinationType, error) {
	switch {
	case accountNumberPattern.MatchString(to) && strings.HasPrefix(to, "000"):
		return psp.DestinationCVU, nil
	case accountNumberPattern.MatchString(to):
		return psp.DestinationCBU, nil
	case aliasPattern.MatchString(to):
		return psp.DestinationAlias, nil
	}
	return "", apperr.New(apperr.CodeDestinationInvalid, nil)
}

// onUsBankName is the bank an on-us destination shows.
const onUsBankName = "Lenapay"

// target is a resolved destination and, on-us, the Lenapay CVU behind it.
type target struct {
	dest psp.Destination
	onUs *payers.CVU
}

// rail is how a payout to t moves.
func (t target) rail() Rail {
	if t.onUs != nil {
		return RailInternal
	}
	return RailBank
}

// resolve looks the destination up in Lenapay's own CVUs first (on-us: the PSP is never
// called), then at the PSP, and validates the snapshot that will be stored with the payout
// (MO5). A timeout or 5xx is provider.unavailable: nothing is decided on it.
func (s *service) resolve(ctx context.Context, to string) (target, error) {
	to = strings.TrimSpace(to)
	kind, err := ClassifyDestination(to)
	if err != nil {
		return target{}, err
	}
	if s.deps.CVUs != nil && kind != psp.DestinationCBU {
		ours, found, err := s.deps.CVUs.OnUsDestination(ctx, to)
		if err != nil {
			return target{}, err
		}
		if found {
			dest := psp.Destination{
				Type: kind, Value: to, AccountNumber: ours.CVU.Number, HolderName: ours.HolderName,
				HolderCUIT: ours.HolderDoc, BankName: onUsBankName,
			}
			if !validSnapshot(dest) {
				return target{}, apperr.New(apperr.CodeDestinationInvalid, nil)
			}
			return target{dest: dest, onUs: &ours.CVU}, nil
		}
	}
	dest, err := s.deps.PSP.ResolveDestination(ctx, to)
	switch {
	case errors.Is(err, psp.ErrNotFound):
		return target{}, apperr.Wrap(apperr.CodeDestinationNotFound, err, nil)
	case errors.Is(err, psp.ErrRejected):
		return target{}, apperr.Wrap(apperr.CodeDestinationInvalid, err, nil)
	case errors.Is(err, psp.ErrUncertain):
		return target{}, apperr.Wrap(apperr.CodeProviderUnavailable, err, nil)
	case err != nil:
		return target{}, err
	}
	dest.Value = to
	if dest.Type == "" {
		dest.Type = kind
	}
	if !validSnapshot(dest) {
		return target{}, apperr.New(apperr.CodeDestinationInvalid, nil)
	}
	return target{dest: dest}, nil
}

// validSnapshot mirrors the DB CHECKs of the payouts destination columns.
func validSnapshot(d psp.Destination) bool {
	name := len([]rune(d.HolderName))
	return accountNumberPattern.MatchString(d.AccountNumber) && holderDocPattern.MatchString(d.HolderCUIT) &&
		name >= 1 && name <= maxHolderName && len([]rune(d.BankName)) <= maxBankName &&
		(d.Type == psp.DestinationCBU || d.Type == psp.DestinationCVU || d.Type == psp.DestinationAlias)
}
