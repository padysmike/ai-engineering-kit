package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	shortWait    = 100 * time.Millisecond
	runDeadline  = 30 * time.Second
	runningReply = "RESULT: RUNNING"
)

// slowGate has a test target that waits until release is called, then exits
// with the given status. runs counts how often it started.
type slowGate struct {
	dir         string
	session     *mcp.ClientSession
	releaseFile string
	runsFile    string
}

func newSlowGate(t *testing.T, exitStatus string) slowGate {
	t.Helper()
	shared := t.TempDir()
	g := slowGate{dir: newRepo(t), releaseFile: filepath.Join(shared, "release"), runsFile: filepath.Join(shared, "runs")}
	write(t, g.dir, "Makefile", ".PHONY: fmt-check vet test coverage\nfmt-check vet coverage:\n\t@true\n"+
		"test:\n\t@echo run >> "+g.runsFile+"; while [ ! -e "+g.releaseFile+" ]; do sleep 0.02; done; exit "+exitStatus+"\n")
	commitAll(t, g.dir, "slow tests")
	branchWith(t, g.dir, "ticket-1", func() { write(t, g.dir, "app/feature.go", "package main\n") })
	cfg := testConfig(g.dir, true)
	cfg.resultWait = shortWait
	g.session = connect(t, cfg)
	t.Cleanup(func() {
		g.release(t)
		if _, ok := g.runsStarted(); ok {
			g.finalResult(t)
		}
	})
	return g
}

func (g slowGate) release(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(g.releaseFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

func (g slowGate) runsStarted() (int, bool) {
	raw, err := os.ReadFile(g.runsFile)
	return strings.Count(string(raw), "run"), err == nil
}

func (g slowGate) call(t *testing.T, tool string) (string, bool) {
	t.Helper()
	text, isError, err := callTool(t, g.session, tool, "ticket-1")
	if err != nil {
		t.Fatal(err)
	}
	return text, isError
}

func (g slowGate) finalResult(t *testing.T) (string, bool) {
	t.Helper()
	deadline := time.Now().Add(runDeadline)
	for {
		text, isError := g.call(t, "gate_result")
		if !strings.HasPrefix(text, runningReply) {
			return text, isError
		}
		if time.Now().After(deadline) {
			t.Fatalf("the run did not end: %s", text)
		}
	}
}

func TestMCPRuns_SlowMerge_AnswersRunningThenResultAndMergesOnce(t *testing.T) {
	g := newSlowGate(t, "0")

	first, firstIsError := g.call(t, "gate_merge")
	repeated, _ := g.call(t, "gate_merge")
	other, otherIsError := g.call(t, "gate_check")
	waiting, _ := g.call(t, "gate_result")
	g.release(t)
	final, finalIsError := g.finalResult(t)

	for _, reply := range []string{first, repeated, waiting} {
		if !strings.HasPrefix(reply, runningReply) {
			t.Errorf("reply = %q, want %q", reply, runningReply)
		}
	}
	if firstIsError || !otherIsError || !strings.Contains(other, "gate_merge is still running") {
		t.Errorf("first is error = %v, other request: %v %q", firstIsError, otherIsError, other)
	}
	if finalIsError || !strings.Contains(final, "RESULT: PASSED") || !strings.Contains(final, "merged ticket-1") {
		t.Errorf("final is error = %v: %s", finalIsError, final)
	}
	if runs, _ := g.runsStarted(); runs != 1 {
		t.Errorf("runs = %d, want 1", runs)
	}
	if merges := strings.Count(runGit(t, g.dir, "log", "--merges", "--oneline", "main"), "\n"); merges != 1 {
		t.Errorf("merges = %d, want 1", merges)
	}
}

func TestMCPRuns_SlowBlockedCheck_ResultIsAnErrorAndAskingAgainStartsNothing(t *testing.T) {
	g := newSlowGate(t, "1")
	g.call(t, "gate_check")
	g.release(t)

	first, firstIsError := g.finalResult(t)
	again, againIsError := g.finalResult(t)

	if !firstIsError || !againIsError || first != again || !strings.Contains(first, "RESULT: BLOCKED") {
		t.Errorf("results: %v %q, %v %q", firstIsError, first, againIsError, again)
	}
	if runs, _ := g.runsStarted(); runs != 1 {
		t.Errorf("runs = %d, want 1", runs)
	}
}

func TestMCPRuns_EndedRun_NextRequestStartsANewOne(t *testing.T) {
	g := newSlowGate(t, "0")
	g.release(t)
	g.call(t, "gate_check")
	g.finalResult(t)

	g.call(t, "gate_check")
	g.finalResult(t)

	if runs, _ := g.runsStarted(); runs != 2 {
		t.Errorf("runs = %d, want 2", runs)
	}
}

func TestMCPResult_Refused(t *testing.T) {
	session := connect(t, testConfig(newRepo(t), false))
	for name, tc := range map[string]struct{ branch, want string }{
		"WHEN the branch has no run THEN it says to start one": {"ticket-9", "no gate run"},
		"WHEN the branch name is invalid THEN it is refused":   {"-bad", "invalid branch name"},
	} {
		t.Run(name, func(t *testing.T) {
			text, isError, err := callTool(t, session, "gate_result", tc.branch)

			if err != nil || !isError || !strings.Contains(text, tc.want) {
				t.Errorf("err = %v, isError = %v, text = %q", err, isError, text)
			}
		})
	}
}
