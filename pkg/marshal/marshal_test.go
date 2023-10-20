package marshal_test

import (
	"testing"

	"github.com/surrealdb/surrealdb.go/pkg/marshal"
)

type testMarshalStruct struct {
	TestInt  int  `json:"TestInt"`
	TestBool bool `json:"TestBool"`
}

func TestUnMarshalMapToStruct(t *testing.T) {
	testDataMap := make(map[string]interface{}, 2)

	testObj := &testMarshalStruct{
		TestInt:  15,
		TestBool: true,
	}

	testDataMap["TestInt"] = testObj.TestInt
	testDataMap["TestBool"] = testObj.TestBool

	err := marshal.UnmarshalMapToStruct(testDataMap, testObj)
	if err != nil {
		t.Fatal(err)
	}
}
