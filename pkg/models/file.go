package models

import (
	"strings"
)

// File is a pointer to a file in a SurrealDB bucket, written f"bucket:/key" in
// SurrealQL. It is encoded as CBOR tag 55 holding [bucket, key].
//
// The key keeps its leading slash, as in "/path/to/file.txt". Files are an
// experimental feature of SurrealDB (--allow-experimental files), but File
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

// String returns the file pointer as a file string, f"bucket:/key", escaped
// the way FileRef in the SurrealDB JavaScript SDK escapes it. It always has
// this one form.
//
// Each character of the bucket and key is written as it is if it is an ASCII
// letter or digit, '_', '-' or '.', or a '/' in the key. Any other character,
// a '/' in the bucket included, is written with a backslash before it. A key
// with no leading slash is written with one, as NewFile adds it.
//
// A file string has no escape syntax, so SurrealDB parses the result back
// into the same File only when no character needed a backslash. Otherwise the
// result is for display only. To send a File in a query, bind it as a
// parameter rather than formatting it into the query text.
func (f File) String() string {
	key := f.Key
	if !strings.HasPrefix(key, "/") {
		key = "/" + key
	}

	var b strings.Builder
	b.WriteString(`f"`)
	writeFilePart(&b, f.Bucket, false)
	b.WriteByte(':')
	writeFilePart(&b, key, true)
	b.WriteByte('"')
	return b.String()
}

// writeFilePart writes s with a backslash before each character that a file
// string does not hold, as the JavaScript SDK's fmtInner does. JavaScript
// escapes each UTF-16 code unit, so it writes two backslashes for a character
// outside the Basic Multilingual Plane; a Go string holds code points, so
// that character gets one, as SurrealDB 3 writes it.
func writeFilePart(b *strings.Builder, s string, allowSlash bool) {
	for _, ch := range s {
		if !isASCIIAlphanumeric(ch) && ch != '_' && ch != '-' && ch != '.' && (!allowSlash || ch != '/') {
			b.WriteByte('\\')
		}
		b.WriteRune(ch)
	}
}
