package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

const maxPasswordBytes = 72

var (
	ErrPasswordEmpty   = errors.New("password must not be empty")
	ErrPasswordTooLong = errors.New("password must not exceed 72 bytes")
)

func HashPassword(password string) (string, error) {
	if password == "" {
		return "", ErrPasswordEmpty
	}
	if len(password) > maxPasswordBytes {
		return "", ErrPasswordTooLong
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
