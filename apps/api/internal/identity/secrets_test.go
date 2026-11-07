package identity

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/erfajc97/lenapay/apps/api/internal/platform/ids"
)

// Passwords are stored as argon2id PHC strings and only the right password verifies.
func TestPasswordHash(t *testing.T) {
	t.Parallel()
	hash := hashPassword("correct horse battery")
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("hash %q is not an argon2id PHC string with the OWASP profile", hash)
	}
	if hash == hashPassword("correct horse battery") {
		t.Fatal("two hashes of one password are equal: the salt is not random")
	}
	tests := []struct {
		name     string
		encoded  string
		password string
		want     bool
	}{
		{"right password", hash, "correct horse battery", true},
		{"wrong password", hash, "correct horse batterY", false},
		{"empty password", hash, "", false},
		{"not argon2id", strings.Replace(hash, "argon2id", "argon2i", 1), "correct horse battery", false},
		{"other version", strings.Replace(hash, "v=19", "v=16", 1), "correct horse battery", false},
		{"broken params", strings.Replace(hash, "m=19456", "m=x", 1), "correct horse battery", false},
		{"broken salt", strings.Replace(hash, "$c", "$!", 1) + "!", "correct horse battery", false},
		{"missing parts", "$argon2id$v=19", "correct horse battery", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := verifyPassword(tt.encoded, tt.password); got != tt.want {
				t.Fatalf("verifyPassword = %v, want %v", got, tt.want)
			}
		})
	}
}

// RFC 6238 appendix B, SHA-1 secret, truncated to 6 digits: at T=59 the code is 287082.
// One 30-second step of drift is accepted on each side, two are not.
func TestVerifyTOTP(t *testing.T) {
	t.Parallel()
	secret := []byte("12345678901234567890")
	at := time.Unix(59, 0)
	tests := []struct {
		name string
		code string
		now  time.Time
		want bool
	}{
		{"exact step", "287082", at, true},
		{"one step later still accepted", "287082", at.Add(30 * time.Second), true},
		{"two steps later rejected", "287082", at.Add(60 * time.Second), false},
		{"one step earlier still accepted", "287082", at.Add(-30 * time.Second), true},
		{"wrong code", "287083", at, false},
		{"short code", "28708", at, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := verifyTOTP(secret, tt.code, tt.now); got != tt.want {
				t.Fatalf("verifyTOTP = %v, want %v", got, tt.want)
			}
		})
	}
	if verifyTOTP(nil, "287082", at) {
		t.Fatal("an empty secret must never verify")
	}
}

func TestEmailCode(t *testing.T) {
	t.Parallel()
	six := regexp.MustCompile(`^[0-9]{6}$`)
	for range 50 {
		if c := newEmailCode(); !six.MatchString(c) {
			t.Fatalf("code %q is not 6 digits", c)
		}
	}
	a, b := ids.New(), ids.New()
	if equalHash(emailCodeHash(a, "123456"), emailCodeHash(b, "123456")) {
		t.Fatal("the same code of two users must hash differently")
	}
	if !equalHash(emailCodeHash(a, "123456"), emailCodeHash(a, "123456")) {
		t.Fatal("the hash of a code must be stable")
	}
}

func TestOpaqueTokens(t *testing.T) {
	t.Parallel()
	tok := newOpaqueToken()
	if len(tok) != 43 || tok == newOpaqueToken() {
		t.Fatalf("token %q: want 43 random base64url chars", tok)
	}
	if equalHash(tokenHash(tok), tokenHash(tok+"x")) || !equalHash(tokenHash(tok), tokenHash(tok)) {
		t.Fatal("token hash must be stable and distinct per token")
	}
}

func TestNormalizeEmail(t *testing.T) {
	t.Parallel()
	if got := normalizeEmail("  Ana.Perez@Example.COM "); got != "ana.perez@example.com" {
		t.Fatalf("normalizeEmail = %q", got)
	}
}
