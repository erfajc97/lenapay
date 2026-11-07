package identity

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // RFC 6238 TOTP is defined over HMAC-SHA1; it is not used for hashing secrets
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
)

// Argon2id parameters: the OWASP minimum profile (19 MiB, 2 passes, 1 lane). Every hash stores
// its own parameters, so raising them later only affects new hashes.
const (
	argonMemoryKiB = 19 * 1024
	argonTime      = 2
	argonThreads   = 1
	argonKeyLen    = 32
	argonSaltLen   = 16
)

// hashPassword returns a PHC string: $argon2id$v=19$m=<KiB>,t=<passes>,p=<lanes>$<salt>$<hash>.
func hashPassword(password string) string {
	salt := randomBytes(argonSaltLen)
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemoryKiB, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemoryKiB, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
}

// errBadHash is a stored hash that is not a PHC argon2id string (it never matches).
var errBadHash = errors.New("identity: malformed password hash")

// verifyPassword reports whether password matches the PHC hash, in constant time.
func verifyPassword(encoded, password string) bool {
	params, salt, want, err := parseHash(encoded)
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, params.time, params.memory, params.threads, uint32(len(want))) //nolint:gosec // len(want) is a decoded 32-byte key
	return subtle.ConstantTimeCompare(got, want) == 1
}

type argonParams struct {
	memory  uint32
	time    uint32
	threads uint8
}

func parseHash(encoded string) (argonParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return argonParams{}, nil, nil, errBadHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return argonParams{}, nil, nil, errBadHash
	}
	var p argonParams
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.time, &p.threads); err != nil {
		return argonParams{}, nil, nil, errBadHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return argonParams{}, nil, nil, errBadHash
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return argonParams{}, nil, nil, errBadHash
	}
	return p, salt, key, nil
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b) // crypto/rand.Read never fails (it panics on a broken OS source)
	return b
}

// newOpaqueToken returns a random URL-safe token (refresh tokens, 256 bits).
func newOpaqueToken() string {
	return base64.RawURLEncoding.EncodeToString(randomBytes(32))
}

// tokenHash is how refresh tokens are stored: only the SHA-256 of the token.
func tokenHash(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// newEmailCode returns a uniformly random 6-digit code.
func newEmailCode() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1_000_000)) // crypto/rand.Int fails only on a non-positive max
	return fmt.Sprintf("%06d", n.Int64())
}

// emailCodeHash binds the code to its user, so equal codes of two users never share a hash.
func emailCodeHash(userID uuid.UUID, code string) []byte {
	sum := sha256.Sum256(append(userID[:], code...))
	return sum[:]
}

// equalHash compares two digests in constant time.
func equalHash(a, b []byte) bool { return subtle.ConstantTimeCompare(a, b) == 1 }

// totpPeriod and totpSkew follow RFC 6238 defaults: 30-second steps, one step of drift.
const (
	totpPeriod = 30
	totpSkew   = 1
)

// verifyTOTP checks a 6-digit RFC 6238 code (HMAC-SHA1) at now, accepting ±1 step of drift.
func verifyTOTP(secret []byte, code string, now time.Time) bool {
	if len(code) != 6 || len(secret) == 0 {
		return false
	}
	step := now.Unix() / totpPeriod
	ok := false
	for drift := int64(-totpSkew); drift <= totpSkew; drift++ {
		want := hotp(secret, uint64(step+drift)) //nolint:gosec // step is a positive Unix step count
		ok = subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 || ok
	}
	return ok
}

// hotp is RFC 4226 with 6 digits.
func hotp(secret []byte, counter uint64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], counter)
	mac := hmac.New(sha1.New, secret)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", value%1_000_000)
}
