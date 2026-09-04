package redux

import "testing"

// goID must degrade to 0 rather than mis-parse if the runtime ever changes the
// header format, because a wrong non-zero id would make the reentrancy guard
// fire against the wrong goroutine.
func TestParseGoroutineID(t *testing.T) {
	cases := map[string]uint64{
		"goroutine 42 [running]:": 42,
		"goroutine 1 [running]:":  1,
		"goroutine  [running]:":   0,
		"goroutine":               0,
		"":                        0,
	}

	for header, want := range cases {
		if got := parseGoroutineID([]byte(header)); got != want {
			t.Errorf("parseGoroutineID(%q) = %d, want %d", header, got, want)
		}
	}
}

func TestGoIDIsStableWithinAGoroutine(t *testing.T) {
	if first, second := goID(), goID(); first == 0 || first != second {
		t.Fatalf("goID() returned %d then %d", first, second)
	}
}

func TestGoIDDiffersAcrossGoroutines(t *testing.T) {
	mine := goID()
	other := make(chan uint64, 1)
	go func() { other <- goID() }()

	if theirs := <-other; theirs == mine {
		t.Fatalf("two goroutines reported the same id %d", mine)
	}
}
