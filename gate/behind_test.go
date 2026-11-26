package main

import (
	"strings"
	"testing"
)

const addedTest = "package lib\n\nimport \"testing\"\n\nfunc TestAddedOnBase(t *testing.T) {}\n\nfunc TestAlsoAddedOnBase(t *testing.T) {}\n"

// behindRepo has a branch that split off before main changed.
func behindRepo(t *testing.T, onBase func(dir string)) string {
	t.Helper()
	dir := moduleRepo(t)
	branchWith(t, dir, "ticket-1", func() { write(t, dir, "lib/extra.go", "package lib\n\nfunc Extra() int { return 1 }\n") })
	onBase(dir)
	commitAll(t, dir, "base moves on")
	return dir
}

func gateBranch(t *testing.T, dir string, disabled ...string) (int, string) {
	t.Helper()
	var out strings.Builder
	code := execute(options{
		repo: dir, base: "main", branch: "ticket-1", targets: []string{"fmt-check", "test"},
		protected: splitList(defaultProtected), approvals: &approvalStore{dir: stateWithDisabled(t, disabled...)},
	}, &out)
	return code, out.String()
}

func TestBehindBase_BaseGainedTests_FailsAtOnceAndSaysToRebase(t *testing.T) {
	dir := behindRepo(t, func(dir string) { write(t, dir, "lib/added_test.go", addedTest) })

	code, out := gateBranch(t, dir)

	_, status := statusFor(code, out, nil)
	if want := "gate blocked: 2 approved tests missing on head: rebase onto main"; code != exitBlocked || status != want {
		t.Fatalf("exit %d, status %q, want %q\n%s", code, status, want, out)
	}
	if !strings.Contains(out, "demo/lib TestAddedOnBase") {
		t.Errorf("the missing tests are not listed:\n%s", out)
	}
	if strings.Contains(out, "fmt-check") {
		t.Errorf("the make targets ran:\n%s", out)
	}
}

func TestBehindBase_BaseGainedNoTests_CheckedAsUsual(t *testing.T) {
	dir := behindRepo(t, func(dir string) { write(t, dir, "notes.md", "later\n") })

	code, out := gateBranch(t, dir)

	if code != exitPassed || !strings.Contains(out, "[ok]   fmt-check") {
		t.Fatalf("exit %d\n%s", code, out)
	}
}

func TestBehindBase_PassCheckDisabled_TargetsRun(t *testing.T) {
	dir := behindRepo(t, func(dir string) { write(t, dir, "lib/added_test.go", addedTest) })

	code, out := gateBranch(t, dir, checkApprovedTestsPass)

	if code != exitPassed || strings.Contains(out, "missing on head") || !strings.Contains(out, "[ok]   fmt-check") {
		t.Fatalf("exit %d\n%s", code, out)
	}
}

func TestBehindBase_UpToDateBranchDeletesATest_NotReportedAsBehind(t *testing.T) {
	dir := moduleRepo(t)
	branchWith(t, dir, "ticket-1", func() { runGit(t, dir, "rm", "-q", "lib/health_test.go") })

	code, out := gateBranch(t, dir)

	if code == exitPassed || strings.Contains(out, "missing on head") {
		t.Fatalf("exit %d\n%s", code, out)
	}
}

func TestGateOptions_PullRequest_BaseNamedWithoutRemote(t *testing.T) {
	o, err := prOptions{repoDir: t.TempDir(), remote: "origin", base: "test", branch: "ticket-1"}.gateOptions(1)

	if err != nil || o.base != "origin/test" || o.baseName() != "test" {
		t.Errorf("base = %q, name = %q, err = %v", o.base, o.baseName(), err)
	}
}
