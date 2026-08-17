package auth

import "golang.org/x/crypto/bcrypt"

const bcryptCost = bcrypt.DefaultCost

type passwordHasher struct{}

// NewPasswordHasher returns the bcrypt password implementation used by the API.
func NewPasswordHasher() PasswordHasher {
	return passwordHasher{}
}

func (passwordHasher) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func (passwordHasher) Verify(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
