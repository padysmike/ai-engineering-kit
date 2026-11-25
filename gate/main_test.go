package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const testTargets = "fmt-check,vet,test,coverage"

var baseFiles = map[string]string{
	"Makefile": ".PHONY: fmt-check vet test coverage\n" +
		"fmt-check:\n\t@true\n" +
		"vet:\n\t@true\n" +
		"test:\n\t@true\n" +
		"coverage:\n\t@sh scripts/cov.sh\n",
	"scripts/cov.sh": "v=$(cat coverage.txt)\n" +
		"if [ \"$v\" -lt 70 ]; then echo \"coverage $v% < 70%\"; exit 1; fi\n",
	"coverage.txt":             "80\n",
	"app/main.go":              "package main\n",
	"app/main_test.go":         "package main\n",
	"scenarios/health.feature": "Feature: health\n",
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitAll(t *testing.T, dir, msg string) {
	t.Helper()
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", msg)
}

func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "config", "user.email", "test@example.com")
	for rel, content := range baseFiles {
		write(t, dir, rel, content)
	}
	commitAll(t, dir, "base")
	return dir
}

func branchWith(t *testing.T, dir, name string, edit func()) {
	t.Helper()
	runGit(t, dir, "checkout", "-q", "-b", name)
	edit()
	commitAll(t, dir, name)
	runGit(t, dir, "checkout", "-q", "main")
}

func gate(t *testing.T, dir string, extra ...string) (int, string) {
	t.Helper()
	var buf bytes.Buffer
	args := append([]string{"--repo", dir, "--base", "main", "--targets", testTargets}, extra...)
	code := run(args, &buf)
	return code, buf.String()
}

func TestGate_CleanBranch_Passes(t *testing.T) {
	dir := newRepo(t)
	branchWith(t, dir, "ticket-1", func() {
		write(t, dir, "app/feature.go", "package main\n")
		write(t, dir, "app/feature_test.go", "package main\n")
	})

	code, out := gate(t, dir, "--branch", "ticket-1")

	if code != 0 {
		t.Fatalf("want exit 0, got %d\n%s", code, out)
	}
	if !strings.Contains(out, "RESULT: PASSED") {
		t.Errorf("missing PASSED line:\n%s", out)
	}
}

func TestGate_WeakenedCheck_Blocked(t *testing.T) {
	dir := newRepo(t)
	branchWith(t, dir, "ticket-2", func() {
		write(t, dir, "coverage.txt", "50\n")
		write(t, dir, "scripts/cov.sh", "exit 0\n")
	})

	code, out := gate(t, dir, "--branch", "ticket-2")

	if code != 1 {
		t.Fatalf("want exit 1, got %d\n%s", code, out)
	}
	for _, want := range []string{
		"[FAIL] protected path changed: scripts/cov.sh",
		"[FAIL] coverage",
		"coverage 50% < 70%",
		"RESULT: BLOCKED",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestGate_ChangedScenario_BlockedUnlessApproved(t *testing.T) {
	dir := newRepo(t)
	branchWith(t, dir, "ticket-3", func() {
		write(t, dir, "scenarios/health.feature", "Feature: weaker\n")
	})

	code, out := gate(t, dir, "--branch", "ticket-3")

	if code != 1 || !strings.Contains(out, "approved test or scenario changed: scenarios/health.feature") {
		t.Fatalf("want blocked on scenario change, got %d\n%s", code, out)
	}

	code, out = gate(t, dir, "--branch", "ticket-3", "--allow-test-changes")

	if code != 0 {
		t.Fatalf("want pass with --allow-test-changes, got %d\n%s", code, out)
	}
}

func TestGate_DeletedTest_Blocked(t *testing.T) {
	dir := newRepo(t)
	branchWith(t, dir, "ticket-4", func() {
		if err := os.Remove(filepath.Join(dir, "app/main_test.go")); err != nil {
			t.Fatal(err)
		}
	})

	code, out := gate(t, dir, "--branch", "ticket-4")

	if code != 1 || !strings.Contains(out, "app/main_test.go") {
		t.Fatalf("want blocked on deleted test, got %d\n%s", code, out)
	}
}

func TestGate_RenamedTest_Blocked(t *testing.T) {
	dir := newRepo(t)
	branchWith(t, dir, "ticket-5", func() {
		runGit(t, dir, "mv", "app/main_test.go", "app/other_test.go")
	})

	code, out := gate(t, dir, "--branch", "ticket-5")

	if code != 1 || !strings.Contains(out, "app/main_test.go") {
		t.Fatalf("want blocked on renamed test, got %d\n%s", code, out)
	}
}

func TestGate_PassedBranch_Merged(t *testing.T) {
	dir := newRepo(t)
	branchWith(t, dir, "ticket-6", func() { write(t, dir, "app/feature.go", "package main\n") })

	code, out := gate(t, dir, "--branch", "ticket-6", "--merge")

	if code != 0 {
		t.Fatalf("want exit 0, got %d\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "app/feature.go")); err != nil {
		t.Errorf("merge did not bring the file into main: %v", err)
	}
	if log := runGit(t, dir, "log", "--format=%s", "-1"); !strings.Contains(log, "gate passed") {
		t.Errorf("unexpected last commit: %q", log)
	}
}

func TestGate_DirtyTree_MergeRefused(t *testing.T) {
	dir := newRepo(t)
	branchWith(t, dir, "ticket-7", func() { write(t, dir, "app/feature.go", "package main\n") })
	write(t, dir, "untracked.txt", "x")

	code, out := gate(t, dir, "--branch", "ticket-7", "--merge")

	if code != 1 || !strings.Contains(out, "uncommitted changes") {
		t.Fatalf("want blocked on dirty tree, got %d\n%s", code, out)
	}
}

func TestGate_BlockedBranch_NeverMerged(t *testing.T) {
	dir := newRepo(t)
	branchWith(t, dir, "ticket-8", func() { write(t, dir, "coverage.txt", "10\n") })
	before := runGit(t, dir, "rev-parse", "main")

	code, _ := gate(t, dir, "--branch", "ticket-8", "--merge")

	if code != 1 {
		t.Fatalf("want exit 1, got %d", code)
	}
	if after := runGit(t, dir, "rev-parse", "main"); after != before {
		t.Error("main moved although the gate blocked the branch")
	}
}

func TestGate_BadArguments_ExitTwo(t *testing.T) {
	dir := newRepo(t)
	if code, _ := gate(t, dir); code != 2 {
		t.Errorf("missing --branch: want exit 2, got %d", code)
	}
	if code, _ := gate(t, dir, "--branch", "nope"); code != 2 {
		t.Errorf("unknown branch: want exit 2, got %d", code)
	}
}

func TestParseDiff_RenameAndDelete_ParsedPaths(t *testing.T) {
	raw := "A\x00new.go\x00M\x00a.go\x00R100\x00old.go\x00moved.go\x00D\x00gone.go\x00"

	got := parseDiff(raw)

	if len(got) != 4 {
		t.Fatalf("want 4 changes, got %d: %+v", len(got), got)
	}
	if got[2].status != "R" || got[2].paths[0] != "old.go" || got[2].paths[1] != "moved.go" {
		t.Errorf("bad rename parse: %+v", got[2])
	}
}

func TestMatchesAny_ProtectedPatterns_PrefixOnly(t *testing.T) {
	patterns := []string{"Makefile", "scripts/", ".github/"}
	cases := map[string]bool{
		"Makefile":             true,
		"scripts/cov.sh":       true,
		".github/CODEOWNERS":   true,
		"Makefile.local":       false,
		"app/scripts/thing.go": false,
	}
	for path, want := range cases {
		if got := matchesAny(path, patterns); got != want {
			t.Errorf("matchesAny(%q) = %v, want %v", path, got, want)
		}
	}
}
