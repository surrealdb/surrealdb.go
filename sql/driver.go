package sql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"fmt"
	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/internal/websocket"
	"net/http"
	"net/url"
	"time"
)

const DriverName = "surrealdb"

func init() {
	sql.Register(DriverName, &Driver{})
}

type Driver struct {
	URL       string
	Username  string
	Password  string
	Namespace string
	Database  string
}

// Open establishes a new connection to SurrealDB.
func (s *Driver) Open(name string) (driver.Conn, error) {
	connector, err := s.OpenConnector(name)
	if err != nil {
		return nil, err
	}

	return connector.Connect(context.Background())
}

// OpenConnector must parse the name in the same format that Driver.Open
// parses the name parameter.
func (s *Driver) OpenConnector(name string) (driver.Connector, error) {
	u, err := url.Parse(name)
	if err != nil {
		return nil, fmt.Errorf("unsupported value: %s - %w", name, err)
	}

	configured := new(Driver)

	// TODO: also read this from ENV ?

	if u.User != nil {
		configured.Username = u.User.Username()
		configured.Password, _ = u.User.Password()

		// Clear it, because the WebSocket library doesn't like auth in URL
		u.User = nil
	}

	// Attempt to find database/namespace values
	queryValues := u.Query()
	configured.Namespace = queryValues.Get("ns")
	configured.Database = queryValues.Get("db")

	// Overwrite the path, because we connect via WebSockets
	u.Path = "/rpc"
	u.RawQuery = ""

	configured.URL = u.String()

	return configured, nil
}

// Connect establishes a new connection to SurrealDB.
func (s *Driver) Connect(ctx context.Context) (driver.Conn, error) {
	header := make(http.Header)
	if s.Username != "" {
		encoded := base64.StdEncoding.EncodeToString(append(append([]byte(s.Username), byte(':')), []byte(s.Password)...))
		header.Add("Authorization", "Basic "+encoded)
	}
	if s.Namespace != "" {
		header.Add("NS", s.Namespace)
	}
	if s.Database != "" {
		header.Add("DB", s.Database)
	}

	// TODO: use the ctx to allow for cancels
	var opts = []websocket.Option{
		websocket.Timeout(websocket.DefaultTimeout),
	}

	deadline, ok := ctx.Deadline()
	if ok {
		timeout := deadline.Sub(time.Now())
		opts = append(opts, websocket.Timeout(timeout.Seconds()))
	}

	ws, err := websocket.NewWebsocketWithOptions(s.URL, header, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to connect: %w (also check your credentials)", err)
	}

	db, err := surrealdb.NewWithConnection(ws)
	if err != nil {
		return nil, err
	}

	return &Conn{
		DB: db,
	}, nil
}

func (s *Driver) Driver() driver.Driver {
	return s
}

func (s *Driver) CheckNamedValue(value *driver.NamedValue) error {
	fmt.Println("value", value)
	return nil
}
