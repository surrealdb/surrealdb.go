package connection

import "github.com/surrealdb/surrealdb.go/v2/pkg/models"

type Notification struct {
	ID     *models.UUID `json:"id,omitempty"`
	Action Action       `json:"action"`
	Result interface{}  `json:"result"`
}
type Action string

const (
	CreateAction Action = "CREATE"
	UpdateAction Action = "UPDATE"
	DeleteAction Action = "DELETE"
)
