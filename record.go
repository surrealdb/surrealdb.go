package surrealdb

import "encoding/json"

type Record[T any] struct {
	ID     string
	OnlyID bool
	Object T
}

func (r *Record[T]) UnmarshalJSON(data []byte) error {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if v == nil {
		return nil
	}
	if id, ok := v.(string); ok {
		r.ID = id
		r.OnlyID = true
		return nil
	}
	r.ID = v.(map[string]any)["id"].(string)
	return json.Unmarshal(data, &r.Object)
}
