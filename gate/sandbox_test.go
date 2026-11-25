package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChoose_Flags_RequireExplicitDecision(t *testing.T) {
	tests := []struct {
		name    string
		flags   sandboxFlags
		wantErr bool
	}{
		{"WHEN neither image nor host is given THEN it fails", sandboxFlags{}, true},
		{"WHEN both are given THEN it fails", sandboxFlags{image: "x", host: true}, true},
		{"WHEN an image is given THEN the sandbox is enabled", sandboxFlags{image: "x"}, false},
		{"WHEN host checks are chosen THEN the sandbox is off", sandboxFlags{host: true}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.flags.choose()

			if (err != nil) != tc.wantErr {
				t.Fatalf("err %v", err)
			}
		})
	}
}

func TestCleanEnv_TokensInEnvironment_Dropped(t *testing.T) {
	t.Setenv("GATE_GITHUB_TOKEN", "secret-a")
	t.Setenv("GATE_MCP_TOKENS", "x=secret-b")

	env := strings.Join(cleanEnv("REQUIRE_LINT=1"), "\n")

	if strings.Contains(env, "secret") || !strings.Contains(env, "REQUIRE_LINT=1") {
		t.Fatalf("env:\n%s", env)
	}
}

func TestDockerArgs_Run_NoNetworkNoTokensAndOnlyNeededMounts(t *testing.T) {
	t.Setenv("GATE_GITHUB_TOKEN", "secret-a")
	run := checkRun{dir: "/tmp/gate-1/wt", target: "test", baseSHA: "abc", diffFile: "/tmp/gate-1/changes.diff", modCache: "/home/u/go/pkg/mod"}

	args := strings.Join(sandbox{image: "checks:1"}.dockerArgs(run), " ")

	for _, want := range []string{"--network none", "--cap-drop ALL", "no-new-privileges", "-v /tmp/gate-1/wt:/work", "/go/pkg/mod:ro", "changes.diff:/gate/changes.diff:ro", "checks:1 make -s test"} {
		if !strings.Contains(args, want) {
			t.Errorf("args lack %q:\n%s", want, args)
		}
	}
	if strings.Contains(args, "secret") || strings.Contains(args, ".gate") {
		t.Errorf("args leak host data:\n%s", args)
	}
}

func TestRunTarget_HostChecksWithTokenInEnv_BranchCodeCannotSeeIt(t *testing.T) {
	t.Setenv("GATE_GITHUB_TOKEN", "secret-a")
	dir := t.TempDir()
	makefile := "probe:\n\t@test -z \"$$GATE_GITHUB_TOKEN\"\n"
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte(makefile), 0o600); err != nil {
		t.Fatal(err)
	}

	got := runTarget(sandbox{}, checkRun{dir: dir, target: "probe"})

	if !got.ok {
		t.Fatalf("token was visible to the check: %+v", got)
	}
}

func TestPrepareRun_Branch_WritesDiffOfGoFilesOutsideWorktree(t *testing.T) {
	dir := newRepo(t)
	branchWith(t, dir, "ticket-1", func() { write(t, dir, "app/feature.go", "package main\n") })
	base := strings.TrimSpace(runGit(t, dir, "rev-parse", "main"))
	branch := strings.TrimSpace(runGit(t, dir, "rev-parse", "ticket-1"))
	worktree := filepath.Join(t.TempDir(), "wt")

	run, err := sandbox{}.prepareRun(dir, worktree, base, branch)

	raw, readErr := os.ReadFile(run.diffFile)
	if err != nil || readErr != nil || !strings.Contains(string(raw), "app/feature.go") {
		t.Fatalf("err %v, read %v, diff %q", err, readErr, raw)
	}
	if strings.HasPrefix(run.diffFile, worktree) {
		t.Fatalf("diff file %s is inside the worktree", run.diffFile)
	}
}
