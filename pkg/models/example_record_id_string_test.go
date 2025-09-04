package models

import "fmt"

func ExampleRecordID_String() {
	intID := NewRecordID("user", 12345)
	fmt.Println("intID:", intID.String())

	// The String function encloses the identifier in angle brackets to avoid ambiguity
	// with plain string identifiers that may contain special characters.
	intLikeStringID := NewRecordID("user", "12345")
	fmt.Println("intLikeStringID:", intLikeStringID.String())

	// Output:
	// intID: user:12345
	// intLikeStringID: user:⟨12345⟩
}
