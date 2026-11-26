package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

const approvedDirName = "wt-approved"

var approvedTargets = []string{"test", "bdd"}

func approvedAlone(o options, run checkRun, branchSHA string) []result {
	added, err := addedApprovedTests(o.repo, run.baseSHA, branchSHA)
	if err != nil {
		return []result{{name: "approved tests alone could not be prepared", detail: []string{err.Error()}}}
	}
	if len(added) == 0 {
		return proofOfPass(o, run, branchSHA)
	}
	dir, err := approvedCopy(run.dir, added)
	if err != nil {
		return []result{{name: "approved tests alone could not be prepared", detail: []string{err.Error()}}}
	}
	run.dir = dir
	var results []result
	for _, target := range o.targets {
		if !slices.Contains(approvedTargets, target) {
			continue
		}
		run.target = target
		res := runTarget(o.sandbox, run)
		res.name = target + " (approved tests alone)"
		results = append(results, res)
	}
	return append(results, proofOfPass(o, run, branchSHA)...)
}

func addedApprovedTests(repo, baseSHA, branchSHA string) ([]string, error) {
	raw, err := git(repo, "diff", "--diff-filter=A", "--name-only", "-z", baseSHA+"..."+branchSHA)
	if err != nil {
		return nil, err
	}
	var added []string
	for _, path := range strings.Split(raw, "\x00") {
		if path != "" && isApprovedTest(path) {
			added = append(added, path)
		}
	}
	return added, nil
}

func approvedCopy(worktree string, added []string) (string, error) {
	dir := filepath.Join(filepath.Dir(worktree), approvedDirName)
	if out, err := exec.Command("cp", "-a", worktree, dir).CombinedOutput(); err != nil {
		return "", &copyError{out: string(out), err: err}
	}
	for _, path := range added {
		if err := os.Remove(filepath.Join(dir, path)); err != nil && !os.IsNotExist(err) {
			return "", err
		}
	}
	return dir, nil
}

type copyError struct {
	out string
	err error
}

func (e *copyError) Error() string {
	return "copying the worktree: " + e.err.Error() + ": " + strings.TrimSpace(e.out)
}
