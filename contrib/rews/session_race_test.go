package rews

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"sync"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/internal/codec"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
	"github.com/surrealdb/surrealdb.go/pkg/models"
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

// accessOf returns the access fields of authData as each CBOR codec the SDK
// offers encodes it, which must agree.
func accessOf(authData any) map[string]any {
	sc := surrealcbor.New()
	got := accessFields(sc, sc, "signin", authData)
	//nolint:staticcheck // The deprecated fxamacker codec is still offered.
	fx := accessFields(&models.CborMarshaler{}, &models.CborUnmarshaler{}, "signin", authData)
	if !reflect.DeepEqual(got, fx) {
		panic(fmt.Sprintf("the codecs disagree on %#v: surrealcbor %v, fxamacker %v", authData, got, fx))
	}
	return got
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

// twoTaggedNS returns a struct embedding taggedNS and otherTaggedNS, which both
// tag a field NS. It is built at run time because go vet rejects the repeated
// tag in a struct type written out.
func twoTaggedNS(a, b string) any {
	v := reflect.New(reflect.StructOf([]reflect.StructField{
		{Name: "TaggedNS", Type: reflect.TypeOf(taggedNS{}), Anonymous: true},
		{Name: "OtherTaggedNS", Type: reflect.TypeOf(otherTaggedNS{}), Anonymous: true},
	})).Elem()
	v.Field(0).Set(reflect.ValueOf(taggedNS{a}))
	v.Field(1).Set(reflect.ValueOf(otherTaggedNS{b}))
	return v.Interface()
}

type wrapsUntaggedNS struct{ untaggedNS }
type alsoWrapsUntaggedNS struct{ untaggedNS }
type repeated struct{ untaggedNS }
type firstRepeat struct{ repeated }
type secondRepeat struct{ repeated }

// TestAccessOf_dominance checks that of access fields of the same name at the
// same depth, the one the encoder sends is kept: a tagged field beats untagged
// ones, and two of the same kind cancel out.
func TestAccessOf_dominance(t *testing.T) {
	cases := map[string]struct {
		authData any
		want     map[string]any
		// jsonDiffers marks a case encoding/json encodes differently from
		// the CBOR encoders.
		jsonDiffers bool
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
			authData: twoTaggedNS("a", "b"),
			want:     map[string]any{},
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
		"a struct reached twice is dropped with all it embeds": {
			authData: struct {
				firstRepeat
				secondRepeat
			}{firstRepeat{repeated{untaggedNS{"a"}}}, secondRepeat{repeated{untaggedNS{"b"}}}},
			want: map[string]any{},
			// encoding/json still promotes the fields of what the repeated
			// struct embeds; fxamacker/cbor drops them.
			jsonDiffers: true,
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

			if tc.jsonDiffers {
				return
			}
			// encoding/json otherwise applies the same rules; check against it.
			data, err := json.Marshal(tc.authData)
			require.NoError(t, err)
			var encoded map[string]any
			require.NoError(t, json.Unmarshal(data, &encoded))
			require.Equal(t, accessOf(encoded), got)
		})
	}
}

// opaqueCredentials marshals itself, as the encoder then sends it, rather
// than as the string it is.
type opaqueCredentials string

func (c opaqueCredentials) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(map[string]any{"NS": "n", "DB": "d", "AC": "a", "user": string(c)})
}

// TestAccessOf_customEncoding checks that access fields are taken from what
// authData encodes to when it encodes itself.
func TestAccessOf_customEncoding(t *testing.T) {
	require.Equal(t, map[string]any{"NS": "n", "DB": "d", "AC": "a"}, accessOf(opaqueCredentials("u")))
}

// renamingMarshaler encodes every value as a signin request naming the
// namespace "from-socket".
type renamingMarshaler struct{}

func (renamingMarshaler) Marshal(any) ([]byte, error) {
	return cbor.Marshal(map[string]any{"method": "signin", "params": []any{map[string]any{"NS": "from-socket"}}})
}

// socketWithMarshaler is a socket that reports the marshaler it encodes with.
type socketWithMarshaler struct {
	recordingConnection
}

func (*socketWithMarshaler) GetMarshaler() codec.Marshaler { return renamingMarshaler{} }

// TestRefreshAccess_usesTheSocketsMarshaler checks that the access fields are
// read from authData as the socket the signin went over encodes it.
func TestRefreshAccess_usesTheSocketsMarshaler(t *testing.T) {
	conn := New(func(context.Context) (*recordingConnection, error) {
		return &recordingConnection{}, nil
	}, 0, surrealcbor.New(), nil)
	authData := map[string]any{"NS": "n"}

	require.Equal(t, map[string]any{"NS": "from-socket"}, conn.refreshAccess(&socketWithMarshaler{}, "signin", authData))
	// A socket that does not say falls back to the codec rews was given.
	require.Equal(t, map[string]any{"NS": "n"}, conn.refreshAccess(&recordingConnection{}, "signin", authData))
}

// requestRewritingMarshaler encodes as surrealcbor does, except that it
// rewrites the namespace in the params of an RPC request, which it treats
// differently from a bare slice of params.
type requestRewritingMarshaler struct{ codec.Marshaler }

func (m requestRewritingMarshaler) Marshal(v any) ([]byte, error) {
	if req, ok := v.(*connection.RPCRequest); ok {
		params := make([]any, len(req.Params))
		for i, p := range req.Params {
			if ad, ok := p.(map[string]any); ok {
				rewritten := copyMap(ad)
				rewritten["NS"] = req.Method + ":" + fmt.Sprint(ad["NS"])
				p = rewritten
			}
			params[i] = p
		}
		r := *req
		r.Params = params
		v = &r
	}
	return m.Marshaler.Marshal(v)
}

// TestAccessFields_readsTheRequestEnvelope checks that the access fields are
// those of the signin request the socket writes, not of its params encoded
// alone, for a marshaler that encodes the two differently.
func TestAccessFields_readsTheRequestEnvelope(t *testing.T) {
	sc := surrealcbor.New()
	m := requestRewritingMarshaler{sc}
	authData := map[string]any{"NS": "n", "DB": "d", "AC": "a", "user": "u"}

	// The signin request, as the socket writes it.
	data, err := m.Marshal(&connection.RPCRequest{ID: "1", Method: "signin", Params: []any{authData}})
	require.NoError(t, err)
	var sent struct {
		Params []map[string]any `json:"params"`
	}
	require.NoError(t, sc.Unmarshal(data, &sent))
	require.Len(t, sent.Params, 1)
	require.Equal(t, "signin:n", sent.Params[0]["NS"])

	// Encoding the params alone, as accessFields used to, misses the rewrite.
	data, err = m.Marshal([]any{authData})
	require.NoError(t, err)
	var bare []map[string]any
	require.NoError(t, sc.Unmarshal(data, &bare))
	require.Len(t, bare, 1)
	require.Equal(t, "n", bare[0]["NS"])

	want := map[string]any{"NS": "signin:n", "DB": "d", "AC": "a"}
	require.Equal(t, want, accessFields(m, sc, "signin", authData))
	require.Equal(t, map[string]any{"NS": "signup:n", "DB": "d", "AC": "a"}, accessFields(m, sc, "signup", authData))

	conn := New(func(context.Context) (*recordingConnection, error) {
		return &recordingConnection{}, nil
	}, 0, sc, nil)
	require.Equal(t, want, conn.refreshAccess(&socketWithRewritingMarshaler{}, "signin", authData))
}

// socketWithRewritingMarshaler is a socket that encodes with
// requestRewritingMarshaler.
type socketWithRewritingMarshaler struct {
	recordingConnection
}

func (*socketWithRewritingMarshaler) GetMarshaler() codec.Marshaler {
	return requestRewritingMarshaler{surrealcbor.New()}
}
