package server

import (
	"errors"
	"fmt"
)

// AuthBackend represents a pluggable authentication backend for the MTA
type AuthBackend interface {
	// Login checks whether the credentials of a user are valid.
	// returns ErrInvalidCredentials if credentials not valid.
	Login(username string, password string) (*User, error)
}

// User denotes an authenticated SMTP user.
type User struct {
	// Username is the username / email address of the user.
	username string
}

// Username returns the username
func (u *User) Username() string {
	return u.username
}

// ErrInvalidCredentials denotes incorrect credentials.
var ErrInvalidCredentials = errors.New("InvalidCredentialsError")

// AuthBackendMemory is a simple in-memory implementation of AuthBackend for testing purpose.
type AuthBackendMemory struct {
	Credentials map[string]string
}

// Login checks whether the credentials of a user are valid
func (auth *AuthBackendMemory) Login(username string, password string) (*User, error) {
	if auth.Credentials == nil {
		return nil, fmt.Errorf("auth backend not initialized")
	}
	if passwordToMatch, ok := auth.Credentials[username]; ok {
		if passwordToMatch == password {
			return &User{username: username}, nil
		}
		return nil, ErrInvalidCredentials
	}
	return nil, ErrInvalidCredentials
}

// NewAuthBackendMemory creates a new in-memory AuthBackend
func NewAuthBackendMemory(credentials map[string]string) *AuthBackendMemory {
	return &AuthBackendMemory{
		Credentials: credentials,
	}
}
