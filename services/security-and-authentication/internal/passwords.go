package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const argon2Version = argon2.Version

type PasswordHasher struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

func NewPasswordHasher(
	memory uint32,
	iterations uint32,
	parallelism uint8,
	saltLength uint32,
	keyLength uint32,
) (*PasswordHasher, error) {
	if memory < 8192 || memory > 1048576 {
		return nil, fmt.Errorf("argon2 memory must be between 8192 and 1048576 KiB")
	}
	if iterations < 1 || iterations > 10 {
		return nil, fmt.Errorf("argon2 iterations must be between 1 and 10")
	}
	if parallelism < 1 || parallelism > 16 {
		return nil, fmt.Errorf("argon2 parallelism must be between 1 and 16")
	}
	if saltLength < 16 || saltLength > 64 {
		return nil, fmt.Errorf("argon2 salt length must be between 16 and 64 bytes")
	}
	if keyLength < 16 || keyLength > 64 {
		return nil, fmt.Errorf("argon2 key length must be between 16 and 64 bytes")
	}

	return &PasswordHasher{
		memory:      memory,
		iterations:  iterations,
		parallelism: parallelism,
		saltLength:  saltLength,
		keyLength:   keyLength,
	}, nil
}

func (h *PasswordHasher) Hash(password string) (string, error) {
	salt := make([]byte, h.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	key := argon2.IDKey(
		[]byte(password),
		salt,
		h.iterations,
		h.memory,
		h.parallelism,
		h.keyLength,
	)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2Version,
		h.memory,
		h.iterations,
		h.parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

func (h *PasswordHasher) Verify(password, encoded string) (bool, error) {
	params, salt, expected, err := parsePasswordHash(encoded)
	if err != nil {
		return false, err
	}

	actual := argon2.IDKey(
		[]byte(password),
		salt,
		params.iterations,
		params.memory,
		params.parallelism,
		uint32(len(expected)),
	)

	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func (h *PasswordHasher) NeedsRehash(encoded string) bool {
	params, _, expected, err := parsePasswordHash(encoded)
	if err != nil {
		return true
	}

	return params.version != argon2Version ||
		params.memory != h.memory ||
		params.iterations != h.iterations ||
		params.parallelism != h.parallelism ||
		uint32(len(expected)) != h.keyLength
}

type argon2Params struct {
	version     int
	memory      uint32
	iterations  uint32
	parallelism uint8
}

func parsePasswordHash(encoded string) (*argon2Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return nil, nil, nil, fmt.Errorf("unsupported password hash format")
	}

	versionPart := strings.TrimPrefix(parts[2], "v=")
	version, err := strconv.Atoi(versionPart)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid password hash version")
	}

	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(
		parts[3],
		"m=%d,t=%d,p=%d",
		&memory,
		&iterations,
		&parallelism,
	); err != nil {
		return nil, nil, nil, fmt.Errorf("invalid password hash parameters")
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid password hash salt")
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid password hash key")
	}
	if len(salt) < 8 || len(key) < 16 {
		return nil, nil, nil, fmt.Errorf("password hash parameters are too small")
	}

	return &argon2Params{
		version:     version,
		memory:      memory,
		iterations:  iterations,
		parallelism: parallelism,
	}, salt, key, nil
}

func ValidatePasswordPolicy(password string) error {
	if len(password) < 12 || len(password) > 128 {
		return newValidationError("password",
			"must be between 12 and 128 characters")
	}
	if !utf8.ValidString(password) {
		return newValidationError("password", "must be valid UTF-8")
	}

	var hasLower, hasUpper, hasDigit, hasSymbol bool
	for _, r := range password {
		switch {
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsSpace(r), unicode.IsControl(r):
			return newValidationError("password",
				"must not contain spaces or control characters")
		default:
			hasSymbol = true
		}
	}

	if !hasLower || !hasUpper || !hasDigit || !hasSymbol {
		return newValidationError("password",
			"must include lowercase, uppercase, digit, and symbol characters")
	}
	return nil
}
