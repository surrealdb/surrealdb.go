package httpclient

import (
	"fmt"
	"testing"

	"github.com/test-go/testify/assert"
	"github.com/test-go/testify/require"
)

func Test_Nominal(t *testing.T) {
	client := New("http://localhost:8000", "test", "test", "root", "root")

	resp, err := client.Execute("INFO FOR DB;")
	require.Nil(t, err)
	assert.Equal(t, "OK", resp.Status)
}

func Test_CreateAll(t *testing.T) {
	client := New("http://localhost:8000", "test", "test", "root", "root")

	resp, err := client.CreateAll("person", `{child: null, name: "FooBar"}`)
	require.Nil(t, err)
	assert.Equal(t, "OK", resp.Status)

	fmt.Println(resp.Result)
}

func Test_CreateOne(t *testing.T) {
	client := New("http://localhost:8000", "test", "test", "root", "root")

	resp, err := client.CreateOne("person", "surrealcreate", `{child: null, name: "FooBar"}`)
	require.Nil(t, err)
	assert.Equal(t, "OK", resp.Status)
	fmt.Println(resp.Result)
}

func Test_SelectAll(t *testing.T) {
	client := New("http://localhost:8000", "test", "test", "root", "root")

	resp, err := client.SelectAll("person")
	require.Nil(t, err)
	assert.Equal(t, "OK", resp.Status)
	fmt.Println(resp.Result)
}

func Test_SelectOne(t *testing.T) {
	client := New("http://localhost:8000", "test", "test", "root", "root")

	resp, err := client.CreateOne("person", "surreal", `{child: null, name: "FooBar"}`)
	require.Nil(t, err)
	fmt.Println(resp.Result)

	resp, err = client.SelectOne("person", "surreal")
	require.Nil(t, err)
	assert.Equal(t, "OK", resp.Status)
	fmt.Println(resp.Result)
}

func Test_ReplaceOne(t *testing.T) {
	client := New("http://localhost:8000", "test", "test", "root", "root")

	resp, err := client.CreateOne("personreplace", "surreal", `{child: null, name: "FooBar"}`)
	require.Nil(t, err)
	fmt.Println(resp.Result)

	resp, err = client.ReplaceOne("personreplace", "surreal", `{child: "DB", name: "FooBar", age: 1000}`)
	require.Nil(t, err)
	assert.Equal(t, "OK", resp.Status)
	fmt.Println(resp.Result)
}

func Test_UpsertOne(t *testing.T) {
	client := New("http://localhost:8000", "test", "test", "root", "root")

	_, err := client.CreateOne("person", "surrealupsert", `{child: null, name: "FooBar"}`)
	require.Nil(t, err)

	resp, err := client.UpsertOne("person", "surrealupsert", `{child: "DB", name: "FooBar"}`)
	require.Nil(t, err)
	fmt.Println(resp.Result)
}

func Test_DeleteOne(t *testing.T) {
	client := New("http://localhost:8000", "test", "test", "root", "root")

	_, err := client.CreateOne("person", "surrealdelete", `{child: null, name: "FooBar"}`)
	require.Nil(t, err)

	resp, err := client.DeleteOne("person", "surrealdelete")
	require.Nil(t, err)
	fmt.Println(resp.Result)

	resp2, err := client.SelectOne("person", "surrealdelete")
	require.Nil(t, err)
	fmt.Println(resp2.Result)
}

func Test_DeleteAll(t *testing.T) {
	client := New("http://localhost:8000", "test", "test", "root", "root")

	_, err := client.CreateOne("person", "surrealdeleteall1", `{child: null, name: "FooBar"}`)
	require.Nil(t, err)

	_, err = client.CreateOne("person", "surrealdeleteall2", `{child: null, name: "FooBar"}`)
	require.Nil(t, err)

	resp, err := client.DeleteAll("person")
	require.Nil(t, err)
	fmt.Println(resp.Result)

	resp2, err := client.SelectAll("person")
	require.Nil(t, err)
	fmt.Println(resp2.Result)
}
