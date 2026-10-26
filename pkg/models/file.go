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

// String returns the file pointer as SurrealQL, the way SurrealDB writes it.
//
// A file whose bucket and key fit a file string is written f"bucket:/key".
// That needs a non-empty bucket of ASCII letters, digits, '_', '-' and '.',
// and a key of a '/' followed by at least one of those characters or '/'.
// A file string has no escapes, so any other file is written as the function
// that builds it, type::file('bucket', '/key'), with both arguments quoted as
// SurrealQL strings.
//
// Either form parses back into the same File. To send a File in a query, bind
// it as a parameter rather than formatting it into the query text.
func (f File) String() string {
	if fileStringAdmits(f.Bucket, f.Key) {
		return `f"` + f.Bucket + ":" + f.Key + `"`
	}
	return "type::file(" + quoteString(f.Bucket) + ", " + quoteString(f.Key) + ")"
}

// fileStringAdmits reports whether the file string f"bucket:key" reads back as
// bucket and key.
func fileStringAdmits(bucket, key string) bool {
	rest, ok := strings.CutPrefix(key, "/")
	return bucket != "" && isFileWord(bucket, false) && ok && rest != "" && isFileWord(rest, true)
}

// isFileWord reports whether s holds only ASCII letters, digits, '_', '-' and
// '.', and '/' if allowSlash is set.
func isFileWord(s string, allowSlash bool) bool {
	for _, ch := range s {
		if !isASCIIAlphanumeric(ch) && ch != '_' && ch != '-' && ch != '.' && (!allowSlash || ch != '/') {
			return false
		}
	}
	return true
}

// quoteString writes s as a SurrealQL string, as the server does: in single
// quotes, or in double quotes if s holds a single quote. Backslashes, the
// quote, and the control characters with short escapes are escaped.
func quoteString(s string) string {
	quote := '\''
	if strings.ContainsRune(s, '\'') {
		quote = '"'
	}

	var b strings.Builder
	b.WriteRune(quote)
	for _, ch := range s {
		switch ch {
		case 0:
			b.WriteString(`\0`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\b':
			b.WriteString(`\u{8}`)
		case '\f':
			b.WriteString(`\f`)
		case '\\':
			b.WriteString(`\\`)
		case quote:
			b.WriteByte('\\')
			b.WriteRune(ch)
		default:
			b.WriteRune(ch)
		}
	}
	b.WriteRune(quote)
	return b.String()
}
