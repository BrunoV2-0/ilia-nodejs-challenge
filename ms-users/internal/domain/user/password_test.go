package user_test

import (
	"strings"
	"testing"

	"github.com/ilia/ms-users/internal/domain/user"
)

func TestHashPassword_ProducesArgon2idFormat(t *testing.T) {
	hash, err := user.HashPassword("secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("expected Argon2id prefix, got %q", hash)
	}
}

func TestHashPassword_DifferentSaltEachCall(t *testing.T) {
	h1, _ := user.HashPassword("secret")
	h2, _ := user.HashPassword("secret")
	if h1 == h2 {
		t.Error("two hashes of the same password must differ due to random salt")
	}
}

func TestVerifyPassword_CorrectPassword(t *testing.T) {
	hash, _ := user.HashPassword("correct")
	if !user.VerifyPassword("correct", hash) {
		t.Error("expected VerifyPassword to return true for correct password")
	}
}

func TestVerifyPassword_WrongPassword(t *testing.T) {
	hash, _ := user.HashPassword("correct")
	if user.VerifyPassword("wrong", hash) {
		t.Error("expected VerifyPassword to return false for wrong password")
	}
}

func TestVerifyPassword_MalformedHash(t *testing.T) {
	if user.VerifyPassword("any", "not-a-valid-hash") {
		t.Error("expected false for malformed hash")
	}
}
