package main

import (
	"errors"
	"testing"

	"github.com/reduxjs/redux-thunk-go/redux"
)

// The three type strings below are the ones the upstream README's sandwich-shop
// example dispatches. The reducer matches on the concrete structs instead, so
// these methods have no other caller and the strings would otherwise be
// unpinned.
func TestActionsCarryTheUpstreamActionTypes(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		action redux.Action
		want   string
	}{
		{"make a sandwich", makeASandwich("Me", "mustard"), "MAKE_SANDWICH"},
		{"apologize", apologize("The shop", "Me", errors.New("no sauce")), "APOLOGIZE"},
		{"withdraw", withdrawMoney(42), "WITHDRAW"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := testCase.action.ActionType(); got != testCase.want {
				t.Fatalf("ActionType() = %q, want %q", got, testCase.want)
			}
		})
	}
}

func Example() {
	run()
	// Output:
	// money after withdrawal: 0
	// sandwiches: [Me]
	// Done!
	// sandwiches after a failed sauce fetch: [Me My partner]
	// component render: <p>MemustardMy partner</p>
	// component render after mount: <p>MemustardMy partnermustardKid</p>
	// component render after update: <p>MemustardMy partnermustardKidmustardKid's friend</p>
	// everybody: [Me My Grandma My wife Our kids]
	// money after the shop run: 58
	// rendered: <p>MemustardMy GrandmamustardMy wifemustardOur kids</p>
	// sandwiches made while the shop was closed: 0
}
