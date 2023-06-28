package mta

import "fmt"

// AuthBackend represents a pluggable authentication backend for the MTA
type AuthBackend interface {
	// Login checks whether the credentials of a user are valid
	Login(username string, password string) (bool, error)
}

// AuthBackendMemory is a simple in-memory implementation of AuthBackend for testing purpose.
type AuthBackendMemory struct {
	Credentials map[string]string
}

// Login checks whether the credentials of a user are valid
func (auth *AuthBackendMemory) Login(username string, password string) (bool, error) {
	if auth.Credentials == nil {
		return false, fmt.Errorf("auth backend not initialized")
	}
	if passwordToMatch, ok := auth.Credentials[username]; ok {
		if passwordToMatch == password {
			return true, nil
		}
		return false, nil
	}
	return false, nil
}

// NewAuthBackendMemory creates a new in-memory AuthBackend
func NewAuthBackendMemory(credentials map[string]string) *AuthBackendMemory {
	return &AuthBackendMemory{
		Credentials: credentials,
	}
}
