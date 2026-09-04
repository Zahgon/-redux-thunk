package thunk_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestTypeErrors is the port of the negative type assertions in
// typescript_test/index.test-d.ts: every `.not.toMatchTypeOf`, `.not.toBeX`,
// `.not.toHaveProperty` and `@ts-expect-error` in the original.
//
// Those assertions state that some expression must NOT compile, so they cannot
// be expressed as compiling Go. Each one is instead a self-contained fixture in
// testdata/typeerrors whose first line is a `// want:` directive naming a
// substring the compiler error must contain. This test compiles each fixture
// and fails if it builds, or if it fails for the wrong reason.
//
// testdata is ignored by the go tool, so the fixtures never affect a normal
// build. To compile one it is copied into a scratch package inside the module.
func TestTypeErrors(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping compiler-invoking test in short mode")
	}

	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("go tool not found in PATH: %v", err)
	}

	const fixtureDir = "testdata/typeerrors"

	entries, err := os.ReadDir(fixtureDir)
	if err != nil {
		t.Fatalf("reading %s: %v", fixtureDir, err)
	}

	found := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		found++

		t.Run(strings.TrimSuffix(entry.Name(), ".go"), func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join(fixtureDir, entry.Name()))
			if err != nil {
				t.Fatalf("reading fixture: %v", err)
			}

			want, err := wantedError(string(source))
			if err != nil {
				t.Fatalf("%s: %v", entry.Name(), err)
			}

			output, buildErr := buildFixture(t, goTool, source)

			if buildErr == nil {
				t.Fatalf("expected compilation to fail with %q, but it succeeded", want)
			}
			if !strings.Contains(output, want) {
				t.Errorf("expected the compiler error to contain %q, got:\n%s", want, output)
			}
		})
	}

	if found == 0 {
		t.Fatalf("no fixtures found in %s", fixtureDir)
	}
}

// wantedError extracts the `// want: <substring>` directive from a fixture.
func wantedError(source string) (string, error) {
	for _, line := range strings.Split(source, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "// want:"); ok {
			want := strings.TrimSpace(rest)
			if want == "" {
				return "", errNoWantDirective
			}
			return want, nil
		}
	}
	return "", errNoWantDirective
}

var errNoWantDirective = errNoWant("fixture has no non-empty `// want:` directive")

type errNoWant string

func (e errNoWant) Error() string { return string(e) }

// buildFixture compiles source as a package inside this module and returns the
// compiler output. The scratch directory lives in the module so that the
// fixture's imports resolve against the working tree with no module
// indirection, and it is removed once the subtest ends.
func buildFixture(t *testing.T, goTool string, source []byte) (string, error) {
	t.Helper()

	dir, err := os.MkdirTemp(".", "typeerrors-scratch-")
	if err != nil {
		t.Fatalf("creating scratch directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("removing scratch directory: %v", err)
		}
	})

	if err := os.WriteFile(filepath.Join(dir, "fixture.go"), source, 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	cmd := exec.Command(goTool, "build", "./"+filepath.Base(dir))
	output, err := cmd.CombinedOutput()
	return string(output), err
}
