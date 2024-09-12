package connection

import (
	"github.com/stretchr/testify/assert"
	"github.com/surrealdb/surrealdb.go/pkg/model"
	"testing"
)

func TestEngine_WsMakeRequest(t *testing.T) {
	//httpClient := NewTestClient(func(req *http.Request) *http.Response {
	//	assert.Equal(t, req.URL.String(), "http://test.surreal/rpc")
	//
	//	return &http.Response{
	//		StatusCode: 400,
	//		// Send response to be tested
	//		Body: ioutil.NopCloser(bytes.NewBufferString(`OK`)),
	//		// Must be set to non-nil value or it panics
	//		Header: make(http.Header),
	//	}
	//})
	//
	//httpEngine := (NewHttp(p)).(*Http)
	//httpEngine.SetHttpClient(httpClient)
	//
	//resp, err := httpEngine.MakeRequest(http.MethodGet, "http://test.surreal/rpc", nil)
	//assert.Error(t, err, "should return error for status code 400")
	//
	//fmt.Println(resp)

	p := NewConnectionParams{
		Marshaler:   model.CborMarshaler{},
		Unmarshaler: model.CborUnmashaler{},
	}
	con := NewWebSocket(p)

	con, err := con.Connect("http://127.0.0.1:8000") // change con initialization?
	assert.Nil(t, err, "no error returned when initializing engine connection")

	err = con.Use("test", "test")
	assert.Nil(t, err, "no error returned when setting namespace and database")

	//con, err = con.Connect("http://127.0.0.1:8000")
	//assert.Nil(t, err, "no error returned when initializing engine connection")
	//
	//token, err := con.SignIn(model.Auth{Username: "pass", Password: "pass"})
	//assert.Nil(t, err, "no error returned when signing in")
	//fmt.Println(token)
	//
	//params := []interface{}{
	//	"SELECT marketing, count() FROM $tb GROUP BY marketing",
	//	map[string]interface{}{
	//		"datetime": time.Now(),
	//		"testnil":  nil,
	//		//"duration": Duration(340),
	//	},
	//}
	//res, err := con.Send("query", params)
	//assert.Nil(t, err, "no error returned when sending a query")
	//fmt.Println(res)
}
