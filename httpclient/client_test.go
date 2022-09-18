package httpclient

import (
	"fmt"
	"testing"

	"github.com/test-go/testify/require"
)

func Test_Nominal(t *testing.T) {
	client := New("http://localhost:8000", "test", "test", "root", "root")

	resp, err := client.Execute("INFO FOR DB;")
	require.Nil(t, err)
	fmt.Println(string(resp))
}

func Test_CreateAll(t *testing.T) {
	client := New("http://localhost:8000", "test", "test", "root", "root")

	resp, err := client.CreateAll("person", `{child: null, name: "FooBar"}`)
	require.Nil(t, err)
	fmt.Println(string(resp))
}

func Test_CreateOne(t *testing.T) {
	client := New("http://localhost:8000", "test", "test", "root", "root")

	resp, err := client.CreateOne("person", "surreal", `{child: null, name: "FooBar"}`)
	require.Nil(t, err)
	fmt.Println(string(resp))
}

func Test_SelectAll(t *testing.T) {
	client := New("http://localhost:8000", "test", "test", "root", "root")

	resp, err := client.SelectAll("transaction")
	require.Nil(t, err)
	fmt.Println(string(resp))
}

func Test_SelectOne(t *testing.T) {
	client := New("http://localhost:8000", "test", "test", "root", "root")

	resp, err := client.SelectOne("transaction", "3b1wx6plrru4lizvs6j8")
	require.Nil(t, err)
	fmt.Println(string(resp))
}

func Test_ReplaceOne(t *testing.T) {
	client := New("http://localhost:8000", "test", "test", "root", "root")

	_, err := client.CreateOne("person", "surreal", `{child: null, name: "FooBar"}`)
	require.Nil(t, err)

	resp, err := client.ReplaceOne("person", "surreal", `{child: "DB", name: "FooBar", age: 1000}`)
	require.Nil(t, err)
	fmt.Println(string(resp))
}

func Test_UpsertOne(t *testing.T) {
	client := New("http://localhost:8000", "test", "test", "root", "root")

	_, err := client.CreateOne("person", "surrealupsert", `{child: null, name: "FooBar"}`)
	require.Nil(t, err)

	resp, err := client.UpsertOne("person", "surrealupsert", `{child: "DB", name: "FooBar"}`)
	require.Nil(t, err)
	fmt.Println(string(resp))
}

func Test_DeleteOne(t *testing.T) {
	client := New("http://localhost:8000", "test", "test", "root", "root")

	_, err := client.CreateOne("person", "surrealdelete", `{child: null, name: "FooBar"}`)
	require.Nil(t, err)

	resp, err := client.DeleteOne("person", "surrealdelete")
	require.Nil(t, err)
	fmt.Println(string(resp))

	resp2, err := client.SelectOne("person", "surrealdelete")
	require.Nil(t, err)
	fmt.Println(string(resp2))
}

func Test_DeleteAll(t *testing.T) {
	client := New("http://localhost:8000", "test", "test", "root", "root")

	_, err := client.CreateOne("person", "surrealdeleteall1", `{child: null, name: "FooBar"}`)
	require.Nil(t, err)

	_, err = client.CreateOne("person", "surrealdeleteall2", `{child: null, name: "FooBar"}`)
	require.Nil(t, err)

	resp, err := client.DeleteAll("person")
	require.Nil(t, err)
	fmt.Println(string(resp))

	resp2, err := client.SelectAll("person")
	require.Nil(t, err)
	fmt.Println(string(resp2))
}
