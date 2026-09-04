package main

import (
	"bytes"
	"strings"
	"testing"
)

// These are the exact lines `qc_probe.mts` prints in the source repository when
// driven with the same probe names. Pinning them here means a change that would
// break the differential fails in `go test`, before qc_migration_kit ever runs.
var expected = map[string]string{
	"middleware-shape": "next-handler-type=function\nnext-handler-arity=1\n" +
		"action-handler-type=function\naction-handler-arity=1\n",
	"dispatch-identity": "dispatch-identity=true\ngetstate-identity=true\ngetstate-value=42\n",
	"plain-action":      "next-received-same-action=true\nnext-return=redux\n",
	"thunk-return":      "thunk-return=rocks\n",
	"synchronous":       "mutated=1\n",
	"extra-argument":    "extra-identity=true\nextra-value=true\n",
	"arity":             "rest-only=3\ndispatch-plus-rest=2\nfourth-is-absent=true\n",
	"missing-api": "error=Cannot destructure property 'dispatch' of 'undefined' " +
		"as it is undefined.\n",
}

func TestEveryProbePrintsTheAgreedOutput(t *testing.T) {
	if len(expected) != len(probes) {
		t.Fatalf("pinned %d probe(s), but %d are registered", len(expected), len(probes))
	}

	for _, name := range probeNames() {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			if code := run(&stdout, &stderr, []string{name}); code != 0 {
				t.Fatalf("run(%q) = %d, want 0 (stderr: %s)", name, code, stderr.String())
			}
			if got := stdout.String(); got != expected[name] {
				t.Fatalf("probe %q printed\n%s\nwant\n%s", name, got, expected[name])
			}
			if stderr.Len() != 0 {
				t.Fatalf("probe %q wrote to stderr: %s", name, stderr.String())
			}
		})
	}
}

func TestHelpListsEveryProbeOnStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if code := run(&stdout, &stderr, []string{"--help"}); code != 0 {
		t.Fatalf("run(--help) = %d, want 0", code)
	}
	for _, name := range probeNames() {
		if !strings.Contains(stdout.String(), "  "+name+"\n") {
			t.Fatalf("--help does not list %q:\n%s", name, stdout.String())
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("--help wrote to stderr: %s", stderr.String())
	}
}

func TestUnknownProbeAndBadArityAreRejectedOnStderr(t *testing.T) {
	for _, testCase := range []struct {
		name string
		args []string
		want string
	}{
		{"unknown probe", []string{"bogus-probe"}, "unknown probe: bogus-probe\n"},
		{"no arguments", nil, "usage: qcprobe <probe>\n"},
		{"too many arguments", []string{"arity", "arity"}, "usage: qcprobe <probe>\n"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			if code := run(&stdout, &stderr, testCase.args); code != 2 {
				t.Fatalf("run(%v) = %d, want 2", testCase.args, code)
			}
			if !strings.HasPrefix(stderr.String(), testCase.want) {
				t.Fatalf("stderr = %q, want prefix %q", stderr.String(), testCase.want)
			}
			if stdout.Len() != 0 {
				t.Fatalf("run(%v) wrote to stdout: %s", testCase.args, stdout.String())
			}
		})
	}
}
