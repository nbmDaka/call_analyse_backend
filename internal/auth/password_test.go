package auth

import (
	"strings"
	"testing"
)

func TestPasswordHasherHashesAndVerifiesWithoutExposingPassword(t *testing.T) {
	const password = "correct horse battery staple"

	hasher := NewPasswordHasher()
	hash, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	if hash == password || strings.Contains(hash, password) {
		t.Fatal("Hash() exposed the plaintext password")
	}
	if err := hasher.Verify(hash, password); err != nil {
		t.Fatalf("Verify() error = %v, want nil for the original password", err)
	}
	if err := hasher.Verify(hash, "wrong password"); err == nil {
		t.Fatal("Verify() error = nil, want an error for a wrong password")
	}
}
