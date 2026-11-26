package main

import (
	"strings"
	"testing"
)

const alwaysPassingMakefile = ".PHONY: fmt-check vet test coverage\n" +
	"fmt-check vet test coverage:\n\t@true\n"

// gateWithOtherMakefile checks a branch whose coverage the trusted Makefile
// rejects, and which brings its own makefile under another name.
func gateWithOtherMakefile(t *testing.T, name string, protectedAllowed bool) (int, string) {
	t.Helper()
	dir := newRepo(t)
	branchWith(t, dir, "ticket-1", func() {
		write(t, dir, "coverage.txt", "10\n")
		write(t, dir, name, alwaysPassingMakefile)
	})
	var out strings.Builder
	code := execute(options{
		repo: dir, base: "main", branch: "ticket-1", targets: splitList(testTargets),
		protected: splitList(defaultProtected), allowProtected: protectedAllowed,
	}, &out)
	return code, out.String()
}

func TestOtherMakefile_AddedByBranch_TrustedMakefileStillRuns(t *testing.T) {
	for _, name := range []string{"GNUmakefile", "makefile"} {
		t.Run("WHEN the branch adds "+name+" THEN the trusted checks run", func(t *testing.T) {
			code, out := gateWithOtherMakefile(t, name, true)

			if code != exitBlocked || !strings.Contains(out, "[FAIL] coverage") || !strings.Contains(out, "coverage 10% < 70%") {
				t.Fatalf("exit %d\n%s", code, out)
			}
		})
	}
}

func TestOtherMakefile_AddedByBranch_NeedsHumanApproval(t *testing.T) {
	for _, name := range []string{"GNUmakefile", "makefile"} {
		t.Run("WHEN the branch adds "+name+" THEN the protected path blocks it", func(t *testing.T) {
			code, out := gateWithOtherMakefile(t, name, false)

			if code != exitBlocked || !strings.Contains(out, "[FAIL] protected path changed: "+name) {
				t.Fatalf("exit %d\n%s", code, out)
			}
		})
	}
}
