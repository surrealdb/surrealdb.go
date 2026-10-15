package models

import (
	"fmt"
	"strings"
)

// File is a pointer to a file in a SurrealDB bucket, written f"bucket:/key" in
// SurrealQL. It is encoded as CBOR tag 55 holding [bucket, key].
//
// The key keeps its leading slash, as in "/path/to/file.txt". Files are an
// experimental feature of SurrealDB 3 (--allow-experimental files), but File
// values are always decoded.
//
// See https://surrealdb.com/docs/surrealql/datamodel/files.
type File struct {
	_ struct{} `cbor:",toarray"`

	// Bucket is the name of the bucket the file is in.
	Bucket string
	// Key is the path of the file in the bucket, beginning with a slash.
	Key string
}

// NewFile returns a pointer to the file at key in bucket. A leading slash is
// added to key if it has none.
func NewFile(bucket, key string) File {
	if !strings.HasPrefix(key, "/") {
		key = "/" + key
	}
	return File{Bucket: bucket, Key: key}
}

// String returns the file pointer as a SurrealQL literal, such as f"bucket:/key".
func (f File) String() string {
	return fmt.Sprintf(`f"%s:%s"`, escapeFileLiteral(f.Bucket), escapeFileLiteral(f.Key))
}

func escapeFileLiteral(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s)
}
