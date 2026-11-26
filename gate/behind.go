package main

import (
	"fmt"
	"slices"
	"strings"
)

// behindBase reports the tests and scenarios the base gained after the branch
// split off and the branch lacks. The gate does not merge the base into the
// branch, so none of them can report a pass until the branch is rebased.
func behindBase(o options, baseSHA, branchSHA string) (result, bool) {
	if o.isDisabled(checkApprovedTestsPass) || !slices.ContainsFunc(o.targets, isApprovedTarget) {
		return result{}, false
	}
	missing, err := missingFromHead(o.repo, baseSHA, branchSHA)
	if err != nil {
		return result{name: "could not compare the branch with its base", detail: []string{err.Error()}}, true
	}
	if len(missing) == 0 {
		return result{}, false
	}
	return result{
		name:   fmt.Sprintf("%d approved tests missing on head: rebase onto %s", len(missing), o.baseName()),
		detail: tail(describeRequired(missing), detailLines),
	}, true
}

func missingFromHead(repo, baseSHA, branchSHA string) ([]requiredTest, error) {
	forkSHA, err := git(repo, "merge-base", baseSHA, branchSHA)
	if err != nil {
		return nil, err
	}
	forkSHA = strings.TrimSpace(forkSHA)
	if forkSHA == baseSHA {
		return nil, nil
	}
	onBase, err := requiredTests(repo, baseSHA)
	if err != nil {
		return nil, err
	}
	atFork, err := requiredTests(repo, forkSHA)
	if err != nil {
		return nil, err
	}
	onHead, err := requiredTests(repo, branchSHA)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(onBase, func(r requiredTest) bool {
		return slices.Contains(atFork, r) || slices.Contains(onHead, r)
	}), nil
}

func describeRequired(tests []requiredTest) string {
	lines := make([]string, 0, len(tests))
	for _, r := range tests {
		lines = append(lines, r.String())
	}
	return strings.Join(lines, "\n")
}
