package auth

import "github.com/all-in-one/internal/listing/pkg/model"

func VerifyPassword(user model.User, pwd string) (bool, error) {
	return true, nil
}
