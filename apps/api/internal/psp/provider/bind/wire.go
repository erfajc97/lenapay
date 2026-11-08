package bind

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// errBadAmount is an amount BIND sent that is not a two-decimal number.
var errBadAmount = errors.New("bind: malformed amount")

// decimal is an amount in minor units on BIND's wire: a JSON number with two decimals
// ("amount": 1500.25). It is converted digit by digit, never through a float (AG12).
type decimal int64

func (d decimal) MarshalJSON() ([]byte, error) {
	minor := int64(d)
	sign := ""
	if minor < 0 {
		sign, minor = "-", -minor
	}
	return fmt.Appendf(nil, "%s%d.%02d", sign, minor/100, minor%100), nil
}

func (d *decimal) UnmarshalJSON(b []byte) error {
	s := string(bytes.Trim(b, `"`))
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	units, frac, hasFrac := strings.Cut(s, ".")
	if units == "" || len(units) > 15 || len(frac) > 2 || (hasFrac && frac == "") || !allDigits(units) || !allDigits(frac) {
		return fmt.Errorf("%w: %q", errBadAmount, b)
	}
	u, err := strconv.ParseInt(units, 10, 64)
	if err != nil {
		return fmt.Errorf("%w: %q", errBadAmount, b)
	}
	f := int64(0)
	if frac != "" {
		f, _ = strconv.ParseInt((frac + "0")[:2], 10, 64) // allDigits checked it
	}
	minor := u*100 + f
	if neg {
		minor = -minor
	}
	*d = decimal(minor)
	return nil
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// errorBody is BIND's error answer.
type errorBody struct {
	Code   string `json:"codigo"`
	Detail string `json:"detalle"`
}

// Codes of BIND answers the adapter branches on.
const (
	codeAliasTaken       = "ALIAS_EN_USO"
	codeTransferNotFound = "TX019" // BIND answers a missing transfer with 422 + TX019, not 404
)

type holderJSON struct {
	Name string `json:"nombre"`
	CUIT string `json:"cuit"`
}

type createCajaRequest struct {
	Reference   string     `json:"referencia"`
	Currency    string     `json:"moneda"`
	Holder      holderJSON `json:"titular"`
	Alias       string     `json:"alias,omitempty"`
	RequiresCVU bool       `json:"requiereCvu"`
}

type cajaResponse struct {
	ID     string `json:"id"`
	CVU    string `json:"cvu"`
	Alias  string `json:"alias"`
	Status string `json:"estado"`
}

type valueJSON struct {
	Currency string  `json:"currency"`
	Amount   decimal `json:"amount"`
}

type transferRequest struct {
	OriginID    string            `json:"origin_id"`
	OriginDebit map[string]string `json:"origin_debit,omitempty"`
	To          map[string]string `json:"to"`
	Value       valueJSON         `json:"value"`
	Description string            `json:"description"`
	Concept     string            `json:"concept"`
	Emails      []string          `json:"emails"`
}

type counterpartyJSON struct {
	Name string `json:"name"`
	CUIT string `json:"cuit"`
	CBU  string `json:"cbu"`
}

type transferResponse struct {
	ID       string `json:"id"`
	OriginID string `json:"origin_id"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	Charge   struct {
		Value valueJSON `json:"value"`
	} `json:"charge"`
	CVU          string           `json:"cvu"`
	Counterparty counterpartyJSON `json:"counterparty"`
	StartDate    time.Time        `json:"start_date"`
	EndDate      *time.Time       `json:"end_date"`
	Details      *errorBody       `json:"details"`
}

type transferList struct {
	Items []transferResponse `json:"items"`
	Total int                `json:"total"`
}

type resolutionResponse struct {
	Owner struct {
		DisplayName string `json:"display_name"`
		ID          string `json:"id"`
	} `json:"owner"`
	Type    string `json:"type"`
	Routing struct {
		Address string `json:"address"`
	} `json:"account_routing"`
	Bank struct {
		Name string `json:"name"`
	} `json:"bank"`
}

// BIND transfer statuses and directions.
const (
	statusCompleted  = "COMPLETED"
	statusInProgress = "IN_PROGRESS"
	statusFailed     = "FAILED"
	typeCredit       = "CREDIT"
)
