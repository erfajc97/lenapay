package bank

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"
)

// The 22-digit Argentine account number (CBU for banks, CVU for PSPs) is two blocks, each
// ending in a check digit: block 1 = entity (3) + branch (4) + check; block 2 = account (13)
// + check. A CVU starts with "000" and carries the PSP id in the branch digits.
var (
	block1Weights = []int{7, 1, 3, 9, 7, 1, 3}
	block2Weights = []int{3, 9, 7, 1, 3, 9, 7, 1, 3, 9, 7, 1, 3}
	aliasPattern  = regexp.MustCompile(`^[a-zA-Z0-9.-]{6,20}$`)
)

// cvuPrefix is block 1 without its check digit: "000" + PSP id 0322 (BIND in this mock).
const cvuPrefix = "0000322"

// Well-known test values (documented in the package doc).
const (
	// notFoundAccountPrefix: a valid CBU/CVU whose 13-digit account starts with it is unknown.
	notFoundAccountPrefix = "9999"
	// notFoundAliasPrefix: an alias starting with it is unknown.
	notFoundAliasPrefix = "noexiste"
)

func checkDigit(digits string, weights []int) byte {
	sum := 0
	for i, w := range weights {
		sum += int(digits[i]-'0') * w
	}
	return byte('0' + (10-sum%10)%10)
}

// ValidAccountNumber reports whether s is 22 digits with both check digits right.
func ValidAccountNumber(s string) bool {
	if len(s) != 22 || !digits(s) {
		return false
	}
	return s[7] == checkDigit(s[:7], block1Weights) && s[21] == checkDigit(s[8:21], block2Weights)
}

// ValidAlias reports whether s is a well-formed alias (6–20 of letters, digits, dot, dash).
func ValidAlias(s string) bool { return aliasPattern.MatchString(s) }

// newCVU builds the CVU of the n-th caja (valid check digits, unique per n).
func newCVU(n int64) string {
	account := fmt.Sprintf("%013d", 1_000_000+n)
	return cvuPrefix + string(checkDigit(cvuPrefix, block1Weights)) + account + string(checkDigit(account, block2Weights))
}

// newAlias builds a readable alias for the n-th caja, e.g. "puma.rio.0042".
func newAlias(n int64) string {
	words := []string{"puma", "rio", "sol", "luna", "cerro", "pampa", "mate", "tango", "lago", "nube", "ceibo", "condor"}
	return fmt.Sprintf("%s.%s.%04d", words[n%int64(len(words))], words[(n/7+3)%int64(len(words))], n%10000)
}

// Holder is the owner of an account as BIND reports it.
type Holder struct {
	Name string `json:"name"`
	CUIT string `json:"cuit"`
}

// externalHolder is a deterministic fake owner for an account that is not ours: the same
// number always resolves to the same person or company.
func externalHolder(seed string) Holder {
	names := []string{
		"Juan Pérez", "María González", "Construcciones del Sur S.A.", "Lucía Fernández",
		"Corralón Rivadavia S.R.L.", "Carlos Gómez", "Ana Martínez", "Hormigones Pampa S.A.",
		"Diego Romero", "Sofía López", "Estudio Aráoz & Asoc.", "Martín Díaz",
	}
	h := hash(seed)
	name := names[h%uint64(len(names))]
	prefix := "20"
	if strings.Contains(name, "S.A.") || strings.Contains(name, "S.R.L.") || strings.Contains(name, "Asoc.") {
		prefix = "30"
	}
	return Holder{Name: name, CUIT: cuit(prefix, fmt.Sprintf("%08d", h%100_000_000))}
}

// cuit appends the modulo-11 check digit to prefix + 8 digits (11 → 0, 10 → 9).
func cuit(prefix, body string) string {
	base := prefix + body
	weights := []int{5, 4, 3, 2, 7, 6, 5, 4, 3, 2}
	sum := 0
	for i, w := range weights {
		sum += int(base[i]-'0') * w
	}
	check := 11 - sum%11
	switch check {
	case 11:
		check = 0
	case 10:
		check = 9
	}
	return fmt.Sprintf("%s%d", base, check)
}

// bankName names the entity of a CBU (or "Billetera virtual" for another PSP's CVU).
func bankName(number string) string {
	if strings.HasPrefix(number, cvuPrefix) {
		return "BIND - Banco Industrial (CVU)"
	}
	if strings.HasPrefix(number, "000") {
		return "Billetera virtual"
	}
	switch number[:3] {
	case "007":
		return "Banco de Galicia"
	case "011":
		return "Banco de la Nación Argentina"
	case "017":
		return "BBVA Argentina"
	case "072":
		return "Banco Santander Argentina"
	case "285":
		return "Banco Macro"
	case "322":
		return "Banco Industrial"
	}
	return "Banco"
}

// externalAccountForAlias is the deterministic CBU behind an alias that is not ours.
func externalAccountForAlias(alias string) string {
	block1 := "0070999"
	account := fmt.Sprintf("%013d", hash(strings.ToLower(alias))%1_000_000_000_000)
	return block1 + string(checkDigit(block1, block1Weights)) + account + string(checkDigit(account, block2Weights))
}

// AccountNumberFor builds a valid CBU (entity "007", branch "0999") for test fixtures:
// account is up to 13 digits. Exported for tests and the auto-payin origin accounts.
func AccountNumberFor(account string) string {
	block1 := "0070999"
	acc := fmt.Sprintf("%013s", account)
	acc = strings.ReplaceAll(acc, " ", "0")
	return block1 + string(checkDigit(block1, block1Weights)) + acc + string(checkDigit(acc, block2Weights))
}

func hash(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s)) // fnv never fails
	return h.Sum64()
}
