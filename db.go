package surrealdb

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"

	"github.com/fxamacker/cbor/v2"
	"github.com/gofrs/uuid"

	"github.com/surrealdb/surrealdb.go/internal/codec"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gorillaws"
	"github.com/surrealdb/surrealdb.go/pkg/connection/http"
	"github.com/surrealdb/surrealdb.go/pkg/connection/rpc"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// sendable is a constraint for types that can send RPC requests.
// It is satisfied by *DB, *Session, and *Transaction.
type sendable interface {
	*DB | *Session | *Transaction
}

// liveQueryable is a constraint for types that support live queries.
// Live queries are session-scoped and not supported within transactions.
type liveQueryable interface {
	*DB | *Session
}

type VersionData struct {
	Version   string `json:"version"`
	Build     string `json:"build"`
	Timestamp string `json:"timestamp"`
}

// DB is a client for the SurrealDB database that holds the connection.
type DB struct {
	con  connection.Connection
	opts options
	// renew keeps the token pair of the default session fresh. It is nil when
	// token renewal is turned off.
	renew *renewer

	// sessionRenewers are the renewers of the attached sessions, which Close
	// stops. It is a pointer so that DB stays comparable and holds no lock.
	sessionRenewers *renewerSet
}

// New creates a new SurrealDB client.
//
// Deprecated: New is deprecated. Use FromEndpointURLString instead.
func New(connectionURL string) (*DB, error) {
	return FromEndpointURLString(context.Background(), connectionURL)
}

// FromConnection creates a new SurrealDB client using the provided connection.
//
// Note that this function calls `conn.Connect(ctx)` for you,
// so you don't need to call it manually.
//
// To pass options, such as [WithVersionCheck], use [FromConnectionWithOptions].
func FromConnection(ctx context.Context, conn connection.Connection) (*DB, error) {
	return FromConnectionWithOptions(ctx, conn)
}

// FromConnectionWithOptions creates a new SurrealDB client using the provided
// connection, configured by opts. With no options it does what [FromConnection]
// does.
//
// With [WithVersionCheck](true), it then checks that the server's version is
// one this SDK supports, and fails with an [UnsupportedVersionError] if not. The
// check sends a version request, so it adds a round trip to every connect,
// which matters to code that connects for each request.
func FromConnectionWithOptions(ctx context.Context, conn connection.Connection, opts ...Option) (*DB, error) {
	o := applyOptions(opts)

	if err := conn.Connect(ctx); err != nil {
		return nil, err
	}

	db := &DB{con: conn, opts: o, sessionRenewers: newRenewerSet()}
	if o.tokenRenewal {
		db.renew = newRenewer(o.expiryMargin, func(ctx context.Context, tokens Tokens) (*Tokens, error) {
			return rpc.Refresh(conn, ctx, heldTokens(ctx, conn, tokens))
		})
	}

	if o.versionCheck {
		if err := db.checkVersion(ctx); err != nil {
			_ = conn.Close(ctx)
			return nil, err
		}
	}

	return db, nil
}

// tokenHolder is a connection which holds the session's token pair itself, and
// may replace it, as one from contrib/rews does when a reconnection refreshes
// an expired access token.
type tokenHolder interface {
	Tokens(ctx context.Context) (connection.Tokens, error)
}

// heldTokens returns the pair to renew: the one the connection holds, if it
// holds one with a refresh token, as it may have replaced tokens since the
// client recorded them, and tokens otherwise.
func heldTokens(ctx context.Context, conn connection.Connection, tokens Tokens) Tokens {
	holder, ok := conn.(tokenHolder)
	if !ok {
		return tokens
	}
	held, err := holder.Tokens(ctx)
	if err != nil || held.Refresh == "" {
		return tokens
	}
	return held
}

// Deprecated: Use FromEndpointURLString instead.
func Connect(ctx context.Context, connectionURL string) (*DB, error) {
	return FromEndpointURLString(ctx, connectionURL)
}

// FromEndpointURLString creates a new SurrealDB client and connects to the database.
//
// This function incurs a network call (currently HTTP request) to the SurrealDB server to check the health of the connection in
// case of HTTP, or to establish a WebSocket connection in case of WebSocket.
//
// The provided `ctx` is used to cancel the connection attempt if needed,
// so that you control how long you want to block in case the network is not reliable
// or any other issues like OS network stack issues/settings/etc.
//
// # Connection Engines
//
// There are 2 different connection engines you can use to connect to SurrealDb backend. You can do so via Websocket or through HTTP
// connections
//
// # Via WebSocket
//
// WebSocket is required when using live queries.
//
//	db, err := surrealdb.FromEndpointURLString(ctx, "ws://localhost:8000")
//
// or for a secure connection
//
//	db, err := surrealdb.FromEndpointURLString(ctx, "wss://localhost:8000")
//
// # Via HTTP
//
// There are some functions that are not available on RPC when using HTTP but on WebSocket.
//
// All these except the "live" endpoint are effectively implemented in the HTTP library and
// provides the same result as though it is natively available on HTTP.
//
//	db, err := surrealdb.FromEndpointURLString(ctx, "http://localhost:8000")
//
// or for a secure connection
//
//	db, err := surrealdb.FromEndpointURLString(ctx, "https://localhost:8000")
//
// To pass options, such as [WithVersionCheck], use [FromEndpointURLStringWithOptions].
func FromEndpointURLString(ctx context.Context, connectionURL string) (*DB, error) {
	return FromEndpointURLStringWithOptions(ctx, connectionURL)
}

// FromEndpointURLStringWithOptions creates a new SurrealDB client and connects
// to the database, configured by opts. With no options it does what
// [FromEndpointURLString] does.
//
//	db, err := surrealdb.FromEndpointURLStringWithOptions(ctx, "ws://localhost:8000",
//		surrealdb.WithVersionCheck(true))
func FromEndpointURLStringWithOptions(ctx context.Context, connectionURL string, opts ...Option) (*DB, error) {
	u, err := url.ParseRequestURI(connectionURL)
	if err != nil {
		return nil, err
	}

	conf := connection.NewConfig(u)

	if confErr := conf.Validate(); confErr != nil {
		return nil, fmt.Errorf("invalid connection config: %w", confErr)
	}

	var con connection.Connection

	switch conf.URL.Scheme {
	case "http", "https":
		con = http.New(conf)
	case "ws", "wss":
		con = gorillaws.New(conf)
	case "memory", "mem", "surrealkv":
		return nil, fmt.Errorf("embedded database not enabled")
	default:
		return nil, fmt.Errorf("invalid connection url")
	}

	return FromConnectionWithOptions(ctx, con, opts...)
}

// --------------------------------------------------
// Public methods
// --------------------------------------------------

// Deprecated: WithContext is deprecated and does nothing. Use context parameters in individual method calls instead.
func (db *DB) WithContext(ctx context.Context) *DB {
	return db
}

// Close closes the underlying WebSocket connection.
func (db *DB) Close(ctx context.Context) error {
	db.renew.stop()
	db.sessionRenewers.stopAll()
	return db.con.Close(ctx)
}

// trackSessionRenewer keeps a session's renewer, for Close to stop.
func (db *DB) trackSessionRenewer(r *renewer) {
	db.sessionRenewers.add(r)
}

// forgetSessionRenewer stops keeping a detached session's renewer.
func (db *DB) forgetSessionRenewer(r *renewer) {
	db.sessionRenewers.remove(r)
}

// Health checks that the server is up, with GET /health over HTTP. It works
// whichever transport the client uses, needs no authentication, and returns nil
// if the server answers with a 2xx status. If the server answers with another
// status, it returns a [*HealthError], which holds the status and the body.
func (db *DB) Health(ctx context.Context) error {
	hc, ok := db.con.(connection.HealthChecker)
	if !ok {
		return fmt.Errorf("%w: health", constants.ErrMethodNotAvailable)
	}
	return hc.Health(ctx)
}

// Use is a method to select the namespace and table to use.
func (db *DB) Use(ctx context.Context, ns, database string) error {
	return db.con.Use(ctx, ns, database)
}

func (db *DB) Info(ctx context.Context) (map[string]any, error) {
	var info connection.RPCResponse[map[string]any]
	err := connection.Send(db.con, ctx, &info, "info")
	if info.Result == nil {
		return nil, err
	}
	return *info.Result, err
}

// SignUp signs up a new user.
//
// The authData parameter can be either:
//   - An Auth struct
//   - A map[string]any with keys like: "namespace", "database", "scope", "user", "pass"
//
// Example with struct:
//
//	db.SignUp(Auth{
//	  Namespace: "app",
//	  Database: "app",
//	  Access: "user",
//	  Username: "yusuke",
//	  Password: "VerySecurePassword123!",
//	})
//
// Example with map:
//
//	db.SignUp(map[string]any{
//	  "NS": "app",
//	  "DB": "app",
//	  "AC": "user",
//	  "user": "yusuke",
//	  "pass": "VerySecurePassword123!",
//	})
func (db *DB) SignUp(ctx context.Context, authData any) (string, error) {
	if err := db.renew.lockAuth(ctx); err != nil {
		return "", err
	}
	defer db.renew.unlockAuth()

	token, err := db.con.SignUp(ctx, authData)
	if err == nil {
		db.renew.clear()
	}
	return token, err
}

// SignUpWithRefresh signs up a new user using a TYPE RECORD access method with WITH REFRESH enabled.
// This is only supported in SurrealDB v3+ and returns both an access token and a refresh token.
//
// The authData parameter should be a map[string]any with the signup credentials:
//
//	tokens, err := db.SignUpWithRefresh(ctx, map[string]any{
//	  "NS":   "app",
//	  "DB":   "app",
//	  "AC":   "user_access",
//	  "user": "yusuke",
//	  "pass": "VerySecurePassword123!",
//	})
//
// The returned Tokens contains:
//   - Access: JWT token (use with Authenticate() on new connections)
//   - Refresh: Refresh token (format: "surreal-refresh-...")
//
// Note: Use this method instead of SignUp when the access method has WITH REFRESH enabled.
// For access methods without WITH REFRESH, use SignUp instead.
func (db *DB) SignUpWithRefresh(ctx context.Context, authData any) (*Tokens, error) {
	if err := db.renew.lockAuth(ctx); err != nil {
		return nil, err
	}
	defer db.renew.unlockAuth()

	tokens, err := db.con.SignUpWithRefresh(ctx, authData)
	if err == nil {
		db.renew.set(tokens)
	}
	return tokens, err
}

// SignIn signs in an existing user.
//
// The authData parameter can be either:
//   - An Auth struct
//   - A map[string]any with keys like: "namespace", "database", "scope", "user", "pass"
//
// In either case, the username and the password are mandatory.
// Depending on whether namespace and database are provided or not,
// the user is signed in as a database-level user, a namespace-level user, or a root-level user.
//
// Moreover, the Access field in the Auth struct or the "AC" key in the map[string]any
// is optional, and is only needed when signing in as a record user, which is like
// a database user that requires the namespace and database to be specified too.
//
// The following examples illustrate the different cases.
//
// The most complex case is signing in as a record user, which requires
// specifying the Access field to indicate which access method to use for authentication.
//
//	db.SignIn(Auth{
//	  Access:    "user",
//	  Namespace: "app",
//	  Database:  "app",
//	  Username:  "yusuke",
//	  Password:  "VerySecurePassword123!",
//	})
//
// If namespace and database are provided, the user is signed in
// as a database-level user.
//
//	db.SignIn(Auth{
//	  Namespace: "app",
//	  Database: "app",
//	  Username: "yusuke",
//	  Password: "VerySecurePassword123!",
//	})
//
//	db.SignIn(map[string]any{
//	  "NS": "app",
//	  "DB": "app",
//	  "user": "yusuke",
//	  "pass": "VerySecurePassword123!",
//	})
//
// If namespace is provided but database is omitted, the user is signed in
// as a namespace-level user.
//
//	db.SignIn(Auth{
//	  Namespace: "app",
//	  Username: "yusuke",
//	  Password: "VerySecurePassword123!",
//	})
//
//	db.SignIn(map[string]any{
//	  "NS": "app",
//	  "user": "yusuke",
//	  "pass": "VerySecurePassword123!",
//	})
//
// If both namespace and database are omitted, the user is signed in
// as a root-level user.
//
//	db.SignIn(Auth{
//	  Username: "yusuke",
//	  Password: "VerySecurePassword123!",
//	})
//
//	db.SignIn(map[string]any{
//	  "user": "yusuke",
//	  "pass": "VerySecurePassword123!",
//	})
//
// # Bearer Access Method
//
// For TYPE BEARER access methods (SurrealDB v3+), use the "key" parameter with
// a bearer key obtained from ACCESS ... GRANT. Bearer keys have the format
// "surreal-bearer-...". No username/password is needed:
//
//	db.SignIn(map[string]any{
//	  "NS":  "app",
//	  "DB":  "app",
//	  "AC":  "bearer_api",
//	  "key": bearerKey,  // from ACCESS bearer_api GRANT FOR USER/RECORD ...
//	})
//
// Note: The "key" parameter is exclusively for bearer access grants.
// For TYPE RECORD access methods with WITH REFRESH, use SignInWithRefresh instead.
func (db *DB) SignIn(ctx context.Context, authData any) (string, error) {
	if err := db.renew.lockAuth(ctx); err != nil {
		return "", err
	}
	defer db.renew.unlockAuth()

	token, err := db.con.SignIn(ctx, authData)
	if err == nil {
		db.renew.clear()
	}
	return token, err
}

// Tokens contains the access token and refresh token returned by SignInWithRefresh.
// Access is the JWT token used for authentication.
// Use this with Authenticate() to establish a session on a new connection.
// Refresh is the refresh token used to obtain new tokens without credentials.
// Use this with SignInWithRefresh to get a new Tokens.
type Tokens = connection.Tokens

// SignInWithRefresh signs in using a TYPE RECORD access method with WITH REFRESH enabled.
// This is only supported in SurrealDB v3+ and returns both an access token and a refresh token.
//
// The authData parameter should be a map[string]any with the signin credentials:
//
//	// Initial signin with username/password
//	pair, err := db.SignInWithRefresh(ctx, map[string]any{
//	  "NS":   "app",
//	  "DB":   "app",
//	  "AC":   "user_access",
//	  "user": "yusuke",
//	  "pass": "VerySecurePassword123!",
//	})
//
// The returned Tokens contains:
//   - Access: JWT token (use with Authenticate() on new connections)
//   - Refresh: Refresh token (format: "surreal-refresh-...")
//
// To obtain new tokens using the refresh token (no credentials needed):
//
//	newPair, err := db.SignInWithRefresh(ctx, map[string]any{
//	  "NS":      "app",
//	  "DB":      "app",
//	  "AC":      "user_access",
//	  "refresh": pair.Refresh,  // no username/password needed
//	})
//
// Note: The "refresh" parameter is for record access refresh tokens only.
// For bearer access methods, use SignIn with the "key" parameter.
// For other access methods (system users, record users without refresh), use SignIn.
//
// With token renewal turned on by [WithTokenRenewal], the client then renews
// the pair before the access token expires; see [DB.Tokens].
//
// The server replaces a refresh token once it is used. On a connection from
// contrib/rews, a reconnection after the access token has expired signs in with
// the refresh token, so the one returned here then no longer works: read the new
// pair with rews.Connection.Tokens, or set rews.Connection.OnTokensRefreshed.
func (db *DB) SignInWithRefresh(ctx context.Context, authData any) (*Tokens, error) {
	if err := db.renew.lockAuth(ctx); err != nil {
		return nil, err
	}
	defer db.renew.unlockAuth()

	tokens, err := db.con.SignInWithRefresh(ctx, authData)
	if err == nil {
		db.renew.set(tokens)
	}
	return tokens, err
}

// Refresh exchanges a token pair, from SignInWithRefresh or SignUpWithRefresh,
// for a new one, and authenticates the session with it. The server revokes
// the refresh token it is given, so only the returned pair can be refreshed
// again. Refresh needs SurrealDB v3+.
//
// With token renewal turned on by [WithTokenRenewal], the client renews the
// pair it holds by itself, and Refresh is then mostly for pairs kept
// elsewhere, such as one stored between runs of a program.
func (db *DB) Refresh(ctx context.Context, tokens Tokens) (*Tokens, error) {
	if err := db.renew.lockAuth(ctx); err != nil {
		return nil, err
	}
	defer db.renew.unlockAuth()

	next, err := rpc.Refresh(db.con, ctx, tokens)
	if err == nil {
		db.renew.set(next)
	}
	return next, err
}

// Revoke revokes the refresh token of a token pair, so that it can no longer
// be used to obtain new tokens. It leaves the session signed in. Revoke needs
// SurrealDB v3+.
func (db *DB) Revoke(ctx context.Context, tokens Tokens) error {
	if err := db.renew.lockAuth(ctx); err != nil {
		return err
	}
	defer db.renew.unlockAuth()

	if err := rpc.Revoke(db.con, ctx, tokens); err != nil {
		return err
	}
	if current := db.renew.current(); current != nil && current.Refresh == tokens.Refresh {
		db.renew.clear()
	}
	return nil
}

// Tokens returns the token pair the session holds, which changes each time the
// client renews it, or nil when the session holds no pair or token renewal is
// off, as it is unless [WithTokenRenewal] turns it on.
//
// On a connection from contrib/rews, a reconnection after the access token has
// expired replaces the pair, which the client then renews. Until it does,
// Tokens returns the pair it replaced: read rews.Connection.Tokens instead.
func (db *DB) Tokens() *Tokens {
	return db.renew.current()
}

// Invalidate signs the session out.
func (db *DB) Invalidate(ctx context.Context) error {
	if err := db.renew.lockAuth(ctx); err != nil {
		return err
	}
	defer db.renew.unlockAuth()

	if err := db.con.Invalidate(ctx); err != nil {
		return err
	}
	db.renew.clear()
	return nil
}

// Reset returns the session to the state it was in when the connection was
// made: signed out, with no namespace, database or variables, and with its
// live queries killed. Over HTTP, where the client keeps that state itself,
// Reset clears it without calling the server.
func (db *DB) Reset(ctx context.Context) error {
	if err := db.renew.lockAuth(ctx); err != nil {
		return err
	}
	defer db.renew.unlockAuth()

	if err := rpc.Reset(db.con, ctx); err != nil {
		return err
	}
	db.renew.clear()
	return nil
}

// Authenticate authenticates the current connection with the provided token.
//
// This is mostly useful when you created a JWT authentication method on SurrealDB
// using `DEFINE ACCESS ... TYPE JWT` query, so that SurrealDB can verify the token
// provided via this method for authentication.
//
// After calling this method, all subsequent requests will be authenticated.
// How the authentication is maintained depends on the connection type:
//
//   - For WebSocket connections, the token is kept in the session on the server side.
//     This means connecting to the server again or creating a new connection to another server
//     will require calling this method again to authenticate.
//
//   - For HTTP connections, the token is sent with every request via the `Authorization` header.
//     This means that even if you create a new connection to another server,
//     as long as you call this method on the new connection, the requests will be authenticated.
func (db *DB) Authenticate(ctx context.Context, token string) error {
	if err := db.renew.lockAuth(ctx); err != nil {
		return err
	}
	defer db.renew.unlockAuth()

	if err := db.con.Authenticate(ctx, token); err != nil {
		return err
	}
	db.renew.clear()
	return nil
}

func (db *DB) Let(ctx context.Context, key string, val any) error {
	return db.con.Let(ctx, key, val)
}

func (db *DB) Unset(ctx context.Context, key string) error {
	return db.con.Unset(ctx, key)
}

func (db *DB) Version(ctx context.Context) (*VersionData, error) {
	ver, err := send[any](ctx, db, "version")
	if err != nil {
		return nil, err
	}
	if ver == nil {
		return nil, errors.New("unexpected version data: none")
	}

	switch v := (*ver).(type) {
	case map[string]any:
		ver, ok := v["version"].(string)
		if !ok {
			return nil, fmt.Errorf("unexpected version data: %v", v)
		}

		build, ok := v["build"].(string)
		if !ok {
			return nil, fmt.Errorf("unexpected build data: %v", v)
		}

		timestamp, ok := v["timestamp"].(string)
		if !ok {
			return nil, fmt.Errorf("unexpected timestamp data: %v", v)
		}
		return &VersionData{
			Version:   ver,
			Build:     build,
			Timestamp: timestamp,
		}, nil
	case string:
		ver := strings.TrimPrefix(v, "surrealdb-")
		return &VersionData{Version: ver}, nil
	default:
		return nil, fmt.Errorf("unexpected version data: %s (%T)", v, v)
	}
}

// Send sends a request to the SurrealDB server.
//
// It is a wrapper around [connection.Send], which is used by various RPC methods like
// [Query], [Insert] and so on.
//
// Compared to the original [connection.Send], [Send] is smarter about methods that are allowed to be sent.
// You usually want to use this function than using [connection.Send] directly.
//
// This function is limited to a selected set of RPC methods listed below:
//
// - select
// - create
// - insert
// - insert_relation
// - kill
// - live
// - merge
// - relate
// - update
// - upsert
// - patch
// - delete
// - query
//
// The `res` needs to be of type `*connection.RPCResponse[T]`.
//
// It returns an error in the following cases:
// - Error if the method is not allowed to be sent, which means that the request was not even sent.
// - Transport error like WebSocket message write timeout, connection closed, etc.
// - Unmarshal error if the response cannot be unmarshaled into the provided res parameter.
// - RPCError if the request was processed by SurrealDB but it failed there.
func Send[Result any](ctx context.Context, db *DB, res *connection.RPCResponse[Result], method string, params ...any) error {
	allowedSendMethods := []string{
		"select", "create", "insert", "insert_relation",
		"kill", "live", "merge", "relate", "update", "upsert",
		"patch", "delete", "query",
	}

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

	return connection.Send(db.con, ctx, res, method, params...)
}

func (db *DB) LiveNotifications(liveQueryID string) (chan connection.Notification, error) {
	return db.con.LiveNotifications(liveQueryID)
}

func (db *DB) CloseLiveNotifications(liveQueryID string) error {
	return db.con.CloseLiveNotifications(liveQueryID)
}

//-------------------------------------------------------------------------------------------------------------------//

// Kill terminates a live query and closes the notification channel.
// S can be *DB or *Session (not *Transaction, as live queries are session-scoped).
func Kill[S liveQueryable](ctx context.Context, s S, id string) error {
	// First kill the live query on the server
	var liveID any = id
	if u, err := uuid.FromString(id); err == nil {
		liveID = models.UUID{UUID: u}
	}
	if _, err := queryOne[any](ctx, s, "KILL $id", map[string]any{"id": liveID}); err != nil {
		return err
	}

	// Then close the notification channel to prevent leaks
	switch v := any(s).(type) {
	case *DB:
		return v.CloseLiveNotifications(id)
	case *Session:
		return v.CloseLiveNotifications(id)
	}
	return nil
}

// Live starts a live query on a table.
// S can be *DB or *Session (not *Transaction, as live queries are session-scoped).
//
// With diff, notifications carry the JSON Patch operations that changed each
// record rather than the record.
func Live[S liveQueryable](ctx context.Context, s S, table models.Table, diff bool) (*models.UUID, error) {
	fields := "*"
	if diff {
		fields = "DIFF"
	}
	return queryOne[models.UUID](ctx, s, "LIVE SELECT "+fields+" FROM $what", map[string]any{"what": table})
}

// Query executes a query against the SurrealDB database.
//
// S can be *DB, *Session, or *Transaction.
//
// [Query] supports:
//
//   - Full SurrealQL syntax including transactions
//   - Parameterized queries for security
//   - Typed results with generics
//   - Multiple statements in a single call
//
// It takes a SurrealQL query to be executed, and the variables to parameterize the query,
// and returns a slice of [QueryResult] whose type parameter is the result type.
//
// # Examples
//
// Execute a SurrealQL query with typed results:
//
//	results, err := surrealdb.Query[[]Person](
//	  context.Background(),
//	  db,
//	  "SELECT * FROM persons WHERE age > $minAge",
//	  map[string]any{
//	      "minAge": 18,
//	  },
//	)
//
// You can also use Query for transactions with variables:
//
//	transactionResults, err := surrealdb.Query[[]any](
//	  context.Background(),
//	  db,
//	  `
//	  BEGIN TRANSACTION;
//	  CREATE person:$johnId SET name = $johnName, age = $johnAge;
//	  CREATE person:$janeId SET name = $janeName, age = $janeAge;
//	  COMMIT TRANSACTION;
//	  `,
//	  map[string]any{
//	      "johnId": "john",
//	      "johnName": "John",
//	      "johnAge": 30,
//	      "janeId": "jane",
//	      "janeName": "Jane",
//	      "janeAge": 25,
//	  },
//	)
//
// Or use a single CREATE with content variable:
//
//	createResult, err := surrealdb.Query[[]Person](
//	    context.Background(),
//	    db,
//	    "CREATE person:$id CONTENT $content",
//	    map[string]any{
//			"id": "alice",
//			"content": map[string]any{
//				"name": "Alice",
//				"age": 28,
//				"city": "New York",
//			},
//		},
//	)
//
// # Handling errors
//
// If the query fails, the returned error will be a `joinError` created by the [errors.Join] function,
// which contains all the errors that occurred during the query execution.
// The caller can check the Error field of each [QueryResult] to see if the query failed,
// or check the returned error from the [Query] function to see if the query failed.
//
// If the caller wants to handle the query errors, if any, it can check the Error field of each [QueryResult],
// or call:
//
//	errors.Is(err, &surrealdb.QueryError{})
//
// on the returned error to see if it is (or contains) a [QueryError].
//
// # Query errors are non-retriable
//
// If the error is a [QueryError], the caller should NOT retry the query,
// because the query is already executed and the error is not recoverable,
// and often times the error is caused by a bug in the query itself.
//
// # When can you safely retry the query when this function returns an error?
//
// Generally speaking, automatic retries make sense only when the error is transient,
// such as a network error, a timeout, or a server error that is not related to the query itself.
// In such cases, the caller can retry the query by calling the [Query] function again.
//
// For this function, the caller may retry when the error is:
//   - [RPCError]: because we should get a RPC error only when the RPC failed due to anything other than the query error
//   - [constants.ErrTimeout]: This means we send the HTTP request or a WebSocket message to SurrealDB in timely manner,
//     which is often due to temporary network issues or server overload.
//
// # What non-retriable errors will Query return?
//
// However, if the error is any of the following, the caller should NOT retry the query:
//   - [QueryError]: This means the query failed due to a syntax error, a type error, or a logical error in the query itself.
//   - Unmarshal error: This means the response from the server could not be unmarshaled into the expected type,
//     which is often due to a bug in the code or a mismatch between the expected type and the actual response type.
//   - Marshal error: This means the request could not be marshaled using CBOR,
//     which is often due to a bug in the code that tries to send something that cannot be marshaled or understood by
//     SurrealDB, such as a struct with unsupported types.
//   - Anything else: It's just safer to not retry when we aren't sure if the error is whether transient or permanent.
//
// # RPCError is retriable only for Query
//
// Note that [RPCError] is retriable only for the [Query] RPC method,
// because in other cases, the [RPCError] may also indicate a query error.
// For example, if you tried to insert a duplicate record using the [Insert] RPC,
// you may get an [RPCError] saying so, which is not retriable.
//
// If you tried to insert the same duplicate record using the [Query] RPC method with `INSERT` statement,
// you may get no [RPCError], but a [QueryError] saying so, enabling you to easily diferentiate
// between retriable and non-retriable errors.
func Query[TResult any, S sendable](ctx context.Context, s S, sql string, vars map[string]any) (*[]QueryResult[TResult], error) {
	return queryWith[TResult](ctx, s, "query", sql, vars)
}

// queryWith sends a query with the query or gql method, whose results have the same form.
func queryWith[TResult any, S sendable](
	ctx context.Context, s S, method, sql string, vars map[string]any,
) (*[]QueryResult[TResult], error) {
	res, err := send[[]QueryResult[cbor.RawMessage]](ctx, s, method, sql, vars)
	if err != nil {
		return nil, err
	}

	// The query errors, if any
	var (
		errs error
	)

	// Get the unmarshaler based on the sender type
	var unmarshaler func([]byte, any) error
	switch v := any(s).(type) {
	case *DB:
		unmarshaler = v.con.GetUnmarshaler().Unmarshal
	case *Session:
		unmarshaler = v.db.con.GetUnmarshaler().Unmarshal
	case *Transaction:
		unmarshaler = v.db.con.GetUnmarshaler().Unmarshal
	}

	// We unmarshal []QueryResult[cbor.RawMessage] first,
	// and then unmarshal each cbor.RawMessage to TResult.
	// This is necessary because the Result field can be a string in case Status is "ERR".
	// In that case if we directly unmarshaled to TResult using []QueryResult[TResult],
	// it would fail with "cannot unmarshal UTF-8 text string into Go struct field"
	// because CBOR string cannot be unmarshaled into TResult except when TResult is a string.
	qr := make([]QueryResult[TResult], len(*res))

	for i, result := range *res {
		var (
			r TResult
			e *QueryError
		)
		if result.Status == statusERR {
			var errMsg string
			if result.Result != nil {
				if err := unmarshaler(result.Result, &errMsg); err != nil {
					return nil, fmt.Errorf("failed to unmarshal error message: %w", err)
				}
			}
			e = &QueryError{
				Message: errMsg,
			}
			errs = errors.Join(errs, e)
		} else if result.Result != nil {
			if err := unmarshaler(result.Result, &r); err != nil {
				return nil, fmt.Errorf("failed to unmarshal result: %w", err)
			}
		}
		qr[i] = QueryResult[TResult]{
			Status: result.Status,
			Time:   result.Time,
			Result: r,
			Error:  e,
		}
	}

	return &qr, errs
}

// QueryRaw composes a query from the provided QueryStmt objects,
// and execute it using the query RPC method.
// S can be *DB, *Session, or *Transaction.
//
// Each QueryStmt must hold exactly one statement, so that the i-th result can be
// given to the i-th QueryStmt. QueryRaw follows each statement with a marker, a
// statement returning a random string, so that the results show where the results
// of each QueryStmt end. If a QueryStmt yields several results or none, as it does
// when it holds several statements or only a comment, QueryRaw returns an error and
// leaves the results unchanged.
//
// A transaction written as statements (BEGIN, ..., COMMIT or CANCEL, each a
// QueryStmt of its own) works too:
//   - Once a transaction has failed, the server runs neither the statements nor the
//     markers in it, and returns an error in place of each. QueryRaw gives each
//     QueryStmt the error returned in its place.
//   - A BEGIN, COMMIT or CANCEL for which the server returns no result, as SurrealDB
//     2.x does for all three and 3.x does for a COMMIT or CANCEL after a failure, is
//     given a NONE result, or an error if it is a COMMIT after a failure.
//   - A RETURN directly before the COMMIT or CANCEL works. SurrealDB 3.0 to 3.3 skip
//     the rest of a transaction after a RETURN in it, so other statements after a
//     RETURN in a transaction make QueryRaw return an error.
//
// If a statement fails in a way that leaves results out, such as a transaction that
// times out, QueryRaw returns an error which includes the server's error.
//
// The variables of all the statements are sent together, so two statements must
// not give one name different values.
//
// You may want to use [Query] with [github.com/surrealdb/surrealdb.go/contrib/surrealql] instead.
func QueryRaw[S sendable](ctx context.Context, s S, queries *[]QueryStmt) error {
	if queries == nil || len(*queries) == 0 {
		return fmt.Errorf("no query to run")
	}

	marker, err := newRawQueryMarker()
	if err != nil {
		return err
	}

	preparedQuery, parameters, err := composeRawQuery(*queries, marker)
	if err != nil {
		return err
	}
	kinds := make([]rawStmtKind, len(*queries))
	for i := range *queries {
		kinds[i] = rawStmtKindOf((*queries)[i].SQL)
	}

	res, err := send[[]QueryResult[cbor.RawMessage]](ctx, s, "query", preparedQuery, parameters)
	if err != nil {
		return err
	}

	unmarshaler := unmarshalerOf(s)
	results, err := splitRawResults(res, kinds, marker, unmarshaler)
	if err != nil {
		return err
	}

	for i := range *queries {
		// assign results
		(*queries)[i].Result = results[i]
		(*queries)[i].unmarshaler = unmarshaler
	}

	return nil
}

// newRawQueryMarker returns a random string, which no statement of QueryRaw can be
// expected to return, to mark the end of the results of each statement.
func newRawQueryMarker() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("QueryRaw: cannot make a marker: %w", err)
	}
	return "QueryRaw-" + hex.EncodeToString(b), nil
}

// composeRawQuery joins the statements of QueryRaw into one query, following each
// with a statement which returns the marker, and their variables into one map.
func composeRawQuery(queries []QueryStmt, marker string) (sql string, vars map[string]any, err error) {
	var preparedQuery strings.Builder
	parameters := map[string]any{}
	for i := range queries {
		stmt := trimRawStmt(queries[i].SQL)
		if stmt == "" {
			return "", nil, fmt.Errorf("QueryRaw: statement %d is empty", i)
		}
		preparedQuery.WriteString(stmt)
		// The newline ends a trailing line comment, which would otherwise swallow the semicolon.
		preparedQuery.WriteString("\n;\n'" + marker + "';\n")

		for k, v := range queries[i].Vars {
			if prev, ok := parameters[k]; ok && !sameRawVar(prev, v) {
				return "", nil, fmt.Errorf("QueryRaw: variable $%s is given different values by different statements (%T and %T)", k, prev, v)
			}
			parameters[k] = v
		}
	}

	return preparedQuery.String(), parameters, nil
}

// trimRawStmt returns sql without surrounding white space and trailing semicolons.
func trimRawStmt(sql string) string {
	return strings.TrimRight(strings.TrimSpace(sql), "; \t\r\n")
}

// sameRawVar reports whether a and b are the same value to the server: equal Go
// values, or values with the same CBOR encoding, such as 1 and int64(1).
func sameRawVar(a, b any) bool {
	if reflect.DeepEqual(a, b) {
		return true
	}
	ea, err := cbor.Marshal(a)
	if err != nil {
		return false
	}
	eb, err := cbor.Marshal(b)
	return err == nil && bytes.Equal(ea, eb)
}

// rawStmtKind tells the statements that begin and end a transaction from others,
// as the server may return no result for them.
type rawStmtKind int

const (
	rawOther rawStmtKind = iota
	rawBegin
	rawCommit
	rawCancel
)

// rawStmtKindOf returns the kind of the statement sql.
func rawStmtKindOf(sql string) rawStmtKind {
	switch strings.ToUpper(strings.Join(strings.Fields(trimRawStmt(sql)), " ")) {
	case "BEGIN", "BEGIN TRANSACTION":
		return rawBegin
	case "COMMIT", "COMMIT TRANSACTION":
		return rawCommit
	case "CANCEL", "CANCEL TRANSACTION":
		return rawCancel
	}
	return rawOther
}

// The errors SurrealDB returns in place of the result of a statement which it did not
// run because the transaction it is in had failed.
const (
	errFailedTransaction    = "The query was not executed due to a failed transaction"
	errCancelledTransaction = "The query was not executed due to a cancelled transaction" //nolint:misspell // the server's spelling
	// errAbortedCommit is what SurrealDB 3.3 and later return for a COMMIT after a failure.
	errAbortedCommit = "Cannot COMMIT: the transaction was aborted due to a prior error"
)

// The statuses of a QueryResult.
const (
	statusOK  = "OK"
	statusERR = "ERR"
)

// splitRawResults returns the result of each statement of a query composed by
// composeRawQuery, given the kind of each. Each statement must have yielded exactly
// one result, followed by the marker after it, or by the error the server returns
// in place of a marker it did not run because its transaction had failed. The
// exceptions are those described for QueryRaw. Otherwise it is unknown which result
// belongs to which statement, and splitRawResults returns an error.
func splitRawResults(
	res *[]QueryResult[cbor.RawMessage], kinds []rawStmtKind, marker string, unmarshaler codec.Unmarshaler,
) ([]QueryResult[cbor.RawMessage], error) {
	m := rawResults{marker: marker, unmarshaler: unmarshaler}
	if res != nil {
		m.all = *res
	}
	rest := m.all
	results := make([]QueryResult[cbor.RawMessage], len(kinds))
	for i := 0; i < len(kinds); {
		rows, stmts := m.match(kinds, i, rest, results)
		if stmts == 0 {
			return nil, m.unmatched(i)
		}
		rest = rest[rows:]
		i += stmts
	}
	if len(rest) != 0 {
		return nil, fmt.Errorf("QueryRaw: the server returned %d results more than expected", len(rest))
	}

	return results, nil
}

// rawResults are the results of a query composed by composeRawQuery.
type rawResults struct {
	all         []QueryResult[cbor.RawMessage]
	marker      string
	unmarshaler codec.Unmarshaler
}

// isText reports whether r has the status and the text result given.
func (m *rawResults) isText(r QueryResult[cbor.RawMessage], status, text string) bool {
	// A text string encodes in at most 9 bytes more than its length, so a longer
	// result is not worth decoding.
	if r.Status != status || len(r.Result) > len(text)+9 {
		return false
	}
	var s string
	return m.unmarshaler.Unmarshal(r.Result, &s) == nil && s == text
}

func (m *rawResults) isMarker(r QueryResult[cbor.RawMessage]) bool {
	return m.isText(r, statusOK, m.marker)
}

// isSkipped reports whether r is the error in place of a statement not run because
// its transaction had failed.
func (m *rawResults) isSkipped(r QueryResult[cbor.RawMessage]) bool {
	return m.isText(r, statusERR, errFailedTransaction) || m.isText(r, statusERR, errCancelledTransaction)
}

// isEnd reports whether r is the marker after a statement, or the error in its place.
func (m *rawResults) isEnd(r QueryResult[cbor.RawMessage]) bool {
	return m.isMarker(r) || m.isSkipped(r)
}

// match gives results to the statements from statement i on, from the results
// rest, and returns how many results it used and for how many statements. It
// returns 0 statements if the results do not match.
func (m *rawResults) match(
	kinds []rawStmtKind, i int, rest []QueryResult[cbor.RawMessage], results []QueryResult[cbor.RawMessage],
) (rows, stmts int) {
	if m.noResult(kinds[i], rest) {
		results[i] = noRawResult(kinds[i], i > 0 && results[i-1].Status == statusERR)
		return 1, 1
	}
	if len(rest) >= 2 && !m.isMarker(rest[0]) && m.isEnd(rest[1]) {
		results[i] = rest[0]
		return 2, 1
	}
	// SurrealDB 3.0 to 3.3 skip the rest of a transaction after a RETURN in it,
	// including the marker after the RETURN, up to the COMMIT or CANCEL.
	if i+1 < len(kinds) && isRawTxnEnd(kinds[i+1]) &&
		len(rest) >= 3 && !m.isMarker(rest[0]) && !m.isMarker(rest[1]) && m.isEnd(rest[2]) {
		results[i], results[i+1] = rest[0], rest[1]
		return 3, 2
	}
	return 0, 0
}

// noResult reports whether the server returned no result for a statement of the
// kind given, whose results would begin with rest.
func (m *rawResults) noResult(kind rawStmtKind, rest []QueryResult[cbor.RawMessage]) bool {
	switch {
	case len(rest) == 0:
		return false
	// SurrealDB 2.x returns no result for BEGIN; 3.x returns NONE, never an error.
	case kind == rawBegin:
		return m.isEnd(rest[0])
	// SurrealDB 2.x returns no result for COMMIT and CANCEL, and 3.x none for a
	// CANCEL, or for a COMMIT before 3.3, after a failure. The marker after them
	// runs outside the transaction.
	case isRawTxnEnd(kind):
		return m.isMarker(rest[0])
	}
	return false
}

// isRawTxnEnd reports whether kind ends a transaction.
func isRawTxnEnd(kind rawStmtKind) bool {
	return kind == rawCommit || kind == rawCancel
}

// unmatched returns the error for results which cannot be matched to the statements
// from statement i on. It names the first error the server returned, other than
// those for statements it skipped, as that is likely the cause.
func (m *rawResults) unmatched(i int) error {
	for _, r := range m.all {
		if r.Status != statusERR || m.isSkipped(r) {
			continue
		}
		var msg string
		if m.unmarshaler.Unmarshal(r.Result, &msg) != nil {
			msg = "an error"
		}
		return fmt.Errorf("QueryRaw: the results cannot be matched to the statements from statement %d on, "+
			"after the server returned an error: %s", i, msg)
	}
	return fmt.Errorf("QueryRaw: statement %d did not yield exactly one result; "+
		"each QueryStmt must hold exactly one statement", i)
}

// noRawResult returns the result given to a BEGIN, COMMIT or CANCEL for which the
// server returned none: NONE, or for a COMMIT after a failure, the error SurrealDB
// 3.3 and later return for it.
func noRawResult(kind rawStmtKind, failed bool) QueryResult[cbor.RawMessage] {
	if kind == rawCommit && failed {
		msg, _ := cbor.Marshal(errAbortedCommit)
		return QueryResult[cbor.RawMessage]{Status: statusERR, Result: msg}
	}
	return QueryResult[cbor.RawMessage]{Status: statusOK, Result: cbor.RawMessage{0xf6}}
}

// unmarshalerOf returns the unmarshaler of the connection s sends on.
func unmarshalerOf[S sendable](s S) codec.Unmarshaler {
	switch v := any(s).(type) {
	case *DB:
		return v.con.GetUnmarshaler()
	case *Session:
		return v.db.con.GetUnmarshaler()
	case *Transaction:
		return v.db.con.GetUnmarshaler()
	}
	return nil
}

// send is a helper function to send a request to the SurrealDB server
// in case the expected response is a connection.RPCResponse[TResult].
// If one expects other types of responses, use db.con.Send directly.
//
// The function uses a type switch to determine how to send the request:
//   - *DB: uses connection.Send (no session/txn context)
//   - *Session: uses connection.Call with session UUID
//   - *Transaction: uses connection.Call with session and txn UUIDs
func send[TResult any, S sendable](ctx context.Context, s S, method string, params ...any) (*TResult, error) {
	var res connection.RPCResponse[TResult]

	switch v := any(s).(type) {
	case *DB:
		if err := connection.Send(v.con, ctx, &res, method, params...); err != nil {
			return nil, err
		}
	case *Session:
		if v.isClosed() {
			return nil, constants.ErrSessionClosed
		}
		req := &connection.RPCRequest{
			Method:  method,
			Params:  params,
			Session: v.id,
		}
		if err := connection.Call(v.db.con, ctx, &res, req); err != nil {
			return nil, err
		}
	case *Transaction:
		if v.IsClosed() {
			return nil, constants.ErrTransactionClosed
		}
		req := &connection.RPCRequest{
			Method:  method,
			Params:  params,
			Session: v.sessionID,
			Txn:     v.id,
		}
		if err := connection.Call(v.db.con, ctx, &res, req); err != nil {
			return nil, err
		}
	}

	return res.Result, nil
}
