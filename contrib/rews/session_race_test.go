package rews

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

// TestSessionState_concurrentReplay changes the session state from several
// goroutines while reconnections replay it. Run with -race: the session map and
// fields used to be read and written without a lock.
func TestSessionState_concurrentReplay(t *testing.T) {
	log := logger.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	newMock := func() *mockWebSocketConnection {
		return &mockWebSocketConnection{notifications: make(map[string]chan connection.Notification)}
	}

	// Each reconnection gets a socket of its own, as it does for real.
	conn := New(func(ctx context.Context) (*mockWebSocketConnection, error) {
		return newMock(), nil
	}, 0, surrealcbor.New(), log)
	conn.WebSocketConnection = newMock()
	conn.state = StateConnected

	ctx := context.Background()
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				key := fmt.Sprintf("k%d_%d", g, i)
				require.NoError(t, conn.Let(ctx, key, i))
				require.NoError(t, conn.Use(ctx, "ns", key))
				require.NoError(t, conn.Authenticate(ctx, key))
				if i%2 == 0 {
					require.NoError(t, conn.Unset(ctx, key))
				}
			}
		}(g)
	}

	for i := 0; i < 20; i++ {
		// Replays the session state as a reconnection does.
		require.NoError(t, conn.reconnect(ctx))
	}
	wg.Wait()
}

func TestAccessOf(t *testing.T) {
	type auth struct {
		Namespace string `json:"NS,omitempty"`
		Database  string `json:"DB,omitempty"`
		Access    string `json:"AC,omitempty"`
		Username  string `json:"user,omitempty"`
		Password  string `json:"pass,omitempty"`
	}

	want := map[string]any{"NS": "n", "DB": "d", "AC": "a"}
	require.Equal(t, want, accessOf(map[string]any{"NS": "n", "DB": "d", "AC": "a", "user": "u", "pass": "p"}))
	require.Equal(t, want, accessOf(map[string]string{"NS": "n", "DB": "d", "AC": "a", "pass": "p"}))
	require.Equal(t, want, accessOf(auth{Namespace: "n", Database: "d", Access: "a", Username: "u", Password: "p"}))
	require.Equal(t, want, accessOf(&auth{Namespace: "n", Database: "d", Access: "a"}))
	require.Equal(t, map[string]any{"namespace": "n"}, accessOf(map[string]any{"namespace": "n", "refresh": "r"}))
	require.Empty(t, accessOf("nonsense"))
	require.Empty(t, accessOf(nil))
	require.Empty(t, accessOf((*auth)(nil)))
	require.Empty(t, accessOf(map[int]any{1: "n"}))
}

// TestAccessOf_namedMapTypes checks that maps of a named type, which a type
// switch on map[string]any does not match, still yield the access fields.
func TestAccessOf_namedMapTypes(t *testing.T) {
	type credentials map[string]any
	type stringCredentials map[string]string
	type key string
	type keyedCredentials map[key]any

	want := map[string]any{"NS": "n", "DB": "d", "AC": "a"}
	require.Equal(t, want, accessOf(credentials{"NS": "n", "DB": "d", "AC": "a", "user": "u", "pass": "p"}))
	require.Equal(t, want, accessOf(&credentials{"NS": "n", "DB": "d", "AC": "a"}))
	require.Equal(t, want, accessOf(stringCredentials{"NS": "n", "DB": "d", "AC": "a", "pass": "p"}))
	require.Equal(t, want, accessOf(keyedCredentials{"NS": "n", "DB": "d", "AC": "a", "pass": "p"}))
}

// TestAccessOf_embeddedStructs checks that access fields promoted from an
// embedded struct, as the encoder promotes them, are found.
func TestAccessOf_embeddedStructs(t *testing.T) {
	type auth struct {
		Namespace string `json:"NS,omitempty"`
		Database  string `json:"DB,omitempty"`
		Access    string `json:"AC,omitempty"`
		Username  string `json:"user,omitempty"`
		Password  string `json:"pass,omitempty"`
	}
	type embedded struct {
		auth
		Extra string `json:"extra"`
	}
	type embeddedPointer struct {
		*auth
	}
	type twoLevels struct {
		embedded
	}
	type shadowed struct {
		auth
		Namespace string `json:"NS"`
	}
	type named struct {
		auth `json:"creds"`
	}

	creds := auth{Namespace: "n", Database: "d", Access: "a", Username: "u", Password: "p"}
	want := map[string]any{"NS": "n", "DB": "d", "AC": "a"}

	require.Equal(t, want, accessOf(embedded{auth: creds, Extra: "x"}))
	require.Equal(t, want, accessOf(&embedded{auth: creds}))
	require.Equal(t, want, accessOf(embeddedPointer{auth: &creds}))
	require.Equal(t, want, accessOf(twoLevels{embedded{auth: creds}}))
	require.Empty(t, accessOf(embeddedPointer{}))

	// A field of the outer struct hides one of the same name it embeds.
	require.Equal(t, map[string]any{"NS": "outer", "DB": "d", "AC": "a"},
		accessOf(shadowed{auth: creds, Namespace: "outer"}))
	// An embedded struct with a tag name is encoded as a field, not promoted.
	require.Empty(t, accessOf(named{auth: creds}))
}

// TestAccessOf_cborTagsFirst checks that a field's cbor tag names it, as it
// does for the encoder, even when it also has a json tag.
func TestAccessOf_cborTagsFirst(t *testing.T) {
	type auth struct {
		Namespace string `cbor:"NS" json:"ignored"`
		Database  string `cbor:"DB,omitempty" json:"database_json"`
		Access    string `cbor:",omitempty" json:"AC"`
		Username  string `json:"user"`
	}

	// Access has a cbor tag without a name, so the encoder sends it as Access,
	// not AC, and that is the name kept to send again.
	require.Equal(t, map[string]any{"NS": "n", "DB": "d", "Access": "a"},
		accessOf(auth{Namespace: "n", Database: "d", Access: "a", Username: "u"}))
}

// TestAccessOf_pointerFields checks that access fields held through pointers
// or interfaces, which the encoder dereferences, are kept.
func TestAccessOf_pointerFields(t *testing.T) {
	type name string
	type auth struct {
		Namespace *string `json:"NS,omitempty"`
		Database  any     `json:"DB,omitempty"`
		Access    *name   `json:"AC,omitempty"`
		Username  *string `json:"user,omitempty"`
	}

	ns, ac, user := "n", name("a"), "u"
	var db any = &ns
	require.Equal(t, map[string]any{"NS": "n", "DB": "n", "AC": "a"},
		accessOf(auth{Namespace: &ns, Database: db, Access: &ac, Username: &user}))
	require.Equal(t, map[string]any{"DB": "d"}, accessOf(&auth{Database: "d"}))
	require.Empty(t, accessOf(auth{Database: 1}))
}

type untaggedNS struct{ NS string }
type taggedNS struct {
	Namespace string `json:"NS"`
}
type otherTaggedNS struct {
	Name string `json:"NS"`
}
type wrapsUntaggedNS struct{ untaggedNS }
type alsoWrapsUntaggedNS struct{ untaggedNS }

// TestAccessOf_dominance checks that of access fields of the same name at the
// same depth, the one the encoder sends is kept: a tagged field beats untagged
// ones, and two of the same kind cancel out.
func TestAccessOf_dominance(t *testing.T) {
	cases := map[string]struct {
		authData any
		want     map[string]any
	}{
		"tagged beats untagged": {
			authData: struct {
				untaggedNS
				taggedNS
			}{untaggedNS{"untagged"}, taggedNS{"tagged"}},
			want: map[string]any{"NS": "tagged"},
		},
		"tagged beats untagged, either order": {
			authData: struct {
				taggedNS
				untaggedNS
			}{taggedNS{"tagged"}, untaggedNS{"untagged"}},
			want: map[string]any{"NS": "tagged"},
		},
		"two tagged cancel out": {
			authData: struct {
				taggedNS
				otherTaggedNS //nolint:govet // two fields tagged NS at one depth is the case under test
			}{taggedNS{"a"}, otherTaggedNS{"b"}},
			want: map[string]any{},
		},
		"two untagged cancel out": {
			authData: struct {
				untaggedNS
				wrapsUntaggedNS
				alsoWrapsUntaggedNS
			}{untaggedNS{"shallow"}, wrapsUntaggedNS{untaggedNS{"a"}}, alsoWrapsUntaggedNS{untaggedNS{"b"}}},
			want: map[string]any{"NS": "shallow"},
		},
		"the same struct reached twice cancels out": {
			authData: struct {
				wrapsUntaggedNS
				alsoWrapsUntaggedNS
			}{wrapsUntaggedNS{untaggedNS{"a"}}, alsoWrapsUntaggedNS{untaggedNS{"b"}}},
			want: map[string]any{},
		},
		"a dominant field behind a nil pointer is not sent": {
			authData: struct {
				*taggedNS
				untaggedNS
			}{nil, untaggedNS{"untagged"}},
			want: map[string]any{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := accessOf(tc.authData)
			require.Equal(t, tc.want, got)

			// encoding/json applies the same rules; check against it.
			data, err := json.Marshal(tc.authData)
			require.NoError(t, err)
			var encoded map[string]any
			require.NoError(t, json.Unmarshal(data, &encoded))
			require.Equal(t, accessOf(encoded), got)
		})
	}
}
