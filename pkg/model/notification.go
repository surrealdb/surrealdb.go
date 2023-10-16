package model

type Notification struct {
	ID     string                 `json:"id"`
	Action Action                 `json:"action"`
	Result map[string]interface{} `json:"result"`
}

type Action string

const (
	CreateAction Action = "CREATE"
	UpdateAction Action = "UPDATE"
	DeleteAction Action = "DELETE"
)
