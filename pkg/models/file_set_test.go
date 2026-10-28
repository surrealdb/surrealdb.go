package models_test

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/pkg/models"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

func codecs() map[string]interface {
	Marshal(any) ([]byte, error)
	Unmarshal([]byte, any) error
} {
	return map[string]interface {
		Marshal(any) ([]byte, error)
		Unmarshal([]byte, any) error
	}{
		"fxamacker":   &modelsCodec{},
		"surrealcbor": surrealcbor.New(),
	}
}

func TestFile_cbor(t *testing.T) {
	// 55(["bucket", "/a/b.txt"])
	const want = "d83782666275636b6574682f612f622e747874"

	for name, c := range codecs() {
		t.Run(name, func(t *testing.T) {
			data, err := c.Marshal(models.NewFile("bucket", "a/b.txt"))
			require.NoError(t, err)
			assert.Equal(t, want, hex.EncodeToString(data))

			raw, _ := hex.DecodeString(want)

			var f models.File
			require.NoError(t, c.Unmarshal(raw, &f))
			assert.Equal(t, models.File{Bucket: "bucket", Key: "/a/b.txt"}, f)

			var anything any
			require.NoError(t, c.Unmarshal(raw, &anything))
			assert.Equal(t, models.File{Bucket: "bucket", Key: "/a/b.txt"}, anything)
		})
	}
}

func TestFile_string(t *testing.T) {
	for _, tt := range []struct {
		file models.File
		want string
	}{
		// A file string: a named bucket of letters, digits, '_', '-' and '.',
		// and a key of '/' followed by those characters or '/'.
		{models.NewFile("bucket", "/a/b.txt"), `f"bucket:/a/b.txt"`},
		{models.NewFile("my_bucket-1.x", "dir/file_name.v2.tar.gz"), `f"my_bucket-1.x:/dir/file_name.v2.tar.gz"`},
		{models.NewFile("b", "//"), `f"b://"`},
		// Anything else is written as type::file with quoted strings.
		{models.NewFile("", "/path"), `type::file('', '/path')`},
		{models.NewFile("bucket", "/"), `type::file('bucket', '/')`},
		// A key with no leading slash is written with one, as the server holds it.
		{models.File{Bucket: "bucket", Key: "a"}, `f"bucket:/a"`},
		{models.File{Bucket: "bucket"}, `type::file('bucket', '/')`},
		{models.File{}, `type::file('', '/')`},
		{models.NewFile("buck et", `/a"b\c:d`), `type::file('buck et', '/a"b\\c:d')`},
		{models.NewFile("b/c", "/é d"), `type::file('b/c', '/é d')`},
		// A single quote switches to double quotes, which are then escaped.
		{models.NewFile("it's", `/"q"`), `type::file("it's", '/"q"')`},
		{models.NewFile("b", `/it's "q"`), `type::file('b', "/it's \"q\"")`},
		// Control characters use the server's escapes.
		{models.NewFile("b", "/\x00\r\t\n\b\f"), `type::file('b', '/\0\r\t\n\u{8}\f')`},
	} {
		assert.Equal(t, tt.want, tt.file.String(), "%#v", tt.file)
	}
}

func TestSet_cbor(t *testing.T) {
	set := models.NewSet(int64(1), "a", models.NewRecordID("t", int64(2)))

	for name, c := range codecs() {
		t.Run(name, func(t *testing.T) {
			data, err := c.Marshal(set)
			require.NoError(t, err)
			// 56([1, "a", 8(["t", 2])]): elements keep their own tags.
			assert.Equal(t, "d83883016161c882617402", hex.EncodeToString(data))

			var back models.Set
			require.NoError(t, c.Unmarshal(data, &back))
			require.Len(t, back, 3)

			var anything any
			require.NoError(t, c.Unmarshal(data, &anything))
			asSet, ok := anything.(models.Set)
			require.True(t, ok, "a set decoded into an interface is a models.Set, got %T", anything)
			assert.Len(t, asSet, 3)

			strs, err := c.Marshal(models.NewSet("x", "y"))
			require.NoError(t, err)
			var slice []string
			require.NoError(t, c.Unmarshal(strs, &slice))
			assert.Equal(t, []string{"x", "y"}, slice)

			type withSet struct {
				Tags models.Set `cbor:"tags"`
			}
			doc, err := c.Marshal(withSet{Tags: models.NewSet("x")})
			require.NoError(t, err)
			var got map[string]any
			require.NoError(t, c.Unmarshal(doc, &got))
			assert.Equal(t, models.NewSet("x"), got["tags"])
		})
	}
}

func TestSet_cborEmpty(t *testing.T) {
	for name, c := range codecs() {
		t.Run(name, func(t *testing.T) {
			// 56([]): an empty set is still a set.
			for _, s := range []models.Set{models.NewSet(), {}} {
				data, err := c.Marshal(s)
				require.NoError(t, err)
				assert.Equal(t, "d83880", hex.EncodeToString(data))
			}

			// A nil Set is a nil slice and encodes as null.
			data, err := c.Marshal(models.Set(nil))
			require.NoError(t, err)
			assert.Equal(t, "f6", hex.EncodeToString(data))
		})
	}
}

type modelsCodec struct {
	models.CborMarshaler
	models.CborUnmarshaler
}
