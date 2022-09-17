package httpclient

import (
	"bytes"
	"io/ioutil"
	"net/http"
)

// SurrealClient is a wrapper to more easily make HTTP calls to the SurrealDB engine
type SurrealClient struct {
	// URL is the endpoint in SurrealDB to be called, needs to have /sql at the end
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

// RunQuery executes a POST with the given query, returning the raw JSON response as a string
// TODO: marshal the response into something nicer, depending on what the community decides
func (sc SurrealClient) RunQuery(query string) (string, error) {
	client := &http.Client{}

	req, err := http.NewRequest("POST", sc.URL, bytes.NewBufferString(query))
	if err != nil {
		return "", err
	}
	req.Header.Set("NS", sc.NS)
	req.Header.Set("DB", sc.DB)
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(sc.User, sc.Password)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	return string(data), err
}
