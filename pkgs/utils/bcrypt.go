package utils

import (
	"golang.org/x/crypto/bcrypt"
)

var cost = 12

func GenBcryptHash(str string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(str), cost)
	if err != nil {
		return "", err
	}
	return string(hashedBytes), nil
}
