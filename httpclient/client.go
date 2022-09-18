package httpclient

import (
	"bytes"
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

// Execute executes a POST with the given query, returning the raw JSON response as a string
// TODO: marshal the response into something nicer, depending on what the community decides
func (sc SurrealClient) Execute(query string) ([]byte, error) {
	return sc.Request("/sql", "POST", query)
}

func (sc SurrealClient) CreateOne(table, id, query string) ([]byte, error) {
	return sc.Request(fmt.Sprintf("/key/%s/%s", table, id), "POST", query)
}

func (sc SurrealClient) CreateAll(table string, query string) ([]byte, error) {
	return sc.Request(fmt.Sprintf("/key/%s", table), "POST", query)
}

func (sc SurrealClient) SelectAll(table string) ([]byte, error) {
	return sc.Request(fmt.Sprintf("/key/%s", table), "GET", "")
}

func (sc SurrealClient) SelectOne(table string, id string) ([]byte, error) {
	return sc.Request(fmt.Sprintf("/key/%s/%s", table, id), "GET", "")
}

func (sc SurrealClient) ReplaceOne(table, id, query string) ([]byte, error) {
	return sc.Request(fmt.Sprintf("/key/%s/%s", table, id), "PUT", query)
}

func (sc SurrealClient) UpsertOne(table, id, query string) ([]byte, error) {
	return sc.Request(fmt.Sprintf("/key/%s/%s", table, id), "PATCH", query)
}

func (sc SurrealClient) DeleteOne(table, id string) ([]byte, error) {
	return sc.Request(fmt.Sprintf("/key/%s/%s", table, id), "DELETE", "")
}

func (sc SurrealClient) DeleteAll(table string) ([]byte, error) {
	return sc.Request(fmt.Sprintf("/key/%s", table), "DELETE", "")
}

func (sc SurrealClient) Request(endpoint string, requestType string, body string) ([]byte, error) {
	client := &http.Client{}

	// TODO: verify its a valid requesttype
	req, err := http.NewRequest(requestType, sc.URL+endpoint, bytes.NewBufferString(body))
	if err != nil {
		return []byte{}, err
	}
	req.Header.Set("NS", sc.NS)
	req.Header.Set("DB", sc.DB)
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(sc.User, sc.Password)

	resp, err := client.Do(req)
	if err != nil {
		return []byte{}, err
	}
	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}
