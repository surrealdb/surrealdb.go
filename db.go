package surrealdb

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"

	"github.com/fxamacker/cbor/v2"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

type VersionData struct {
	Version   string `json:"version"`
	Build     string `json:"build"`
	Timestamp string `json:"timestamp"`
}

// DB is a client for the SurrealDB database that holds the connection.
type DB struct {
	ctx context.Context
	con connection.Connection
}

// New creates a new SurrealDB client.
func New(connectionURL string) (*DB, error) {
	u, err := url.ParseRequestURI(connectionURL)
	if err != nil {
		return nil, err
	}

	scheme := u.Scheme

	newParams := connection.NewConnectionParams{
		Marshaler:   models.CborMarshaler{},
		Unmarshaler: models.CborUnmarshaler{},
		BaseURL:     fmt.Sprintf("%s://%s", u.Scheme, u.Host),
		Logger:      logger.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	var con connection.Connection
	if scheme == "http" || scheme == "https" {
		con = connection.NewHTTPConnection(newParams)
	} else if scheme == "ws" || scheme == "wss" {
		con = connection.NewWebSocketConnection(newParams)
	} else if scheme == "memory" || scheme == "mem" || scheme == "surrealkv" {
		return nil, fmt.Errorf("embedded database not enabled")
		// con = connection.NewEmbeddedConnection(newParams)
	} else {
		return nil, fmt.Errorf("invalid connection url")
	}

	err = con.Connect()
	if err != nil {
		return nil, err
	}

	return &DB{con: con}, nil
}

// --------------------------------------------------
// Public methods
// --------------------------------------------------

// WithContext
func (db *DB) WithContext(ctx context.Context) *DB {
	db.ctx = ctx
	return db
}

// Close closes the underlying WebSocket connection.
func (db *DB) Close() error {
	return db.con.Close()
}

// Use is a method to select the namespace and table to use.
func (db *DB) Use(ns, database string) error {
	return db.con.Use(ns, database)
}

func (db *DB) Info() (map[string]interface{}, error) {
	var info connection.RPCResponse[map[string]interface{}]
	err := db.con.Send(&info, "info")
	return *info.Result, err
}

// SignUp is a helper method for signing up a new user.
func (db *DB) SignUp(authData *Auth) (string, error) {
	var token connection.RPCResponse[string]
	if err := db.con.Send(&token, "signup", authData); err != nil {
		return "", err
	}

	if err := db.con.Let(constants.AuthTokenKey, token.Result); err != nil {
		return "", err
	}

	return *token.Result, nil
}

// SignIn is a helper method for signing in a user.
func (db *DB) SignIn(authData *Auth) (string, error) {
	var token connection.RPCResponse[string]
	if err := db.con.Send(&token, "signin", authData); err != nil {
		return "", err
	}

	if err := db.con.Let(constants.AuthTokenKey, token.Result); err != nil {
		return "", err
	}

	return *token.Result, nil
}

func (db *DB) Invalidate() error {
	if err := db.con.Send(nil, "invalidate"); err != nil {
		return err
	}

	if err := db.con.Unset(constants.AuthTokenKey); err != nil {
		return err
	}

	return nil
}

func (db *DB) Authenticate(token string) error {
	if err := db.con.Send(nil, "authenticate", token); err != nil {
		return err
	}

	if err := db.con.Let(constants.AuthTokenKey, token); err != nil {
		return err
	}

	return nil
}

func (db *DB) Let(key string, val interface{}) error {
	return db.con.Let(key, val)
}

func (db *DB) Unset(key string) error {
	return db.con.Unset(key)
}

func (db *DB) Version() (*VersionData, error) {
	var ver connection.RPCResponse[VersionData]
	if err := db.con.Send(&ver, "version"); err != nil {
		return nil, err
	}
	return ver.Result, nil
}

func (db *DB) Send(res interface{}, method string, params ...interface{}) error {
	allowedSendMethods := []string{"select", "create", "insert", "update", "upsert", "patch", "delete", "query"}

	allowed := false
	for i := 0; i < len(allowedSendMethods); i++ {
		if strings.EqualFold(allowedSendMethods[i], strings.ToLower(method)) {
			allowed = true
			break
		}
	}

	if !allowed {
		return fmt.Errorf("provided method is not allowed")
	}

	return db.con.Send(&res, method, params...)
}

func (db *DB) LiveNotifications(liveQueryID string) (chan connection.Notification, error) {
	return db.con.LiveNotifications(liveQueryID)
}

//-------------------------------------------------------------------------------------------------------------------//

func Kill(db *DB, id string) error {
	return db.con.Send(nil, "kill", id)
}

func Live(db *DB, table models.Table, diff bool) (*models.UUID, error) {
	var res connection.RPCResponse[models.UUID]
	if err := db.con.Send(&res, "live", table, diff); err != nil {
		return nil, err
	}

	return res.Result, nil
}

func Query[TResult any](db *DB, sql string, vars map[string]interface{}) (*[]QueryResult[TResult], error) {
	var res connection.RPCResponse[[]QueryResult[TResult]]
	if err := db.con.Send(&res, "query", sql, vars); err != nil {
		return nil, err
	}

	return res.Result, nil
}

func Create[TResult any, TWhat TableOrRecord](db *DB, what TWhat, data interface{}) (*TResult, error) {
	var res connection.RPCResponse[TResult]
	if err := db.con.Send(&res, "create", what, data); err != nil {
		return nil, err
	}

	return res.Result, nil
}

func Select[TResult any, TWhat TableOrRecord](db *DB, what TWhat) (*TResult, error) {
	var res connection.RPCResponse[TResult]

	if err := db.con.Send(&res, "select", what); err != nil {
		return nil, err
	}

	return res.Result, nil
}

func Patch(db *DB, what interface{}, patches []PatchData) (*[]PatchData, error) {
	var patchRes connection.RPCResponse[[]PatchData]
	if err := db.con.Send(&patchRes, "patch", what, patches, true); err != nil {
		return nil, err
	}

	return patchRes.Result, nil
}

func Delete[TResult any, TWhat TableOrRecord](db *DB, what TWhat) (*TResult, error) {
	var res connection.RPCResponse[TResult]
	if err := db.con.Send(&res, "delete", what); err != nil {
		return nil, err
	}

	return res.Result, nil
}

func Upsert[TResult any, TWhat TableOrRecord](db *DB, what TWhat, data interface{}) (*TResult, error) {
	var res connection.RPCResponse[TResult]
	if err := db.con.Send(&res, "upsert", what, data); err != nil {
		return nil, err
	}

	return res.Result, nil
}

// Update a table or record in the database like a PUT request.
func Update[TResult any, TWhat TableOrRecord](db *DB, what TWhat, data interface{}) (*TResult, error) {
	var res connection.RPCResponse[TResult]
	if err := db.con.Send(&res, "update", what, data); err != nil {
		return nil, err
	}

	return res.Result, nil
}

// Merge a table or record in the database like a PATCH request.
func Merge[TResult any, TWhat TableOrRecord](db *DB, what TWhat, data interface{}) (*TResult, error) {
	var res connection.RPCResponse[TResult]
	if err := db.con.Send(&res, "merge", what, data); err != nil {
		return nil, err
	}

	return res.Result, nil
}

// Insert a table or a row from the database like a POST request.
func Insert[TResult any](db *DB, what models.Table, data interface{}) (*[]TResult, error) {
	var res connection.RPCResponse[[]TResult]
	if err := db.con.Send(&res, "insert", what, data); err != nil {
		return nil, err
	}

	return res.Result, nil
}

func Relate(db *DB, rel *Relationship) error {
	var res connection.RPCResponse[connection.ResponseID[models.RecordID]]
	if err := db.con.Send(&res, "relate", rel.In, rel.Relation, rel.Out, rel.Data); err != nil {
		return err
	}

	rel.ID = res.Result.ID
	return nil
}

func InsertRelation(db *DB, relationship *Relationship) error {
	var res connection.RPCResponse[[]connection.ResponseID[models.RecordID]]

	rel := map[string]any{
		"in":  relationship.In,
		"out": relationship.Out,
	}
	if relationship.ID != nil {
		rel["id"] = relationship.ID
	}
	for k, v := range relationship.Data {
		rel[k] = v
	}

	if err := db.con.Send(&res, "insert_relation", relationship.Relation, rel); err != nil {
		return err
	}

	relationship.ID = (*res.Result)[0].ID
	return nil
}

func QueryRaw(db *DB, queries *[]QueryStmt) error {
	preparedQuery := ""
	parameters := map[string]interface{}{}
	for i := 0; i < len(*queries); i++ {
		// append query
		preparedQuery += fmt.Sprintf("%s;", (*queries)[i].SQL)
		for k, v := range (*queries)[i].Vars {
			parameters[k] = v
		}
	}

	if preparedQuery == "" {
		return fmt.Errorf("no query to run")
	}

	var res connection.RPCResponse[[]QueryResult[cbor.RawMessage]]
	if err := db.con.Send(&res, "query", preparedQuery, parameters); err != nil {
		return err
	}

	for i := 0; i < len(*queries); i++ {
		// assign results
		(*queries)[i].Result = (*res.Result)[i]
		(*queries)[i].unmarshaler = db.con.GetUnmarshaler()
	}

	return nil
}
