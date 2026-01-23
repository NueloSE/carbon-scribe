package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
}

func NewAuthService() *AuthService {
	return &AuthService{}
}

func (s *AuthService) Register(email string, password string) error {
	if email == "" || password == "" {
		return errors.New("email and password are required")
	}

	_, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// NOTE:
	// This is where DB persistence will go later.
	// For now, we just simulate success.

	return nil
}

func (s *AuthService) Login(email string, password string) error {
	if email == "" || password == "" {
		return errors.New("email and password are required")
	}

	// NOTE:
	// This is where user lookup + password comparison will go later.

	return nil
}
