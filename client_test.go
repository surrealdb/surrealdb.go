package surrealdb

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
)

// TODO for testing: adding in more edge case detections, including NULLs and other weird scenarios. Most of these are just
// best case scenarios

type person struct {
	Child string `json:"child"`
	Name  string `json:"name"`
	Age   int    `json:"age"`
}

func Test_Nominal(t *testing.T) {
	client := NewClient("http://localhost:8000", "test", "test", "root", "root")

	resp, err := client.Execute("INFO FOR DB;")
	IsEqual(t, nil, err)
	IsEqual(t, "OK", resp.Status)
}

func Test_Create(t *testing.T) {
	client := NewClient("http://localhost:8000", "test", "test", "root", "root")

	resp, err := client.Create("person", `{child: null, name: "FooBar"}`)
	IsEqual(t, nil, err)
	IsEqual(t, "OK", resp.Status)

	fmt.Println(resp.Result)
}

func Test_CreateOne(t *testing.T) {
	client := NewClient("http://localhost:8000", "test", "test", "root", "root")

	resp, err := client.CreateOne("person", "surrealcreate", `{child: null, name: "FooBar"}`)
	IsEqual(t, nil, err)
	IsEqual(t, "OK", resp.Status)
	fmt.Println(resp.Result)
}

func Test_SelectAll(t *testing.T) {
	client := NewClient("http://localhost:8000", "test", "test", "root", "root")

	resp, err := client.SelectAll("person")
	IsEqual(t, nil, err)
	IsEqual(t, "OK", resp.Status)
	fmt.Println(resp.Result)
}

func Test_SelectOne(t *testing.T) {
	client := NewClient("http://localhost:8000", "test", "test", "root", "root")

	expectedPerson := person{
		Child: "hello",
		Name:  "FooBar",
	}

	resp, err := client.CreateOne("person", "surreal", `{child: "hello", name: "FooBar"}`)
	IsEqual(t, nil, err)
	fmt.Println(resp.Result)

	resp, err = client.SelectOne("person", "surreal")
	IsEqual(t, nil, err)
	IsEqual(t, "OK", resp.Status)
	fmt.Println(resp.Result)

	var actualPerson person
	err = Unmarshal(resp.Result, &actualPerson)
	IsEqual(t, nil, err)
	IsEqual(t, expectedPerson, actualPerson)
}

func Test_ReplaceOne(t *testing.T) {
	client := NewClient("http://localhost:8000", "test", "test", "root", "root")

	resp, err := client.CreateOne("personreplace", "surreal", `{child: null, name: "FooBar"}`)
	IsEqual(t, nil, err)
	fmt.Println(resp.Result)

	resp, err = client.ReplaceOne("personreplace", "surreal", `{child: "DB", name: "FooBar", age: 1000}`)
	IsEqual(t, nil, err)
	IsEqual(t, "OK", resp.Status)
	fmt.Println(resp.Result)
}

func Test_ReplaceOne_PassInInterface(t *testing.T) {
	client := NewClient("http://localhost:8000", "test", "test", "root", "root")

	expectedPerson := person{
		Child: "interface",
		Name:  "interface",
		Age:   100,
	}

	resp, err := client.CreateOne("personreplace", "surrealinterface", `{child: null, name: "FooBar"}`)
	IsEqual(t, nil, err)
	fmt.Println(resp.Result)

	// Replace with the struct
	resp, err = client.ReplaceOne("personreplace", "surrealinterface", expectedPerson)
	IsEqual(t, nil, err)
	IsEqual(t, "OK", resp.Status)
	fmt.Println(resp.Result)

	// Now select it to ensure that it is correct still
	resp, err = client.SelectOne("personreplace", "surrealinterface")
	IsEqual(t, nil, err)
	IsEqual(t, "OK", resp.Status)
	fmt.Println(resp.Result)

	// Unmarshal and verify its the same
	var actualPerson person
	err = Unmarshal(resp.Result, &actualPerson)
	IsEqual(t, nil, err)
	IsEqual(t, expectedPerson, actualPerson)
}

func Test_UpsertOne(t *testing.T) {
	client := NewClient("http://localhost:8000", "test", "test", "root", "root")
	expectedPerson := person{
		Child: "DB",
		Name:  "FooBar",
		Age:   0,
	}

	_, err := client.CreateOne("person", "surrealupsert", `{child: null, name: "FooBar"}`)
	IsEqual(t, nil, err)

	resp, err := client.UpsertOne("person", "surrealupsert", `{child: "DB", name: "FooBar"}`)
	IsEqual(t, nil, err)

	// Unmarshal and verify its the same
	var actualPerson person
	err = Unmarshal(resp.Result, &actualPerson)
	IsEqual(t, nil, err)
	IsEqual(t, expectedPerson, actualPerson)
}

func Test_DeleteOne(t *testing.T) {
	client := NewClient("http://localhost:8000", "test", "test", "root", "root")

	_, err := client.CreateOne("person", "surrealdelete", `{child: null, name: "FooBar"}`)
	IsEqual(t, nil, err)

	resp, err := client.DeleteOne("person", "surrealdelete")
	IsEqual(t, nil, err)
	IsEqual(t, []interface{}{}, resp.Result)

	resp2, err := client.SelectOne("person", "surrealdelete")
	IsEqual(t, nil, err)
	IsEqual(t, []interface{}{}, resp2.Result)
}

func Test_DeleteAll(t *testing.T) {
	client := NewClient("http://localhost:8000", "test", "test", "root", "root")

	_, err := client.CreateOne("person", "surrealdeleteall1", `{child: null, name: "FooBar"}`)
	IsEqual(t, nil, err)

	_, err = client.CreateOne("person", "surrealdeleteall2", `{child: null, name: "FooBar"}`)
	IsEqual(t, nil, err)

	resp, err := client.DeleteAll("person")
	IsEqual(t, nil, err)
	IsEqual(t, []interface{}{}, resp.Result)

	resp2, err := client.SelectAll("person")
	IsEqual(t, nil, err)
	IsEqual(t, []interface{}{}, resp2.Result)
}

// Testing helpers brought in from testify, with extras removed

func IsEqual(t *testing.T, expected, actual interface{}) bool {
	if !ObjectsAreEqual(expected, actual) {
		t.Errorf("Not equal: \n"+
			"expected: %s\n"+
			"actual  : %s", expected, actual)
		return false
	}
	return true
}

// ObjectsAreEqual determines if two objects are considered equal.
//
// This function does no assertion of any kind.
func ObjectsAreEqual(expected, actual interface{}) bool {
	if expected == nil || actual == nil {
		return expected == actual
	}

	exp, ok := expected.([]byte)
	if !ok {
		return reflect.DeepEqual(expected, actual)
	}

	act, ok := actual.([]byte)
	if !ok {
		return false
	}
	if exp == nil || act == nil {
		return exp == nil && act == nil
	}
	return bytes.Equal(exp, act)
}
