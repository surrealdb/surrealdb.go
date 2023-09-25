package surrealdb_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/surrealdb/surrealdb.go"
)

type testRecordUser struct {
	ID      string                               `json:"id,omitempty"`
	Profile surrealdb.Record[*testRecordProfile] `json:"profile,omitempty"`
}

type testRecordProfile struct {
	ID  string `json:"id,omitempty"`
	Bio string `json:"bio,omitempty"`
}

func TestRecord(t *testing.T) {
	rules := []struct {
		Name     string
		Data     string
		Expected testRecordUser
	}{
		{
			Name: "OnlyID",
			Data: `{
  "id": "user:xjk6w1vrc3nxel2tic2b",
  "profile": "profile:9rp7kx7zc8o1c2up1dsu"
}`,
			Expected: testRecordUser{
				ID: "user:xjk6w1vrc3nxel2tic2b",
				Profile: surrealdb.Record[*testRecordProfile]{
					ID:     "profile:9rp7kx7zc8o1c2up1dsu",
					OnlyID: true,
				},
			},
		},
		{
			Name: "Fetched",
			Data: `{
  "id": "user:xjk6w1vrc3nxel2tic2b",
  "profile": {
	"id": "profile:9rp7kx7zc8o1c2up1dsu",
	"bio": "Watch anime Kill Me Baby"
  }
}`,
			Expected: testRecordUser{
				ID: "user:xjk6w1vrc3nxel2tic2b",
				Profile: surrealdb.Record[*testRecordProfile]{
					ID: "profile:9rp7kx7zc8o1c2up1dsu",
					Object: &testRecordProfile{
						ID:  "profile:9rp7kx7zc8o1c2up1dsu",
						Bio: "Watch anime Kill Me Baby",
					},
				},
			},
		},
	}
	for _, rule := range rules {
		t.Run(rule.Name, func(t *testing.T) {
			var actual testRecordUser
			err := json.Unmarshal([]byte(rule.Data), &actual)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(rule.Expected, actual) {
				t.Errorf("Expected %v, got %v", rule.Expected, actual)
			}
		})
	}
}
