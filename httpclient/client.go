package httpclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

// SurrealClient is a wrapper to more easily make HTTP calls to the SurrealDB engine
type SurrealClient struct {
	// URL is the base URL in SurrealDB to be called
	URL string
	// DB that you want to connect to
	DB string
	// Namespace that you want to connect to
	NS       string
	User     string
	Password string
}

type Response struct {
	Time   string `json:"time"`
	Status string `json:"status"`
	Result any    `json:"result"`
}

// New creates a new instance of a SurrealClient
func New(url, db, ns, user, password string) SurrealClient {
	return SurrealClient{
		URL:      url,
		DB:       db,
		NS:       ns,
		User:     user,
		Password: password,
	}
}

// Execute calls the endpoint POST /sql, executing whatever given statement
func (sc SurrealClient) Execute(query string) (Response, error) {
	return sc.Request("/sql", "POST", query)
}

// CreateOne calls the endpoint POST /key/:table/:id, executing the statement
//
// CREATE type::table($table) CONTENT $body;
func (sc SurrealClient) CreateOne(table, id, query string) (Response, error) {
	return sc.Request(fmt.Sprintf("/key/%s/%s", table, id), "POST", query)
}

// CreateAll calls the endpoint POST /key/:table, executing the statement
//
// CREATE type::thing($table, $id) CONTENT $body;
func (sc SurrealClient) CreateAll(table string, query string) (Response, error) {
	return sc.Request(fmt.Sprintf("/key/%s", table), "POST", query)
}

// SelectAll calls the endpoint GET /key/:table, executing the statement
//
// SELECT * FROM type::table($table);
func (sc SurrealClient) SelectAll(table string) (Response, error) {
	return sc.Request(fmt.Sprintf("/key/%s", table), "GET", "")
}

// SelectOne calls the endpoint GET /key/:table/:id, executing the statement
//
// SELECT * FROM type::thing(:table, :id);
func (sc SurrealClient) SelectOne(table string, id string) (Response, error) {
	return sc.Request(fmt.Sprintf("/key/%s/%s", table, id), "GET", "")
}

// ReplaceOne calls the endpoint PUT /key/:table/:id, executing the statement
//
// UPDATE type::thing($table, $id) CONTENT $body;
func (sc SurrealClient) ReplaceOne(table, id, query string) (Response, error) {
	return sc.Request(fmt.Sprintf("/key/%s/%s", table, id), "PUT", query)
}

// UpsertOne calls the endpoint PUT /key/:table/:id, executing the statement
//
// UPDATE type::thing($table, $id) MERGE $body;
func (sc SurrealClient) UpsertOne(table, id, query string) (Response, error) {
	return sc.Request(fmt.Sprintf("/key/%s/%s", table, id), "PATCH", query)
}

// DeleteOne calls the endpoint DELETE /key/:table/:id, executing the statement
//
// DELETE FROM type::thing($table, $id);
func (sc SurrealClient) DeleteOne(table, id string) (Response, error) {
	return sc.Request(fmt.Sprintf("/key/%s/%s", table, id), "DELETE", "")
}

// DeleteAll calls the endpoint DELETE /key/:table/, executing the statement
//
// DELETE FROM type::table($table);
func (sc SurrealClient) DeleteAll(table string) (Response, error) {
	return sc.Request(fmt.Sprintf("/key/%s", table), "DELETE", "")
}

// Request makes a request to surrealdb to the given endpoint, with the given data. Responses returned from
// surrealdb vary, and this function will only return the first response
// TODO: have it return the array, or some other data type that more properly reflects the responses
func (sc SurrealClient) Request(endpoint string, requestType string, body string) (Response, error) {
	client := &http.Client{}

	// TODO: verify its a valid requesttype
	req, err := http.NewRequest(requestType, sc.URL+endpoint, bytes.NewBufferString(body))
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("NS", sc.NS)
	req.Header.Set("DB", sc.DB)
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(sc.User, sc.Password)

	resp, err := client.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Response{}, err
	}

	var realResp []Response
	err = json.Unmarshal(data, &realResp)
	if err != nil {
		return Response{}, err
	}

	return realResp[0], err
}
