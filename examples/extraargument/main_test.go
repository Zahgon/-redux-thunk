package main

import (
	"testing"

	"github.com/reduxjs/redux-thunk-go/redux"
)

// Nothing else in this example calls ActionType: the reducer matches on the
// concrete struct. Pinning the string here keeps the action's public name from
// drifting silently.
func TestUserLoadedCarriesItsActionType(t *testing.T) {
	var action redux.Action = UserLoaded{}

	if got := action.ActionType(); got != "USER_LOADED" {
		t.Fatalf("ActionType() = %q, want %q", got, "USER_LOADED")
	}
}

func Example() {
	main()
	// Output:
	// fetched: user-7
	// state: user-7
	// fetchUser failed: no such user
	// fetched with otherValue: user-42
}
