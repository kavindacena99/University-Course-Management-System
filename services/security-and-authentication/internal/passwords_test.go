package auth

import (
	"testing"
)

func newTestHasher(t *testing.T) *PasswordHasher {
	t.Helper()

	hasher, err := NewPasswordHasher(8192, 1, 1, 16, 16)
	if err != nil {
		t.Fatalf("create password hasher: %v", err)
	}
	return hasher
}

func TestPasswordHasherHashAndVerify(t *testing.T) {
	hasher := newTestHasher(t)
	password := "StrongPass1!"

	encoded, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if encoded == password {
		t.Fatal("hash must not equal the plaintext password")
	}

	matches, err := hasher.Verify(password, encoded)
	if err != nil {
		t.Fatalf("verify password: %v", err)
	}
	if !matches {
		t.Fatal("expected password to match")
	}

	matches, err = hasher.Verify("WrongPass1!", encoded)
	if err != nil {
		t.Fatalf("verify wrong password: %v", err)
	}
	if matches {
		t.Fatal("expected wrong password not to match")
	}
}

func TestPasswordHasherNeedsRehash(t *testing.T) {
	hasher := newTestHasher(t)
	encoded, err := hasher.Hash("StrongPass1!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if hasher.NeedsRehash(encoded) {
		t.Fatal("expected hash not to need rehashing")
	}

	other, err := NewPasswordHasher(16384, 2, 1, 16, 16)
	if err != nil {
		t.Fatalf("create alternate hasher: %v", err)
	}
	if !other.NeedsRehash(encoded) {
		t.Fatal("expected hash to need rehashing")
	}
	if !hasher.NeedsRehash("not-a-hash") {
		t.Fatal("expected malformed hash to require rehashing")
	}
}

func TestPasswordHasherRejectsMalformedHash(t *testing.T) {
	hasher := newTestHasher(t)

	if _, err := hasher.Verify("password", "not-a-hash"); err == nil {
		t.Fatal("expected malformed hash to fail verification")
	}
}

func TestValidatePasswordPolicy(t *testing.T) {
	valid := []string{
		"StrongPass1!",
		"Another$Good2Password",
	}
	for _, password := range valid {
		if err := ValidatePasswordPolicy(password); err != nil {
			t.Errorf("ValidatePasswordPolicy(%q) = %v", password, err)
		}
	}

	invalid := map[string]string{
		"short":          "Short1!",
		"no lowercase":   "UPPERCASE1!",
		"no uppercase":   "lowercase1!",
		"no digit":       "NoDigitsHere!",
		"no symbol":      "NoSymbols123",
		"contains space": "Strong Pass1!",
		"contains tab":   "Strong\tPass1!",
	}
	for name, password := range invalid {
		t.Run(name, func(t *testing.T) {
			if err := ValidatePasswordPolicy(password); err == nil {
				t.Fatalf("ValidatePasswordPolicy(%q) accepted invalid password", password)
			}
		})
	}
}
