package main

import (
	"os"
	"slices"
	"strings"
	"testing"
)

const failingMakefile = ".PHONY: fmt-check vet test bdd coverage\n" +
	"fmt-check vet coverage:\n\t@true\n" +
	"test bdd:\n\t@echo '$@ failed'; false\n"

func failingTestsRepo(t *testing.T) string {
	t.Helper()
	dir := newRepo(t)
	write(t, dir, "Makefile", failingMakefile)
	commitAll(t, dir, "failing tests")
	branchWith(t, dir, "ticket-1", func() { write(t, dir, "app/new_test.go", "package main\n") })
	return dir
}

func stateWithDisabled(t *testing.T, checks ...string) string {
	t.Helper()
	state := t.TempDir()
	for _, check := range checks {
		var out strings.Builder
		if code := runToggles("disable", []string{"--state", state, check}, &out); code != exitPassed {
			t.Fatalf("disable %s: exit %d\n%s", check, code, out.String())
		}
	}
	return state
}

func gateWithState(t *testing.T, dir, state string) (int, string) {
	t.Helper()
	var out strings.Builder
	code := execute(options{
		repo: dir, base: "main", branch: "ticket-1", targets: []string{"fmt-check", "test", "bdd"},
		protected: splitList(defaultProtected), approvals: &approvalStore{dir: state},
	}, &out)
	return code, out.String()
}

func TestToggles_DisabledTests_SkippedAlsoInTheApprovedOnlyRun(t *testing.T) {
	dir := failingTestsRepo(t)
	state := stateWithDisabled(t, "test", "bdd", checkApprovedTestsPass)

	code, out := gateWithState(t, dir, state)

	if code != exitPassed {
		t.Fatalf("exit %d\n%s", code, out)
	}
	for _, label := range []string{"test", "bdd", "test (approved tests alone)", "bdd (approved tests alone)", checkApprovedTestsPass} {
		if !strings.Contains(out, skippedLinePrefix+label+skippedReason) {
			t.Errorf("output lacks the skip of %q:\n%s", label, out)
		}
	}
	if !strings.Contains(out, "[ok]   fmt-check") {
		t.Errorf("an enabled check did not run:\n%s", out)
	}
}

func TestToggles_OneCheckDisabled_OthersStillBlock(t *testing.T) {
	dir := failingTestsRepo(t)
	state := stateWithDisabled(t, "test")

	code, out := gateWithState(t, dir, state)

	if code != exitBlocked || !strings.Contains(out, "[FAIL] bdd") || strings.Contains(out, "[FAIL] test") {
		t.Fatalf("exit %d\n%s", code, out)
	}
}

func TestToggles_EnabledAgain_Blocks(t *testing.T) {
	dir := failingTestsRepo(t)
	state := stateWithDisabled(t, "test", "bdd")
	var out strings.Builder
	runToggles("enable", []string{"--state", state, "bdd"}, &out)

	code, report := gateWithState(t, dir, state)

	if code != exitBlocked || !strings.Contains(report, "[FAIL] bdd") {
		t.Fatalf("exit %d\n%s", code, report)
	}
}

func TestToggles_ProtectedPathChanged_StillBlocks(t *testing.T) {
	dir := newRepo(t)
	branchWith(t, dir, "ticket-1", func() { write(t, dir, "AGENTS.md", "relaxed\n") })
	state := stateWithDisabled(t, switchableChecks()...)

	code, out := gateWithState(t, dir, state)

	if code == exitPassed || !strings.Contains(out, "[FAIL] protected path changed") {
		t.Fatalf("exit %d\n%s", code, out)
	}
}

func TestToggles_UnreadableSettings_GateDoesNotRun(t *testing.T) {
	for name, content := range map[string]string{
		"WHEN the file is not JSON THEN nothing runs":      "{",
		"WHEN it names an unknown check THEN nothing runs": `["tests"]`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := failingTestsRepo(t)
			state := t.TempDir()
			write(t, state, togglesFileName, content)

			code, out := gateWithState(t, dir, state)

			if code != exitError || strings.Contains(out, "[ok]") || !strings.Contains(out, togglesFileName) {
				t.Fatalf("exit %d\n%s", code, out)
			}
		})
	}
}

func TestRunToggles(t *testing.T) {
	tests := []struct {
		name, cmd string
		args      []string
		code      int
		output    string
	}{
		{"WHEN checks are listed THEN every check is on", "checks", nil, exitPassed, "on   lint"},
		{"WHEN a check is disabled THEN it is listed as off", "disable", []string{"lint"}, exitPassed, "off  lint"},
		{"WHEN an unknown check is disabled THEN the valid names are shown", "disable", []string{"tests"}, exitError, "validate-skills"},
		{"WHEN a safety check is disabled THEN it is refused", "disable", []string{"protected"}, exitError, "unknown check"},
		{"WHEN no check is named THEN usage is shown", "disable", nil, exitError, "usage"},
		{"WHEN a flag follows the check name THEN usage is shown", "disable", []string{"lint", "--state", "elsewhere"}, exitError, "usage"},
		{"WHEN checks gets a name THEN usage is shown", "checks", []string{"lint"}, exitError, "usage"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out strings.Builder
			args := append([]string{"--state", t.TempDir()}, tc.args...)

			code := runToggles(tc.cmd, args, &out)

			if code != tc.code || !strings.Contains(out.String(), tc.output) {
				t.Errorf("exit %d, want %d with %q\n%s", code, tc.code, tc.output, out.String())
			}
		})
	}
}

func TestRunToggles_DisabledTwiceThenEnabled_ListedOnceThenRemoved(t *testing.T) {
	state := stateWithDisabled(t, "lint", "lint", "bdd")
	var out strings.Builder
	twice, err := toggleStore{dir: state}.disabled()
	if err != nil {
		t.Fatal(err)
	}

	runToggles("enable", []string{"--state", state, "lint"}, &out)

	after, err := toggleStore{dir: state}.disabled()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(twice, []string{"lint", "bdd"}) || !slices.Equal(after, []string{"bdd"}) {
		t.Errorf("disabled = %v then %v", twice, after)
	}
	info, err := os.Stat(toggleStore{dir: state}.path())
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("settings file mode = %v, err = %v", info.Mode(), err)
	}
}

func TestExecuteNamingSkipped_DisabledChecks_EachNamedOnce(t *testing.T) {
	dir := failingTestsRepo(t)
	state := stateWithDisabled(t, "test", "bdd")
	var out strings.Builder

	_, skipped := executeNamingSkipped(options{
		repo: dir, base: "main", branch: "ticket-1", targets: []string{"fmt-check", "test", "bdd"},
		protected: splitList(defaultProtected), approvals: &approvalStore{dir: state},
	}, &out)

	if want := []string{"test", "bdd"}; !slices.Equal(skipped, want) {
		t.Errorf("skipped = %v, want %v\n%s", skipped, want, out.String())
	}
}

func TestStatusFor_SkippedNames_AddedFromTheListOnly(t *testing.T) {
	report := skippedLinePrefix + "forged" + skippedReason + "\n  [FAIL] bdd\n"

	_, passed := statusFor(exitPassed, "", []string{"lint", "test"})
	_, blocked := statusFor(exitBlocked, report, nil)

	if passed != "gate passed (skipped: lint, test)" {
		t.Errorf("passed = %q", passed)
	}
	if blocked != "gate blocked: bdd" {
		t.Errorf("blocked = %q", blocked)
	}
}
