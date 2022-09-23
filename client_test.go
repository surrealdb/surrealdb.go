package surrealdb

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.Nil(t, err)
	assert.Equal(t, "OK", resp.Status)
}

func Test_CreateAll(t *testing.T) {
	client := NewClient("http://localhost:8000", "test", "test", "root", "root")

	resp, err := client.CreateAll("person", `{child: null, name: "FooBar"}`)
	require.Nil(t, err)
	assert.Equal(t, "OK", resp.Status)

	fmt.Println(resp.Result)
}

func Test_CreateOne(t *testing.T) {
	client := NewClient("http://localhost:8000", "test", "test", "root", "root")

	resp, err := client.CreateOne("person", "surrealcreate", `{child: null, name: "FooBar"}`)
	require.Nil(t, err)
	assert.Equal(t, "OK", resp.Status)
	fmt.Println(resp.Result)
}

func Test_SelectAll(t *testing.T) {
	client := NewClient("http://localhost:8000", "test", "test", "root", "root")

	resp, err := client.SelectAll("person")
	require.Nil(t, err)
	assert.Equal(t, "OK", resp.Status)
	fmt.Println(resp.Result)
}

func Test_SelectOne(t *testing.T) {
	client := NewClient("http://localhost:8000", "test", "test", "root", "root")

	expectedPerson := person{
		Child: "hello",
		Name:  "FooBar",
	}

	resp, err := client.CreateOne("person", "surreal", `{child: "hello", name: "FooBar"}`)
	require.Nil(t, err)
	fmt.Println(resp.Result)

	resp, err = client.SelectOne("person", "surreal")
	require.Nil(t, err)
	assert.Equal(t, "OK", resp.Status)
	fmt.Println(resp.Result)

	var actualPerson person
	err = Unmarshal(resp.Result, &actualPerson)
	require.Nil(t, err)
	assert.Equal(t, expectedPerson, actualPerson)
}

func Test_ReplaceOne(t *testing.T) {
	client := NewClient("http://localhost:8000", "test", "test", "root", "root")

	resp, err := client.CreateOne("personreplace", "surreal", `{child: null, name: "FooBar"}`)
	require.Nil(t, err)
	fmt.Println(resp.Result)

	resp, err = client.ReplaceOne("personreplace", "surreal", `{child: "DB", name: "FooBar", age: 1000}`)
	require.Nil(t, err)
	assert.Equal(t, "OK", resp.Status)
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
	require.Nil(t, err)
	fmt.Println(resp.Result)

	// Replace with the struct
	resp, err = client.ReplaceOne("personreplace", "surrealinterface", expectedPerson)
	require.Nil(t, err)
	assert.Equal(t, "OK", resp.Status)
	fmt.Println(resp.Result)

	// Now select it to ensure that it is correct still
	resp, err = client.SelectOne("personreplace", "surrealinterface")
	require.Nil(t, err)
	assert.Equal(t, "OK", resp.Status)
	fmt.Println(resp.Result)

	// Unmarshal and verify its the same
	var actualPerson person
	err = Unmarshal(resp.Result, &actualPerson)
	require.Nil(t, err)
	assert.Equal(t, expectedPerson, actualPerson)
}

func Test_UpsertOne(t *testing.T) {
	client := NewClient("http://localhost:8000", "test", "test", "root", "root")
	expectedPerson := person{
		Child: "DB",
		Name:  "FooBar",
		Age:   0,
	}

	_, err := client.CreateOne("person", "surrealupsert", `{child: null, name: "FooBar"}`)
	require.Nil(t, err)

	resp, err := client.UpsertOne("person", "surrealupsert", `{child: "DB", name: "FooBar"}`)
	require.Nil(t, err)

	// Unmarshal and verify its the same
	var actualPerson person
	err = Unmarshal(resp.Result, &actualPerson)
	require.Nil(t, err)
	assert.Equal(t, expectedPerson, actualPerson)
}

func Test_DeleteOne(t *testing.T) {
	client := NewClient("http://localhost:8000", "test", "test", "root", "root")

	_, err := client.CreateOne("person", "surrealdelete", `{child: null, name: "FooBar"}`)
	require.Nil(t, err)

	resp, err := client.DeleteOne("person", "surrealdelete")
	require.Nil(t, err)
	assert.Equal(t, []interface{}{}, resp.Result)

	resp2, err := client.SelectOne("person", "surrealdelete")
	require.Nil(t, err)
	assert.Equal(t, []interface{}{}, resp2.Result)
}

func Test_DeleteAll(t *testing.T) {
	client := NewClient("http://localhost:8000", "test", "test", "root", "root")

	_, err := client.CreateOne("person", "surrealdeleteall1", `{child: null, name: "FooBar"}`)
	require.Nil(t, err)

	_, err = client.CreateOne("person", "surrealdeleteall2", `{child: null, name: "FooBar"}`)
	require.Nil(t, err)

	resp, err := client.DeleteAll("person")
	require.Nil(t, err)
	assert.Equal(t, []interface{}{}, resp.Result)

	resp2, err := client.SelectAll("person")
	require.Nil(t, err)
	assert.Equal(t, []interface{}{}, resp2.Result)
}
