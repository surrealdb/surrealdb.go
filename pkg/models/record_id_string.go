package models

import (
	"fmt"
	"strings"
)

// isASCIIDigit checks if a rune is an ASCII digit (0-9)
func isASCIIDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

// isASCIIAlphanumeric checks if a rune is an ASCII letter or digit
func isASCIIAlphanumeric(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || isASCIIDigit(ch)
}

// escapeString escapes a string according to the delimiter character.
// It escapes the delimiter and backslash characters.
func escapeString(s string, delimiter rune) string {
	var result strings.Builder
	for _, ch := range s {
		if ch == delimiter || ch == '\\' {
			result.WriteRune('\\')
		}
		result.WriteRune(ch)
	}
	return result.String()
}

// isAllDigitsOrUnderscore checks if a string contains only digits and underscores
func isAllDigitsOrUnderscore(s string) bool {
	if s == "" {
		return false
	}
	for _, ch := range s {
		if ch != '_' && !isASCIIDigit(ch) {
			return false
		}
	}
	return true
}

// needsEscaping checks if a string needs escaping based on the Rust logic
func needsEscaping(s string) bool {
	// Check if contains non-alphanumeric (except underscore) characters
	hasSpecialChars := false
	for _, ch := range s {
		if !isASCIIAlphanumeric(ch) && ch != '_' {
			hasSpecialChars = true
			break
		}
	}

	// If has special chars OR is all digits/underscores, needs escaping
	return hasSpecialChars || isAllDigitsOrUnderscore(s)
}

// String returns the string representation of the RecordID with proper escaping.
//
// The ID part is escaped following the [Rust EscapeRid logic].
//
// This implementation assumes ACCESSIBLE_OUTPUT is always false, using angle brackets ⟨⟩
//
// Beware that the format for complex ID types (like arrays or objects) does not follow
// that of the Rust implementation.
//
// For example, `CREATE foo:["a","2",3]` will be formatted as `foo:['a','2',3]` in other places,
// but here it will be `foo:[a 2 3]`
//
// We aren't sure if it worth to implement the same escaping and formatting logic as in Rust,
// so we just use a simple string representation for now.
//
// [Rust EscapeRid logic]: https://github.com/surrealdb/surrealdb/blob/v3.0.0-alpha.7/crates/core/src/sql/escape.rs#L89-L102
func (r *RecordID) String() string {
	// Format ID with escaping if needed
	idStr := fmt.Sprintf("%v", r.ID)
	if strID, ok := r.ID.(string); ok && needsEscaping(strID) {
		escapedID := escapeString(strID, '⟩')
		idStr = fmt.Sprintf("⟨%s⟩", escapedID)
	}

	return fmt.Sprintf("%s:%s", r.Table, idStr)
}
