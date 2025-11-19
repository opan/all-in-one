package auth

import (
	"golang.org/x/crypto/bcrypt"
)

func CheckPassword(password, hash string) (bool, error) {
	// Implement password hash checking logic here
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return (err == nil), err
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	// Implement password hashing logic here
	return string(hash), nil
}
