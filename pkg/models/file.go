package models

import (
	"strings"
)

// File is a pointer to a file in a SurrealDB bucket, written f"bucket:/key" in
// SurrealQL. It is encoded as CBOR tag 55 holding [bucket, key].
//
// The key keeps its leading slash, as in "/path/to/file.txt". Files are an
// experimental feature of SurrealDB 3 (--allow-experimental files), but File
// values are always decoded.
//
// To send a File in a query, bind it as a parameter (RETURN $f with
// {"f": file}). It then travels as CBOR tag 55, so any bucket and key work.
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

// String returns the file pointer as SurrealDB displays it, such as
// f"bucket:/key".
//
// The result is a SurrealQL literal that the server parses back into the same
// File only when the bucket uses ASCII letters, digits, '_', '-' and '.', and
// the key uses those and '/'. SurrealQL has no escape syntax inside a file
// pointer, so any other character is written with a backslash before it, as
// the server's own <string> cast does, and the result is for display only.
// Bind such a File as a query parameter instead of formatting it into a query.
func (f File) String() string {
	var b strings.Builder
	b.WriteString(`f"`)
	writeFilePart(&b, f.Bucket, false)
	b.WriteByte(':')
	writeFilePart(&b, f.Key, true)
	b.WriteByte('"')
	return b.String()
}

// writeFilePart writes s, putting a backslash before each character a file
// pointer literal does not allow. This matches the server's formatting.
func writeFilePart(b *strings.Builder, s string, allowSlash bool) {
	for _, ch := range s {
		if !isASCIIAlphanumeric(ch) && ch != '_' && ch != '-' && ch != '.' && (!allowSlash || ch != '/') {
			b.WriteByte('\\')
		}
		b.WriteRune(ch)
	}
}
