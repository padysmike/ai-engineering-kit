package main

import (
	"strings"
	"testing"
)

type approvalFixture struct {
	dir    string
	branch string
	fake   *fakeGitHub
	store  *approvalStore
	notify *strings.Builder
}

func newApprovalFixture(t *testing.T, branch string, edit func(dir string)) approvalFixture {
	t.Helper()
	dir := repoWithRemote(t)
	sha := pushBranch(t, dir, branch, func() { edit(dir) })
	fake := newFakeGitHub(t)
	fake.prHead = sha
	return approvalFixture{dir: dir, branch: branch, fake: fake, store: &approvalStore{dir: t.TempDir()}, notify: &strings.Builder{}}
}

func (f approvalFixture) options(merge bool) prOptions {
	o := prOptionsFor(f.dir, f.branch, merge, f.fake.api())
	o.approvals = f.store
	o.notify = f.notify
	return o
}

func changeScenario(t *testing.T) func(dir string) {
	return func(dir string) { write(t, dir, "scenarios/health.feature", "Feature: reworded\n") }
}

func (f approvalFixture) lastStatus() map[string]string {
	return f.fake.statuses[len(f.fake.statuses)-1]
}

func TestApprovals_ChangedScenario_RequestedWithoutBlocking(t *testing.T) {
	f := newApprovalFixture(t, "ticket-1", changeScenario(t))
	var out strings.Builder

	code := runPullRequest(f.options(true), &out)

	if code != exitNeedsApproval {
		t.Fatalf("exit %d\n%s", code, out.String())
	}
	if f.lastStatus()["state"] != "pending" || len(f.fake.merged) != 0 {
		t.Fatalf("status %v, merged %v", f.lastStatus(), f.fake.merged)
	}
	if !strings.Contains(f.notify.String(), "approval requested") || !strings.Contains(f.notify.String(), "scenarios/health.feature") {
		t.Errorf("notification: %q", f.notify.String())
	}
	if !strings.Contains(out.String(), "RESULT: NEEDS APPROVAL") {
		t.Errorf("output lacks the result line:\n%s", out.String())
	}
}

func TestApprovals_RepeatedCallWhilePending_NoSecondNotification(t *testing.T) {
	f := newApprovalFixture(t, "ticket-2", changeScenario(t))
	var first, second strings.Builder
	runPullRequest(f.options(true), &first)
	notified := f.notify.String()

	code := runPullRequest(f.options(true), &second)

	if code != exitNeedsApproval || f.notify.String() != notified {
		t.Fatalf("exit %d, notification changed: %q", code, f.notify.String())
	}
}

func TestApprovals_ApprovedByBranch_PassesAndMerges(t *testing.T) {
	f := newApprovalFixture(t, "ticket-3", changeScenario(t))
	var first, out, decision strings.Builder
	runPullRequest(f.options(true), &first)
	decideApproval(*f.store, "ticket-3", stateApproved, "", &decision)

	code := runPullRequest(f.options(true), &out)

	if code != exitPassed || f.lastStatus()["state"] != "success" || len(f.fake.merged) != 1 {
		t.Fatalf("exit %d, status %v, merged %v\n%s", code, f.lastStatus(), f.fake.merged, out.String())
	}
}

func TestApprovals_ApprovedByPullRequestNumber_Passes(t *testing.T) {
	f := newApprovalFixture(t, "ticket-4", changeScenario(t))
	var first, out, decision strings.Builder
	runPullRequest(f.options(false), &first)
	decideApproval(*f.store, "7", stateApproved, "", &decision)

	code := runPullRequest(f.options(false), &out)

	if code != exitPassed {
		t.Fatalf("exit %d\n%s", code, out.String())
	}
}

func TestApprovals_Denied_BlockedWithReasonAndNoNewRequest(t *testing.T) {
	f := newApprovalFixture(t, "ticket-5", changeScenario(t))
	var first, out, decision strings.Builder
	runPullRequest(f.options(true), &first)
	decideApproval(*f.store, "ticket-5", stateDenied, "keep the old wording", &decision)
	notified := f.notify.String()

	code := runPullRequest(f.options(true), &out)

	if code != exitBlocked || !strings.Contains(out.String(), "approval denied: keep the old wording") {
		t.Fatalf("exit %d\n%s", code, out.String())
	}
	if f.notify.String() != notified {
		t.Errorf("a new request was created after a denial: %q", f.notify.String())
	}
}

func TestApprovals_NewCommitAfterApproval_RequestedAgain(t *testing.T) {
	f := newApprovalFixture(t, "ticket-6", changeScenario(t))
	var first, second, decision strings.Builder
	runPullRequest(f.options(true), &first)
	decideApproval(*f.store, "ticket-6", stateApproved, "", &decision)
	runGit(t, f.dir, "checkout", "-q", "ticket-6")
	write(t, f.dir, "app/extra.go", "package main\n")
	commitAll(t, f.dir, "more")
	runGit(t, f.dir, "push", "-q", "origin", "ticket-6")
	runGit(t, f.dir, "checkout", "-q", "main")
	f.fake.prHead = strings.TrimSpace(runGit(t, f.dir, "rev-parse", "ticket-6"))

	code := runPullRequest(f.options(true), &second)

	if code != exitNeedsApproval {
		t.Fatalf("exit %d, want a new request for the new commit\n%s", code, second.String())
	}
}

func TestApprovals_OtherCheckFails_BlockedWithoutRequest(t *testing.T) {
	f := newApprovalFixture(t, "ticket-7", func(dir string) {
		write(t, dir, "scenarios/health.feature", "Feature: reworded\n")
		write(t, dir, "coverage.txt", "10\n")
	})
	var out strings.Builder

	code := runPullRequest(f.options(true), &out)

	records, _ := f.store.list()
	if code != exitBlocked || len(records) != 0 {
		t.Fatalf("exit %d, records %v\n%s", code, records, out.String())
	}
}

func TestApprovals_ListAndDecideCommands_ReportRecords(t *testing.T) {
	f := newApprovalFixture(t, "ticket-8", changeScenario(t))
	var run, listed, approved, again strings.Builder
	runPullRequest(f.options(false), &run)

	listCode := runApprovals("approvals", []string{"--state", f.store.dir}, &listed)
	approveCode := runApprovals("approve", []string{"--state", f.store.dir, "ticket-8"}, &approved)
	againCode := runApprovals("approve", []string{"--state", f.store.dir, "ticket-8"}, &again)

	if listCode != exitPassed || !strings.Contains(listed.String(), "ticket-8 (PR #7)") || !strings.Contains(listed.String(), "pending") {
		t.Errorf("list: %d %q", listCode, listed.String())
	}
	if approveCode != exitPassed || !strings.Contains(approved.String(), "approved") {
		t.Errorf("approve: %d %q", approveCode, approved.String())
	}
	if againCode != exitError || !strings.Contains(again.String(), "no pending approval") {
		t.Errorf("approving twice: %d %q", againCode, again.String())
	}
}

func TestApprovals_EmptyStoreAndMissingRef_ReportedClearly(t *testing.T) {
	var listed, missing strings.Builder
	state := t.TempDir()

	listCode := runApprovals("approvals", []string{"--state", state}, &listed)
	missingCode := runApprovals("deny", []string{"--state", state}, &missing)

	if listCode != exitPassed || !strings.Contains(listed.String(), "no approval requests") {
		t.Errorf("list: %d %q", listCode, listed.String())
	}
	if missingCode != exitError {
		t.Errorf("deny without a reference: %d %q", missingCode, missing.String())
	}
}

func TestApprovals_LocalModeChangedScenario_NeedsApprovalThenPasses(t *testing.T) {
	dir := newRepo(t)
	branchWith(t, dir, "ticket-9", func() { write(t, dir, "scenarios/health.feature", "Feature: reworded\n") })
	store := &approvalStore{dir: t.TempDir()}
	opts := options{
		repo: dir, base: "main", branch: "ticket-9", targets: splitList(testTargets),
		protected: splitList(defaultProtected), approvals: store, notify: &strings.Builder{},
	}
	var first, second, decision strings.Builder

	firstCode := execute(opts, &first)
	decideApproval(*store, "ticket-9", stateApproved, "", &decision)
	secondCode := execute(opts, &second)

	if firstCode != exitNeedsApproval || secondCode != exitPassed {
		t.Fatalf("first %d, second %d\n%s\n%s", firstCode, secondCode, first.String(), second.String())
	}
}
