package models

import (
	"fmt"
	"strings"

	"github.com/surrealdb/surrealdb.go/v2/internal/cborwire"
)

// Regex is a regular expression, such as /a+/: SurrealQL's regex kind. It
// holds the text of the pattern in the syntax of the Rust regex crate, which
// SurrealDB compiles it with.
//
// That syntax is close to, but not the same as, the RE2 syntax of Go's regexp
// package, so the SDK never compiles a pattern, and never converts a
// *regexp.Regexp into a Regex or back. regexp.Compile(r.Pattern) gives Go's
// reading of the pattern, which can differ from the server's: \d, \w, \s and
// \b match Unicode text in Rust but only ASCII in Go, and Rust's class set
// operations, such as [a-z&&[^aeiou]], mean something else in Go.
//
// Over cbor.2 a Regex is sent as tag 193 and decoded from it. Plain cbor has
// no form for a regex: encoding one for plain cbor fails with an error that
// wraps constants.ErrNeedsCBOR2, and the server cannot send one.
//
// The zero value is the empty pattern, which matches any text.
type Regex struct {
	// Pattern is the text of the regex, without the slashes around it.
	Pattern string
}

// String returns r as the server writes it: the pattern between slashes,
// with each slash in it escaped, as in /a\/b/.
func (r Regex) String() string {
	return "/" + strings.ReplaceAll(r.Pattern, "/", `\/`) + "/"
}

// SurrealString returns r as a SurrealQL regex literal, such as /a+/. It
// escapes each slash the pattern does not escape already: the server reads an
// escaped slash \/ in a literal as a plain slash, which matches the same
// text. The empty pattern is a cast of the empty string instead, as //
// starts a comment in SurrealQL.
func (r Regex) SurrealString() string {
	if r.Pattern == "" {
		return "<regex> ''"
	}
	var b strings.Builder
	b.Grow(len(r.Pattern) + 4)
	b.WriteByte('/')
	for i := 0; i < len(r.Pattern); i++ {
		switch c := r.Pattern[i]; {
		case c == '\\' && i+1 < len(r.Pattern):
			// The server's lexer skips an escape and the byte after it.
			b.WriteByte(c)
			b.WriteByte(r.Pattern[i+1])
			i++
		case c == '/':
			b.WriteString(`\/`)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('/')
	return b.String()
}

// MarshalText returns r as String writes it.
func (r Regex) MarshalText() ([]byte, error) {
	return []byte(r.String()), nil
}

// UnmarshalText parses the form String writes: a pattern between slashes,
// where \/ stands for a slash.
func (r *Regex) UnmarshalText(text []byte) error {
	s := string(text)
	if len(s) < 2 || s[0] != '/' || s[len(s)-1] != '/' {
		return fmt.Errorf("invalid regex %q: want a pattern between slashes", s)
	}
	r.Pattern = strings.ReplaceAll(s[1:len(s)-1], `\/`, "/")
	return nil
}

// MarshalCBOR encodes r as tag 193 around its pattern. An encoder for plain
// cbor fails, as plain cbor has no form for a regex.
func (r Regex) MarshalCBOR() ([]byte, error) {
	return cborwire.AppendText(cborwire.AppendTag(make([]byte, 0, len(r.Pattern)+12), TagRegex), r.Pattern), nil
}

// UnmarshalCBOR decodes a regex (tag 193) into r. NULL and NONE leave r as
// it is.
func (r *Regex) UnmarshalCBOR(data []byte) error {
	const into = "models.Regex"
	item, absent, err := readKindItem(data, TagRegex, into)
	if err != nil || absent {
		return err
	}
	if !item.tagged || item.tag != TagRegex {
		return item.mismatch(into)
	}
	pattern, err := cborwire.ReadText(item.content)
	if err != nil {
		return fmt.Errorf("cannot decode %s: %w", into, err)
	}
	r.Pattern = pattern
	return nil
}
