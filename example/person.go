package main

import (
	"encoding/json"
	"fmt"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

type Person struct {
	ID       *models.RecordID     `json:"id,omitempty"`
	Name     string               `json:"name"`
	Surname  string               `json:"surname"`
	Location models.GeometryPoint `json:"location"`
}

type CustomRecordID struct {
	models.RecordID
}

func (r CustomRecordID) MarshalJSON() ([]byte, error) {
	return json.Marshal(fmt.Sprintf("%s:%v", r.Table, r.ID))
}

type PersonWithCustomID struct {
	ID       CustomRecordID       `json:"id,omitempty"`
	Name     string               `json:"name"`
	Surname  string               `json:"surname"`
	Location models.GeometryPoint `json:"location"`
}
