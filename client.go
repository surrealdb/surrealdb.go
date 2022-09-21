package surrealdb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

// Client is a wrapper to more easily make HTTP calls to the SurrealDB engine
type Client struct {
	// URL is the base URL in SurrealDB to be called
	URL string
	// Namespace that you want to connect to
	NS string
	// Database that you want to connect to
	DB string
	// The user to authenticate as
	User string
	// The password to authenticate with
	Pass string
}

type Response struct {
	Time   string      `json:"time"`
	Status string      `json:"status"`
	Result interface{} `json:"result"`
}

// New creates a new instance of a Client
func NewClient(url, ns, db, user, pass string) Client {
	return Client{
		URL:  url,
		NS:   ns,
		DB:   db,
		User: user,
		Pass: pass,
	}
}

// Execute calls the endpoint POST /sql, executing whatever given statement
func (sc Client) Execute(query string) (Response, error) {
	return sc.Request("/sql", "POST", query)
}

// CreateOne calls the endpoint POST /key/:table/:id, executing the statement
//
// CREATE type::table($table) CONTENT $body;
func (sc Client) CreateOne(table, id, query string) (Response, error) {
	return sc.Request(fmt.Sprintf("/key/%s/%s", table, id), "POST", query)
}

// CreateAll calls the endpoint POST /key/:table, executing the statement
//
// CREATE type::thing($table, $id) CONTENT $body;
func (sc Client) CreateAll(table string, query string) (Response, error) {
	return sc.Request(fmt.Sprintf("/key/%s", table), "POST", query)
}

// SelectAll calls the endpoint GET /key/:table, executing the statement
//
// SELECT * FROM type::table($table);
func (sc Client) SelectAll(table string) (Response, error) {
	return sc.Request(fmt.Sprintf("/key/%s", table), "GET", "")
}

// SelectOne calls the endpoint GET /key/:table/:id, executing the statement
//
// SELECT * FROM type::thing(:table, :id);
func (sc Client) SelectOne(table string, id string) (Response, error) {
	return sc.Request(fmt.Sprintf("/key/%s/%s", table, id), "GET", "")
}

// ReplaceOne calls the endpoint PUT /key/:table/:id, executing the statement
//
// UPDATE type::thing($table, $id) CONTENT $body;
func (sc Client) ReplaceOne(table, id, query string) (Response, error) {
	return sc.Request(fmt.Sprintf("/key/%s/%s", table, id), "PUT", query)
}

// UpsertOne calls the endpoint PUT /key/:table/:id, executing the statement
//
// UPDATE type::thing($table, $id) MERGE $body;
func (sc Client) UpsertOne(table, id, query string) (Response, error) {
	return sc.Request(fmt.Sprintf("/key/%s/%s", table, id), "PATCH", query)
}

// DeleteOne calls the endpoint DELETE /key/:table/:id, executing the statement
//
// DELETE FROM type::thing($table, $id);
func (sc Client) DeleteOne(table, id string) (Response, error) {
	return sc.Request(fmt.Sprintf("/key/%s/%s", table, id), "DELETE", "")
}

// DeleteAll calls the endpoint DELETE /key/:table/, executing the statement
//
// DELETE FROM type::table($table);
func (sc Client) DeleteAll(table string) (Response, error) {
	return sc.Request(fmt.Sprintf("/key/%s", table), "DELETE", "")
}

// Request makes a request to surrealdb to the given endpoint, with the given data. Responses returned from
// surrealdb vary, and this function will only return the first response
// TODO: have it return the array, or some other data type that more properly reflects the responses
func (sc Client) Request(endpoint string, requestType string, body string) (Response, error) {
	client := &http.Client{}

	// TODO: verify its a valid requesttype
	req, err := http.NewRequest(requestType, sc.URL+endpoint, bytes.NewBufferString(body))
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("NS", sc.NS)
	req.Header.Set("DB", sc.DB)
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(sc.User, sc.Pass)

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
