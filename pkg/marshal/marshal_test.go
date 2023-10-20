package marshal_test

import (
	"testing"

	"github.com/surrealdb/surrealdb.go/pkg/marshal"
)

type testMarshalStruct struct {
	TestInt    int    `json:"testInt"`
	TestBool   bool   `json:"testBool"`
	TestString string `json:"testString"`
}

func TestUnMarshalMapToStruct(t *testing.T) {
	testkey := "testKey"
	testDataMap := make(map[string]interface{})

	// nil data
	testDataMap[testkey] = nil

	testObj := new(testMarshalStruct)
	err := marshal.UnmarshalMapToStruct(testDataMap, testObj)
	if err == nil {
		t.Fatal("nil value need to give error")
	}

	testObj = &testMarshalStruct{
		TestInt:    15,
		TestBool:   true,
		TestString: "testString",
	}

	testDataMap["testInt"] = testObj.TestInt
	testDataMap["testBool"] = testObj.TestBool
	testDataMap["testString"] = testObj.TestString

	delete(testDataMap, testkey)

	err = marshal.UnmarshalMapToStruct(testDataMap, testObj)
	if err != nil {
		t.Fatal(err)
	}
}
