package models

import "github.com/gofrs/uuid"

type UUIDString string

type UUID struct {
	uuid.UUID
}
