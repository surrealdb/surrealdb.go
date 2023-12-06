package surrealdb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	rawslog "log/slog"
	"net"
	"os/exec"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/surrealdb/surrealdb.go/pkg/conn/gorilla"
	"github.com/surrealdb/surrealdb.go/pkg/logger/slog"
)

var (
	testHasSurrealCLI     bool
	checkSurrealCLIOnce   sync.Once
	DelayAfterServerStart time.Duration = 300 * time.Millisecond
	DelayBeforeServerExit time.Duration = 300 * time.Millisecond
)

func Test_NewTestSurrealDB_Simple(t *testing.T) {
	// Simple illustration of how NewTestSurrealDB can be used.

	// Tests can run in parallel.
	t.Parallel()

	// NewTestSurrealDB returns endpoint (random open port used), DB instance
	// (which is wrapped in DBForTest to provide extra methods), and a func to
	// shutdown the SurrealDB server.
	endpoint, db, close := NewTestSurrealDB(t)
	defer close()
	_ = endpoint // endpoint could be useful for some low level testing.

	// DBForTest.Prepare is to run any SurrealQL for test prep. This would fail
	// early if the provided string results in an error from SurrealDB server.
	db.Prepare(t, "CREATE user:x SET name = 'x'")

	// More interactions can simply use db instance from here on.
}

func Test_TestSurrealDBParallelSimple(t *testing.T) {
	t.Parallel()
	instanceCount := 20
	for i := 0; i < instanceCount; i++ {
		t.Run(fmt.Sprintf("parallel__%d", i), func(t *testing.T) {
			t.Parallel()

			_, db, close := NewTestSurrealDB(t)
			defer close()

			// Ensure that CREATE statement does not run against the same
			// database (which would fail).
			db.Prepare(t, "CREATE user:x SET name = 'x';")
		})
	}
}

func Test_TestSurrealDBParalleSelect(t *testing.T) {
	t.Parallel()
	instanceCount := 20
	for i := 0; i < instanceCount; i++ {
		i := i
		t.Run(fmt.Sprintf("parallel__%d", i), func(t *testing.T) {
			t.Parallel()
			_, db, close := NewTestSurrealDB(t)
			defer close()

			// Ensure that CREATE statement does not run against the same
			// database (which would fail).
			db.Prepare(t, fmt.Sprintf("CREATE user:x SET name = 'x', index = %d;", i))

			// Query the saved data.
			data, err := db.Select("user")
			if err != nil {
				t.Fatalf("failed to select: %v", err)
			}
			jsonBytes, err := json.Marshal(data)
			if err != nil {
				t.Fatalf("failed to marshal to json: %v", err)
			}
			// Note the Index field, which would change based on the for loop.
			type dataholder struct {
				ID    string `json:"id,omitempty"`
				Name  string `json:"name,omitempty"`
				Index int    `json:"index,omitempty"`
			}
			d := []dataholder{}
			err = json.Unmarshal(jsonBytes, &d)
			if err != nil {
				t.Fatalf("failed to unmarshal data: %v", err)
			}
			// Simply check all fields match as expected.
			want := dataholder{ID: "user:x", Name: "x", Index: i}
			if d[0].ID != want.ID || d[0].Name != want.Name || d[0].Index != want.Index {
				t.Fatalf("data mismatch:\n    want: %+v, got:  %+v", want, d)
			}
		})
	}
}

type DBForTest struct {
	*DB
}

func NewTestSurrealDB(t testing.TB) (string, *DBForTest, func()) {
	t.Helper()

	checkSurrealCLIOnce.Do(func() {
		if _, err := exec.LookPath("surreal"); err == nil {
			testHasSurrealCLI = true
		}
	})
	if !testHasSurrealCLI {
		t.Skip("surreal CLI not found, skipping")
	}

	port := getFreePort(t)
	srvEndpoint := fmt.Sprintf("0.0.0.0:%d", port)

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "surreal",
		"start",
		"--auth",
		"--user", "root",
		"--pass", "surrealdb",
		"--bind", srvEndpoint,
		"memory")
	// Ensure to wait for a short while before context cancellation to
	// propagate, so that the client can shut down before.
	// NOTE: There is currently a race condition and potential panic due to the
	// closed websocket being used in a tight loop.
	cmd.WaitDelay = DelayBeforeServerExit

	go func() {
		err := cmd.Run()
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				t.Logf("SurrealDB server shut down by context")
				return
			}
			exit := &exec.ExitError{}
			if errors.As(err, &exit) {
				t.Errorf("SurrealDB server exited with error: %v", err)
				return
			}
			t.Errorf("Command failed to run: %v", err)
		}
	}()

	clientEndpoint := fmt.Sprintf("localhost:%d", port)

	// Give some time for the server to start up.
	time.Sleep(DelayAfterServerStart)
	db := newDBForTest(t, clientEndpoint)
	login := &Auth{
		Username:  "root",
		Password:  "surrealdb",
		Namespace: "dummy",
		Database:  "dummy",
	}
	if _, err := db.Signin(login); err != nil {
		t.Fatalf("failed to connect using '%+v' for signin: %v", login, err)
	}

	// Client can potentially stay around, so ensure to close connection after
	// the test is complete.
	t.Cleanup(func() {
		db.conn.Close()
	})

	return clientEndpoint, &DBForTest{db}, cancel
}

func getFreePort(t testing.TB) uint32 {
	t.Helper()

	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("could not find any open port: %v", err)
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		t.Fatalf("could not listen to \"%s\": %v", addr, err)
	}
	defer l.Close()
	return uint32(l.Addr().(*net.TCPAddr).Port)
}

func newDBForTest(t testing.TB, endpoint string) *DB {
	t.Helper()

	url := fmt.Sprintf("ws://%s/rpc", endpoint)
	buff := bytes.NewBuffer([]byte{})
	handler := rawslog.NewJSONHandler(buff, &rawslog.HandlerOptions{Level: rawslog.LevelDebug})
	log := slog.New(handler)
	ws, err := gorilla.Create().SetTimeOut(5 * time.Second).SetCompression(true).Logger(log).Connect(url)
	if err != nil {
		t.Fatalf("failed to create websocket connection: %v", err)
	}
	db, err := New(url, ws)
	if err != nil {
		t.Errorf("failed to establish connection to \"%v\": %v", url, err)
	}
	return db
}

func (d *DBForTest) Prepare(t testing.TB, schema string) {
	t.Helper()

	// Use SurrealDB's raw query support. This expects no complex string
	// manipulation, and thus the second param is set to nil.
	x, err := d.Query(schema, nil)
	if err != nil {
		t.Fatalf("failed to define tables: %v", err)
	}
	if err := checkQueryResponse(x); err != nil {
		t.Errorf("failed to define tables:\n  %v", err)
	}
}

type queryResponse struct {
	Status string `json:"status,omitempty"`
	Time   string `json:"time,omitempty"`
	Detail string `json:"detail,omitempty"`
	// Result string `json:"result,omitempty"`
}

// checkQueryResponse checks the response bytes which is expected to be a json
// input, and returns an error if any error is found in that response. If there
// are multiple errors found, all of the error details are described as a part
// of the error message.
func checkQueryResponse(data interface{}) error {
	var err error
	var ok bool
	d := data
	if isSlice(data) {
		d, ok = data.([]interface{})
		if !ok {
			return errors.New("failed to deserialise response to slice")
		}
	}
	jsonBytes, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("failed to deserialise response '%+v' to slice", d)
	}

	var responses []queryResponse
	err = json.Unmarshal(jsonBytes, &responses)
	if err != nil {
		return fmt.Errorf("failed unmarshaling jsonBytes '%s': %w", jsonBytes, err)
	}

	errs := []string{}
	emptyErrDetailFound := false
	for _, r := range responses {
		if r.Status != "OK" {
			if r.Detail != "" {
				errs = append(errs, r.Detail)
				continue
			}

			// TODO: The server reuses the "result" field for both the actual
			// data and error string. If the server uses a different field for
			// the error details, we should be able to simply rely on the above
			// logic, and when no detail found, report with unknown error.
			// errs = append(errs, "unknown error with no detail from server")
			//
			// Because this is not the case, adding an extra logic to do a
			// separate unmarshal to pull out the result string below.
			emptyErrDetailFound = true
		}
	}
	if emptyErrDetailFound {
		type errorResponse struct {
			Status string `json:"status,omitempty"`
			Time   string `json:"time,omitempty"`
			Detail string `json:"detail,omitempty"`
			Result string `json:"result,omitempty"`
		}
		var errResponses []errorResponse
		err = json.Unmarshal(jsonBytes, &errResponses)
		if err != nil {
			return fmt.Errorf("failed unmarshaling jsonBytes with error '%s': %w", jsonBytes, err)
		}
		for _, r := range errResponses {
			if r.Status != "OK" {
				if r.Detail != "" {
					errs = append(errs, r.Detail)
					continue
				}
				if r.Result != "" {
					errs = append(errs, r.Result)
					continue
				}

				// At this point, neither Detail nor Result have error details.
				// Report it as unknown error.
				errs = append(errs, "unknown error with no detail from server")
			}
		}

	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to execute query:\n    %s", strings.Join(errs, "\n    "))
	}

	return nil
}

func isSlice(possibleSlice interface{}) bool {
	val := reflect.ValueOf(possibleSlice)
	return val.Kind() == reflect.Slice
}

func toSliceOfAny[T any](s []T) []any {
	result := make([]any, len(s))
	for i, v := range s {
		result[i] = v
	}
	return result
}
