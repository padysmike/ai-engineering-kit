package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func branchAddingTests(t *testing.T) (dir, base, branch string) {
	dir = newRepo(t)
	branchWith(t, dir, "ticket-1", func() {
		write(t, dir, "app/hook_test.go", "package main\n")
		write(t, dir, "scenarios/new.feature", "Feature: new\n")
		write(t, dir, "app/feature.go", "package main\n")
		write(t, dir, "app/main_test.go", "package main\n// edited\n")
	})
	base = strings.TrimSpace(runGit(t, dir, "rev-parse", "main"))
	branch = strings.TrimSpace(runGit(t, dir, "rev-parse", "ticket-1"))
	return dir, base, branch
}

func TestAddedApprovedTests_Branch_ListsOnlyNewTestsAndScenarios(t *testing.T) {
	dir, base, branch := branchAddingTests(t)

	got, err := addedApprovedTests(dir, base, branch)

	want := []string{"app/hook_test.go", "scenarios/new.feature"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, err %v", got, err)
	}
}

func TestApprovedCopy_AddedFiles_RemovedFromCopyOnly(t *testing.T) {
	worktree := filepath.Join(t.TempDir(), "wt")
	write(t, worktree, "app/hook_test.go", "package main\n")
	write(t, worktree, "app/main_test.go", "package main\n")

	dir, err := approvedCopy(worktree, []string{"app/hook_test.go"})

	_, hookErr := os.Stat(filepath.Join(dir, "app/hook_test.go"))
	_, keptErr := os.Stat(filepath.Join(dir, "app/main_test.go"))
	_, origErr := os.Stat(filepath.Join(worktree, "app/hook_test.go"))
	if err != nil || !os.IsNotExist(hookErr) || keptErr != nil || origErr != nil {
		t.Fatalf("err %v, hook %v, kept %v, orig %v", err, hookErr, keptErr, origErr)
	}
}

func TestApprovedAlone_NewTestBreaksTarget_ApprovedRunStillPasses(t *testing.T) {
	dir, base, branch := branchAddingTests(t)
	worktree := filepath.Join(t.TempDir(), "wt")
	write(t, worktree, "Makefile", "test:\n\t@test ! -e app/hook_test.go\n")
	write(t, worktree, "app/hook_test.go", "package main\n")
	o := options{repo: dir, targets: []string{"test", "vet"}}

	got := approvedAlone(o, checkRun{dir: worktree, baseSHA: base}, branch)

	if len(got) != 1 || got[0].name != "test (approved tests alone)" || !got[0].ok {
		t.Fatalf("results %+v", got)
	}
}

func TestApprovedAlone_NoNewTests_NothingToRun(t *testing.T) {
	dir := newRepo(t)
	branchWith(t, dir, "ticket-2", func() { write(t, dir, "app/feature.go", "package main\n") })
	base := strings.TrimSpace(runGit(t, dir, "rev-parse", "main"))
	branch := strings.TrimSpace(runGit(t, dir, "rev-parse", "ticket-2"))

	got := approvedAlone(options{repo: dir, targets: []string{"test"}}, checkRun{baseSHA: base}, branch)

	if len(got) != 0 {
		t.Fatalf("results %+v", got)
	}
}
