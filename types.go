package surrealdb

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

var (
	ErrInvalidToken = errors.New("token string is invalid")
)

// Patch represents a patch object set to MODIFY a record
type Patch struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

type UserInfo struct {
	User      string `json:"user"`
	Password  string `json:"pass"`
	Namespace string `json:"NS,omitempty"`
	Database  string `json:"DB,omitempty"`
	Scope     string `json:"SC,omitempty"`
}

type AuthenticationResult struct {
	Success bool   `json:"success"`
	Token   string `json:"token"`

	TokenData
}

type TokenData struct {
	IssuedAt  int    `json:"iat"`
	NotBefore int    `json:"nbf"`
	ExpiresAt int    `json:"exp"`
	Issuer    string `json:"iss"`
	Namespace string `json:"ns"`
	Database  string `json:"db"`
	Scope     string `json:"sc"`
	Id        string `json:"id"`
}

func (token TokenData) FromToken(tokenString string) (TokenData, error) {
	data := TokenData{}

	if tokenString == "" {
		return data, ErrInvalidToken
	}

	segments := strings.Split(tokenString, ".")
	if len(segments) != 3 {
		return data, ErrInvalidToken
	}

	// Decode the payload
	payload, err := base64.RawStdEncoding.DecodeString(segments[1])
	if err != nil {
		return data, err
	}

	// Unmarshal the payload
	err = json.Unmarshal(payload, &data)
	if err != nil {
		return data, err
	}

	return data, nil
}
