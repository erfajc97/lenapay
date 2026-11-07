package payouts

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/apperr"
	"github.com/erfajc97/lenapay/apps/api/internal/psp"
)

// Destination formats (BCRA): a CBU or CVU is 22 digits and a CVU starts with "000"; an alias
// is 6 to 20 letters, digits, dots or hyphens.
var (
	accountNumberPattern = regexp.MustCompile(`^[0-9]{22}$`)
	aliasPattern         = regexp.MustCompile(`^[A-Za-z0-9.-]{6,20}$`)
	cuitPattern          = regexp.MustCompile(`^[0-9]{11}$`)
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

// resolve looks the destination up at the PSP and validates the snapshot that will be stored
// with the payout (MO5). A timeout or 5xx is provider.unavailable: nothing is decided on it.
func (s *service) resolve(ctx context.Context, to string) (psp.Destination, error) {
	to = strings.TrimSpace(to)
	kind, err := ClassifyDestination(to)
	if err != nil {
		return psp.Destination{}, err
	}
	dest, err := s.deps.PSP.ResolveDestination(ctx, to)
	switch {
	case errors.Is(err, psp.ErrNotFound):
		return psp.Destination{}, apperr.Wrap(apperr.CodeDestinationNotFound, err, nil)
	case errors.Is(err, psp.ErrRejected):
		return psp.Destination{}, apperr.Wrap(apperr.CodeDestinationInvalid, err, nil)
	case errors.Is(err, psp.ErrUncertain):
		return psp.Destination{}, apperr.Wrap(apperr.CodeProviderUnavailable, err, nil)
	case err != nil:
		return psp.Destination{}, err
	}
	dest.Value = to
	if dest.Type == "" {
		dest.Type = kind
	}
	if !validSnapshot(dest) {
		return psp.Destination{}, apperr.New(apperr.CodeDestinationInvalid, nil)
	}
	return dest, nil
}

// validSnapshot mirrors the DB CHECKs of the payouts destination columns.
func validSnapshot(d psp.Destination) bool {
	name := len([]rune(d.HolderName))
	return accountNumberPattern.MatchString(d.AccountNumber) && cuitPattern.MatchString(d.HolderCUIT) &&
		name >= 1 && name <= maxHolderName && len([]rune(d.BankName)) <= maxBankName &&
		(d.Type == psp.DestinationCBU || d.Type == psp.DestinationCVU || d.Type == psp.DestinationAlias)
}
