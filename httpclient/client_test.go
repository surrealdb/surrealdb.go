package httpclient

import (
	"fmt"
	"testing"

	"github.com/test-go/testify/require"
)

func Test_Nominal(t *testing.T) {
	client := New("http://localhost:8000/sql", "test", "test", "root", "root")

	resp, err := client.RunQuery("INFO FOR DB;")
	require.Nil(t, err)
	fmt.Println(resp)
}
